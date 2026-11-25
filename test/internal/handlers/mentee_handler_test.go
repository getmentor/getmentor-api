package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/handlers"
	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const menteeTestSecret = "test-secret-test-secret-test-secret"

func init() {
	gin.SetMode(gin.TestMode)
	metrics.Init("test")
	_ = logger.Initialize(logger.Config{Level: "error", Environment: "test", ServiceName: "getmentor-api-test"}) //nolint:errcheck
}

// menteeStore: mentee-1 owns request ownedRequestID; everything else is unknown.
type menteeStore struct{ version int }

const ownedRequestID = "11111111-1111-1111-1111-111111111111"

func (s *menteeStore) SetLoginToken(_ context.Context, email, _ string, _ time.Time) (string, error) {
	if email == "known@example.com" {
		return "mentee-1", nil
	}
	return "", repository.ErrMenteeNotFound
}
func (s *menteeStore) ConsumeLoginToken(context.Context, string) (*models.Mentee, error) {
	return nil, repository.ErrMenteeNotFound
}
func (s *menteeStore) GetSessionVersion(_ context.Context, id string) (int, error) {
	if id != "mentee-1" {
		return 0, repository.ErrMenteeNotFound
	}
	return s.version, nil
}
func (s *menteeStore) RevokeSessions(context.Context, string) error { s.version++; return nil }
func (s *menteeStore) ListRequests(context.Context, string) ([]models.MenteeRequestRow, error) {
	return []models.MenteeRequestRow{}, nil
}
func (s *menteeStore) CreateReview(_ context.Context, _ string, requestID string, _, _, _ string) (string, error) {
	if requestID == ownedRequestID {
		return "review-1", nil
	}
	// The repository can't tell (and doesn't say) whether the id exists for someone else.
	return "", repository.ErrMenteeRequestNotFound
}

type noLimit struct{}

func (noLimit) AllowKey(string) bool { return true }

type okCaptcha struct{}

func (okCaptcha) Verify(string) error { return nil }

type nopHTTP struct{}

func (nopHTTP) Post(string, string, io.Reader) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}
func (nopHTTP) Get(string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}
func (nopHTTP) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
}

func menteeRouter(t *testing.T) *gin.Engine {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.BaseURL = "https://getmentor.dev"
	cfg.MentorSession.JWTSecret = menteeTestSecret
	cfg.MentorSession.JWTIssuer = "getmentor-api"
	cfg.MentorSession.SessionTTLHours = 1
	cfg.MentorSession.LoginTokenTTLMinutes = 15
	store := &menteeStore{version: 1}
	svc := services.NewMenteeService(store, cfg, noLimit{}, okCaptcha{}, nopHTTP{}, nil)
	h := handlers.NewMenteeHandler(svc)

	r := gin.New()
	session := middleware.MenteeSessionMiddleware(svc, "", false)
	r.POST("/auth/mentee/request-login", h.RequestLogin)
	r.POST("/auth/mentee/logout", h.Logout)
	r.GET("/auth/mentee/session", session, h.GetSession)
	r.GET("/mentee/requests", session, h.ListRequests)
	r.POST("/mentee/reviews", session, h.SubmitReview)
	return r
}

// menteeCookie is a session for mentee-1 at session_version 1 (the store's starting version).
func menteeCookie(t *testing.T) *http.Cookie {
	t.Helper()
	token, err := jwt.NewMenteeTokenManager(menteeTestSecret, "getmentor-api", 1).GenerateToken("mentee-1", 1)
	require.NoError(t, err)
	return &http.Cookie{Name: middleware.MenteeSessionCookieName, Value: token}
}

func do(r *gin.Engine, method, path string, body interface{}, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var reader io.Reader = http.NoBody
	if body != nil {
		b, _ := json.Marshal(body) //nolint:errcheck
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMenteeRequestLogin_IdenticalForKnownAndUnknown(t *testing.T) {
	r := menteeRouter(t)
	known := do(r, http.MethodPost, "/auth/mentee/request-login", gin.H{"email": "known@example.com"})
	unknown := do(r, http.MethodPost, "/auth/mentee/request-login", gin.H{"email": "unknown@example.com"})
	assert.Equal(t, http.StatusOK, known.Code)
	assert.Equal(t, known.Code, unknown.Code)
	assert.Equal(t, known.Body.String(), unknown.Body.String())
}

func TestMenteeRequestLogin_ValidatesEmail(t *testing.T) {
	r := menteeRouter(t)
	w := do(r, http.MethodPost, "/auth/mentee/request-login", gin.H{"email": "not-an-email"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "not-an-email")
}

func TestMenteeRealm_RejectsMentorAndAdminTokens(t *testing.T) {
	r := menteeRouter(t)
	tm := jwt.NewTokenManager(menteeTestSecret, "getmentor-api", 1)
	mentorToken, err := tm.GenerateToken("mentee-1", 1, "", "x")
	require.NoError(t, err)
	adminToken, err := tm.GenerateTokenWithRole("mentee-1", 0, "", "x", "admin")
	require.NoError(t, err)

	for _, token := range []string{mentorToken, adminToken} {
		w := do(r, http.MethodGet, "/mentee/requests", nil, &http.Cookie{Name: middleware.MenteeSessionCookieName, Value: token})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
	// A mentor/admin cookie under its own name is simply not a mentee session.
	w := do(r, http.MethodGet, "/mentee/requests", nil,
		&http.Cookie{Name: middleware.MentorSessionCookieName, Value: mentorToken},
		&http.Cookie{Name: middleware.AdminSessionCookieName, Value: adminToken})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	assert.Equal(t, http.StatusOK, do(r, http.MethodGet, "/mentee/requests", nil, menteeCookie(t)).Code)
}

func TestMenteeToken_RejectedByMentorAndAdminMiddleware(t *testing.T) {
	tm := jwt.NewTokenManager(menteeTestSecret, "getmentor-api", 1)
	menteeToken := menteeCookie(t).Value

	r := gin.New()
	r.GET("/mentor", middleware.MentorSessionMiddleware(tm, "", false, nil), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/admin", middleware.AdminSessionMiddleware(tm, "", false), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := do(r, http.MethodGet, "/mentor", nil, &http.Cookie{Name: middleware.MentorSessionCookieName, Value: menteeToken})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	w = do(r, http.MethodGet, "/admin", nil, &http.Cookie{Name: middleware.AdminSessionCookieName, Value: menteeToken})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMenteeLogout_RevokesCopiedCookies(t *testing.T) {
	r := menteeRouter(t)
	cookie := menteeCookie(t)
	require.Equal(t, http.StatusOK, do(r, http.MethodGet, "/auth/mentee/session", nil, cookie).Code)

	w := do(r, http.MethodPost, "/auth/mentee/logout", nil, cookie)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Set-Cookie"), middleware.MenteeSessionCookieName+"=;")

	// The same cookie value, replayed after logout, no longer works.
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodGet, "/auth/mentee/session", nil, cookie).Code)
}

func TestMenteeSubmitReview_NotOwnedAndMissingLookTheSame(t *testing.T) {
	r := menteeRouter(t)
	body := func(id string) gin.H {
		return gin.H{"requestId": id, "mentorReview": "great mentor!", "recaptchaToken": "t"}
	}
	notOwned := do(r, http.MethodPost, "/mentee/reviews", body("22222222-2222-2222-2222-222222222222"), menteeCookie(t))
	missing := do(r, http.MethodPost, "/mentee/reviews", body("33333333-3333-3333-3333-333333333333"), menteeCookie(t))
	assert.Equal(t, http.StatusNotFound, notOwned.Code)
	assert.Equal(t, notOwned.Body.String(), missing.Body.String())

	ok := do(r, http.MethodPost, "/mentee/reviews", body(ownedRequestID), menteeCookie(t))
	assert.Equal(t, http.StatusOK, ok.Code)
}

func TestMenteeSubmitReview_Validation(t *testing.T) {
	r := menteeRouter(t)
	cases := []gin.H{
		{"requestId": "not-a-uuid", "mentorReview": "great mentor!", "recaptchaToken": "t"},
		{"requestId": ownedRequestID, "mentorReview": "short", "recaptchaToken": "t"},
		{"requestId": ownedRequestID, "mentorReview": "great mentor!"},
		{"mentorReview": "great mentor!", "recaptchaToken": "t"},
	}
	for _, c := range cases {
		w := do(r, http.MethodPost, "/mentee/reviews", c, menteeCookie(t))
		assert.Equal(t, http.StatusBadRequest, w.Code, "%v", c)
		assert.NotContains(t, w.Body.String(), "not-a-uuid")
	}
	assert.Equal(t, http.StatusUnauthorized, do(r, http.MethodPost, "/mentee/reviews", cases[0]).Code)
}
