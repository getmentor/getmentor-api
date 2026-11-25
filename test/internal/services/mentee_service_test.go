package services_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	metrics.Init("test")
	_ = logger.Initialize(logger.Config{Level: "error", Environment: "test", ServiceName: "getmentor-api-test"}) //nolint:errcheck
}

// fakeMenteeStore is an in-memory MenteeStore.
type fakeMenteeStore struct {
	mu            sync.Mutex
	knownEmails   map[string]string // email -> mentee id
	tokens        map[string]string // hash -> mentee id
	versions      map[string]int
	consumeErr    error
	versionErr    error
	reviewErr     error
	lastTokenHash string
}

func newFakeStore() *fakeMenteeStore {
	return &fakeMenteeStore{
		knownEmails: map[string]string{"known@example.com": "mentee-1"},
		tokens:      map[string]string{},
		versions:    map[string]int{"mentee-1": 1},
	}
}

func (f *fakeMenteeStore) SetLoginToken(_ context.Context, email, hash string, _ time.Time) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.knownEmails[email]
	if !ok {
		return "", repository.ErrMenteeNotFound
	}
	f.tokens[hash] = id
	f.lastTokenHash = hash
	return id, nil
}

func (f *fakeMenteeStore) ConsumeLoginToken(_ context.Context, hash string) (*models.Mentee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	id, ok := f.tokens[hash]
	if !ok {
		return nil, repository.ErrMenteeNotFound
	}
	delete(f.tokens, hash)
	return &models.Mentee{ID: id, SessionVersion: f.versions[id]}, nil
}

func (f *fakeMenteeStore) GetSessionVersion(_ context.Context, id string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.versionErr != nil {
		return 0, f.versionErr
	}
	v, ok := f.versions[id]
	if !ok {
		return 0, repository.ErrMenteeNotFound
	}
	return v, nil
}

func (f *fakeMenteeStore) RevokeSessions(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versions[id]++
	return nil
}

func (f *fakeMenteeStore) ListRequests(context.Context, string) ([]models.MenteeRequestRow, error) {
	return []models.MenteeRequestRow{}, nil
}

func (f *fakeMenteeStore) CreateReview(context.Context, string, string, string, string, string) (string, error) {
	if f.reviewErr != nil {
		return "", f.reviewErr
	}
	return "review-1", nil
}

type allowAll struct{ denied bool }

func (a *allowAll) AllowKey(string) bool { return !a.denied }

type fakeCaptcha struct{ err error }

func (c fakeCaptcha) Verify(string) error { return c.err }

// recordingHTTP captures trigger calls.
type recordingHTTP struct {
	mu     sync.Mutex
	bodies []string
	urls   []string
}

func (r *recordingHTTP) Post(url, _ string, body io.Reader) (*http.Response, error) {
	b, _ := io.ReadAll(body) //nolint:errcheck
	r.mu.Lock()
	r.urls = append(r.urls, url)
	r.bodies = append(r.bodies, string(b))
	r.mu.Unlock()
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}
func (r *recordingHTTP) Get(string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}
func (r *recordingHTTP) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}
func (r *recordingHTTP) snapshot() ([]string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.urls...), append([]string{}, r.bodies...)
}

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.BaseURL = "https://getmentor.dev"
	cfg.Server.AppEnv = "test"
	cfg.MentorSession.JWTSecret = "test-secret-test-secret-test-secret"
	cfg.MentorSession.JWTIssuer = "getmentor-api"
	cfg.MentorSession.SessionTTLHours = 1
	cfg.MentorSession.LoginTokenTTLMinutes = 15
	cfg.EventTriggers.MenteeLoginEmailTriggerURL = "https://func.example/api/mentee-login-email?code=x"
	return cfg
}

func newService(store *fakeMenteeStore, httpClient *recordingHTTP, limiter services.KeyedRateLimiter, captcha services.CaptchaVerifier) *services.MenteeService {
	return services.NewMenteeService(store, testConfig(), limiter, captcha, httpClient, nil)
}

func TestMenteeRequestLogin_SameResponseForKnownAndUnknownEmail(t *testing.T) {
	store := newFakeStore()
	svc := newService(store, &recordingHTTP{}, &allowAll{}, fakeCaptcha{})

	known, err := svc.RequestLogin(context.Background(), "Known@Example.com ")
	require.NoError(t, err)
	unknown, err := svc.RequestLogin(context.Background(), "nobody@example.com")
	require.NoError(t, err)
	assert.Equal(t, known, unknown)
}

func TestMenteeRequestLogin_EmailJobGetsIDNotAddress(t *testing.T) {
	store := newFakeStore()
	httpClient := &recordingHTTP{}
	svc := newService(store, httpClient, &allowAll{}, fakeCaptcha{})

	_, err := svc.RequestLogin(context.Background(), "known@example.com")
	require.NoError(t, err)

	require.Eventually(t, func() bool { urls, _ := httpClient.snapshot(); return len(urls) == 1 }, time.Second, 10*time.Millisecond)
	urls, bodies := httpClient.snapshot()
	assert.Contains(t, urls[0], "mentee-login-email")
	assert.Contains(t, bodies[0], `"mentee_id":"mentee-1"`)
	assert.Contains(t, bodies[0], `"type":"mentee_login"`)
	assert.NotContains(t, bodies[0], "@")
	// Only the hash is stored, never the raw token in the URL.
	assert.NotContains(t, bodies[0], store.lastTokenHash)
}

func TestMenteeRequestLogin_UnknownEmailSendsNothing(t *testing.T) {
	httpClient := &recordingHTTP{}
	svc := newService(newFakeStore(), httpClient, &allowAll{}, fakeCaptcha{})
	_, err := svc.RequestLogin(context.Background(), "nobody@example.com")
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	urls, _ := httpClient.snapshot()
	assert.Empty(t, urls)
}

func TestMenteeRequestLogin_PerEmailRateLimit(t *testing.T) {
	limiter := middleware.NewRateLimiter(0.00667, 2)
	svc := newService(newFakeStore(), &recordingHTTP{}, limiter, fakeCaptcha{})
	ctx := context.Background()

	_, err := svc.RequestLogin(ctx, "known@example.com")
	require.NoError(t, err)
	_, err = svc.RequestLogin(ctx, "KNOWN@example.com")
	require.NoError(t, err)
	_, err = svc.RequestLogin(ctx, "known@example.com")
	assert.ErrorIs(t, err, services.ErrMenteeRateLimited)

	// Unknown emails are limited the same way, so the limit reveals nothing either.
	_, _ = svc.RequestLogin(ctx, "nobody@example.com") //nolint:errcheck
	_, _ = svc.RequestLogin(ctx, "nobody@example.com") //nolint:errcheck
	_, err = svc.RequestLogin(ctx, "nobody@example.com")
	assert.ErrorIs(t, err, services.ErrMenteeRateLimited)

	// Another email is unaffected.
	store := newFakeStore()
	store.knownEmails["other@example.com"] = "mentee-2"
	_, err = newService(store, &recordingHTTP{}, limiter, fakeCaptcha{}).RequestLogin(ctx, "other@example.com")
	assert.NoError(t, err)
}

// loginToken runs RequestLogin and pulls the raw token out of the emailed login URL.
func loginToken(t *testing.T, svc *services.MenteeService, httpClient *recordingHTTP) string {
	t.Helper()
	_, err := svc.RequestLogin(context.Background(), "known@example.com")
	require.NoError(t, err)
	require.Eventually(t, func() bool { u, _ := httpClient.snapshot(); return len(u) > 0 }, time.Second, 10*time.Millisecond)
	_, bodies := httpClient.snapshot()
	body := bodies[len(bodies)-1]
	i := strings.Index(body, "token=")
	require.Greater(t, i, 0)
	return body[i+len("token=") : i+len("token=")+len("mntk_")+64]
}

func TestMenteeVerify_SingleUseAndRevocableByLogout(t *testing.T) {
	store := newFakeStore()
	httpClient := &recordingHTTP{}
	svc := newService(store, httpClient, &allowAll{}, fakeCaptcha{})
	ctx := context.Background()

	token := loginToken(t, svc, httpClient)
	session, jwtToken, err := svc.VerifyLogin(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, "mentee-1", session.MenteeID)

	_, _, err = svc.VerifyLogin(ctx, token)
	assert.ErrorIs(t, err, services.ErrMenteeInvalidLoginToken)

	_, err = svc.ValidateSession(ctx, jwtToken)
	require.NoError(t, err)
	require.NoError(t, svc.Logout(ctx, jwtToken))
	_, err = svc.ValidateSession(ctx, jwtToken)
	assert.ErrorIs(t, err, services.ErrMenteeSessionInvalid)
}

func TestMenteeVerify_StorageErrorMintsNoSession(t *testing.T) {
	store := newFakeStore()
	httpClient := &recordingHTTP{}
	svc := newService(store, httpClient, &allowAll{}, fakeCaptcha{})
	token := loginToken(t, svc, httpClient)

	store.consumeErr = errors.New("db down")
	session, jwtToken, err := svc.VerifyLogin(context.Background(), token)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrMenteeInvalidLoginToken)
	assert.Nil(t, session)
	assert.Empty(t, jwtToken)
}

func TestMenteeVerify_RejectsMalformedToken(t *testing.T) {
	svc := newService(newFakeStore(), &recordingHTTP{}, &allowAll{}, fakeCaptcha{})
	for _, token := range []string{"mtk_" + strings.Repeat("a", 64) + "_1", "mntk_short", "mntk_" + strings.Repeat("Z", 64)} {
		_, _, err := svc.VerifyLogin(context.Background(), token)
		assert.ErrorIs(t, err, services.ErrMenteeInvalidLoginToken)
	}
}

func TestMenteeValidateSession_OutageIsNotAuthFailure(t *testing.T) {
	store := newFakeStore()
	httpClient := &recordingHTTP{}
	svc := newService(store, httpClient, &allowAll{}, fakeCaptcha{})
	_, jwtToken, err := svc.VerifyLogin(context.Background(), loginToken(t, svc, httpClient))
	require.NoError(t, err)

	store.versionErr = errors.New("db down")
	_, err = svc.ValidateMenteeSession(context.Background(), jwtToken)
	require.Error(t, err)
	assert.NotErrorIs(t, err, middleware.ErrMenteeSessionRejected)
}

func TestMenteeValidateSession_RejectsOtherRealms(t *testing.T) {
	svc := newService(newFakeStore(), &recordingHTTP{}, &allowAll{}, fakeCaptcha{})
	tm := jwt.NewTokenManager(testConfig().MentorSession.JWTSecret, "getmentor-api", 1)
	mentorToken, err := tm.GenerateToken("mentee-1", 1, "", "x") // even with a mentee's id as subject
	require.NoError(t, err)
	adminToken, err := tm.GenerateTokenWithRole("mentee-1", 0, "", "x", "admin")
	require.NoError(t, err)

	for _, token := range []string{mentorToken, adminToken} {
		_, err := svc.ValidateMenteeSession(context.Background(), token)
		assert.ErrorIs(t, err, middleware.ErrMenteeSessionRejected)
	}
}

func TestMenteeSubmitReview_ErrorMapping(t *testing.T) {
	ctx := context.Background()
	req := &models.MenteeSubmitReviewRequest{RequestID: "11111111-1111-1111-1111-111111111111", MentorReview: "great mentor!", RecaptchaToken: "t"}

	_, err := newService(newFakeStore(), &recordingHTTP{}, &allowAll{}, fakeCaptcha{err: errors.New("bad")}).SubmitReview(ctx, "mentee-1", req)
	assert.ErrorIs(t, err, services.ErrMenteeReviewCaptchaError)

	for repoErr, want := range map[error]error{
		repository.ErrMenteeRequestNotFound:  services.ErrMenteeRequestNotFound,
		repository.ErrMenteeReviewNotAllowed: services.ErrMenteeReviewNotAllowed,
		repository.ErrMenteeReviewExists:     services.ErrMenteeReviewExists,
	} {
		store := newFakeStore()
		store.reviewErr = repoErr
		_, submitErr := newService(store, &recordingHTTP{}, &allowAll{}, fakeCaptcha{}).SubmitReview(ctx, "mentee-1", req)
		assert.ErrorIs(t, submitErr, want)
	}

	id, err := newService(newFakeStore(), &recordingHTTP{}, &allowAll{}, fakeCaptcha{}).SubmitReview(ctx, "mentee-1", req)
	require.NoError(t, err)
	assert.Equal(t, "review-1", id)
}
