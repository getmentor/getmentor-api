package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/analytics"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

var (
	ErrMenteeRateLimited        = errors.New("too many login requests for this email")
	ErrMenteeInvalidLoginToken  = errors.New("invalid or expired mentee login token")
	ErrMenteeSessionInvalid     = errors.New("invalid mentee session")
	ErrMenteeNotConfigured      = errors.New("mentee login not configured")
	ErrMenteeRequestNotFound    = errors.New("request not found")
	ErrMenteeReviewNotAllowed   = errors.New("review not allowed for this request")
	ErrMenteeReviewExists       = errors.New("review already exists for this request")
	ErrMenteeReviewCaptchaError = errors.New("captcha verification failed")
)

// MenteeLoginMessage is the response to every accepted login request, whether or not
// the email is known, so the response never reveals which emails sent a request.
const MenteeLoginMessage = "Если с этого email отправлялись заявки, мы прислали на него ссылку для входа"

const menteeLoginTokenPrefix = "mntk_"

var menteeLoginTokenPattern = regexp.MustCompile(`^mntk_[0-9a-f]{64}$`)

// MenteeStore is the persistence the mentee service needs (implemented by MenteeRepository).
type MenteeStore interface {
	SetLoginToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) (string, error)
	ConsumeLoginToken(ctx context.Context, tokenHash string) (*models.Mentee, error)
	GetSessionVersion(ctx context.Context, menteeID string) (int, error)
	RevokeSessions(ctx context.Context, menteeID string) error
	ListRequests(ctx context.Context, menteeID string) ([]models.MenteeRequestRow, error)
	CreateReview(ctx context.Context, menteeID, requestID, mentorReview, platformReview, improvements string) (string, error)
}

// KeyedRateLimiter limits by an arbitrary key (implemented by middleware.RateLimiter).
type KeyedRateLimiter interface {
	AllowKey(key string) bool
}

// CaptchaVerifier verifies a reCAPTCHA token (implemented by recaptcha.Verifier).
type CaptchaVerifier interface {
	Verify(token string) error
}

// MenteeService implements mentee login, sessions and the read-only dashboard.
type MenteeService struct {
	store        MenteeStore
	config       *config.Config
	tokenManager *jwt.MenteeTokenManager
	emailLimiter KeyedRateLimiter
	captcha      CaptchaVerifier
	httpClient   httpclient.Client
	tracker      analytics.Tracker
}

// NewMenteeService creates the service. tokenManager is nil when JWT_SECRET is not set.
func NewMenteeService(
	store MenteeStore,
	cfg *config.Config,
	emailLimiter KeyedRateLimiter,
	captcha CaptchaVerifier,
	httpClient httpclient.Client,
	tracker analytics.Tracker,
) *MenteeService {

	if tracker == nil {
		tracker = analytics.NoopTracker{}
	}
	var tokenManager *jwt.MenteeTokenManager
	if cfg.MentorSession.JWTSecret != "" {
		tokenManager = jwt.NewMenteeTokenManager(cfg.MentorSession.JWTSecret, cfg.MentorSession.JWTIssuer, cfg.MentorSession.SessionTTLHours)
	}
	return &MenteeService{
		store:        store,
		config:       cfg,
		tokenManager: tokenManager,
		emailLimiter: emailLimiter,
		captcha:      captcha,
		httpClient:   httpClient,
		tracker:      tracker,
	}
}

// IsConfigured reports whether sessions can be issued.
func (s *MenteeService) IsConfigured() bool {
	return s.tokenManager != nil
}

// RequestLogin emails a one-time login link if the email has sent at least one request.
// The caller gets the same response for known and unknown emails.
func (s *MenteeService) RequestLogin(ctx context.Context, email string) (*models.RequestLoginResponse, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	generic := &models.RequestLoginResponse{Success: true, Message: MenteeLoginMessage}
	track := func(distinctID, outcome string) {
		s.tracker.Track(ctx, analytics.EventMenteeAuthLoginRequested, distinctID, map[string]interface{}{"outcome": outcome})
	}

	// Per-email limit: the per-IP limiter sees only the Next.js server's address.
	if s.emailLimiter != nil && !s.emailLimiter.AllowKey("mentee-login:"+hashToken(normalized)) {
		metrics.MenteeAuthLoginRequests.WithLabelValues("rate_limited").Inc()
		track(analytics.SystemDistinctID("api"), "rate_limited")
		return nil, ErrMenteeRateLimited
	}

	token, err := generateMenteeLoginToken()
	if err != nil {
		metrics.MenteeAuthLoginRequests.WithLabelValues("token_generation_failed").Inc()
		track(analytics.SystemDistinctID("api"), "token_generation_failed")
		return nil, ErrTokenGenerationFail
	}
	expiresAt := time.Now().Add(time.Duration(s.config.MentorSession.LoginTokenTTLMinutes) * time.Minute)

	menteeID, err := s.store.SetLoginToken(ctx, normalized, hashToken(token), expiresAt)
	if errors.Is(err, repository.ErrMenteeNotFound) {
		metrics.MenteeAuthLoginRequests.WithLabelValues("unknown_email").Inc()
		track(analytics.SystemDistinctID("api"), "unknown_email")
		return generic, nil
	}
	if err != nil {
		metrics.MenteeAuthLoginRequests.WithLabelValues("storage_failed").Inc()
		track(analytics.SystemDistinctID("api"), "storage_failed")
		logger.Error("Failed to store mentee login token", zap.Error(err))
		return nil, fmt.Errorf("failed to store mentee login token: %w", err)
	}

	loginURL := fmt.Sprintf("%s/mentee/auth/callback?token=%s", s.config.Server.BaseURL, url.QueryEscape(token))
	if s.config.EventTriggers.MenteeLoginEmailTriggerURL != "" {
		// The job reads the address from the mentees row; the payload carries no email.
		trigger.CallAsyncWithPayload(s.config.EventTriggers.MenteeLoginEmailTriggerURL, map[string]interface{}{
			"type":      "mentee_login",
			"mentee_id": menteeID,
			"login_url": loginURL,
		}, s.httpClient)
	} else if s.config.IsDevelopment() {
		logger.Info("=== DEVELOPMENT MENTEE LOGIN URL ===",
			zap.String("mentee_id", menteeID),
			zap.String("login_url", loginURL))
	}

	metrics.MenteeAuthLoginRequests.WithLabelValues("success").Inc()
	s.tracker.Track(ctx, analytics.EventMenteeAuthLoginRequested, analytics.MenteeDistinctID(menteeID), map[string]interface{}{
		"outcome":                 "success",
		"login_token_ttl_minutes": s.config.MentorSession.LoginTokenTTLMinutes,
	})
	return generic, nil
}

// VerifyLogin consumes a login token and issues a session token.
func (s *MenteeService) VerifyLogin(ctx context.Context, token string) (*models.MenteeSession, string, error) {
	track := func(distinctID, outcome string) {
		s.tracker.Track(ctx, analytics.EventMenteeAuthLoginVerified, distinctID, map[string]interface{}{"outcome": outcome})
	}
	if s.tokenManager == nil {
		metrics.MenteeAuthVerifyRequests.WithLabelValues("not_configured").Inc()
		track(analytics.SystemDistinctID("api"), "not_configured")
		return nil, "", ErrMenteeNotConfigured
	}
	if !menteeLoginTokenPattern.MatchString(token) {
		metrics.MenteeAuthVerifyRequests.WithLabelValues("invalid_token").Inc()
		track(analytics.SystemDistinctID("api"), "invalid_token")
		return nil, "", ErrMenteeInvalidLoginToken
	}

	mentee, err := s.store.ConsumeLoginToken(ctx, hashToken(token))
	if errors.Is(err, repository.ErrMenteeNotFound) {
		metrics.MenteeAuthVerifyRequests.WithLabelValues("invalid_token").Inc()
		track(analytics.SystemDistinctID("api"), "invalid_token")
		return nil, "", ErrMenteeInvalidLoginToken
	}
	if err != nil {
		// A storage error never mints a session.
		metrics.MenteeAuthVerifyRequests.WithLabelValues("storage_failed").Inc()
		track(analytics.SystemDistinctID("api"), "storage_failed")
		return nil, "", fmt.Errorf("failed to consume mentee login token: %w", err)
	}

	jwtToken, err := s.tokenManager.GenerateToken(mentee.ID, mentee.SessionVersion)
	if err != nil {
		metrics.MenteeAuthVerifyRequests.WithLabelValues("jwt_failed").Inc()
		track(analytics.MenteeDistinctID(mentee.ID), "jwt_failed")
		return nil, "", fmt.Errorf("failed to generate mentee session: %w", err)
	}

	now := time.Now()
	metrics.MenteeAuthVerifyRequests.WithLabelValues("success").Inc()
	s.tracker.Track(ctx, analytics.EventMenteeAuthLoginVerified, analytics.MenteeDistinctID(mentee.ID), map[string]interface{}{
		"outcome":           "success",
		"session_ttl_hours": s.config.MentorSession.SessionTTLHours,
	})
	return &models.MenteeSession{
		MenteeID:       mentee.ID,
		SessionVersion: mentee.SessionVersion,
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(s.tokenManager.GetExpirationTime()).Unix(),
	}, jwtToken, nil
}

// ValidateSession checks a session token and re-checks its session version in the DB,
// so a logout revokes every copy of the cookie at once. Infrastructure errors are
// returned as-is (the middleware fails closed with 503); bad sessions give ErrMenteeSessionInvalid.
func (s *MenteeService) ValidateSession(ctx context.Context, token string) (*models.MenteeSession, error) {
	if s.tokenManager == nil {
		return nil, ErrMenteeNotConfigured
	}
	claims, err := s.tokenManager.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMenteeSessionInvalid, err)
	}
	version, err := s.store.GetSessionVersion(ctx, claims.Subject)
	if errors.Is(err, repository.ErrMenteeNotFound) {
		return nil, ErrMenteeSessionInvalid
	}
	if err != nil {
		return nil, err
	}
	if version != claims.SessionVersion {
		return nil, fmt.Errorf("%w: session revoked", ErrMenteeSessionInvalid)
	}
	return &models.MenteeSession{
		MenteeID:       claims.Subject,
		SessionVersion: claims.SessionVersion,
		ExpiresAt:      claims.ExpiresAt.Unix(),
		IssuedAt:       claims.IssuedAt.Unix(),
	}, nil
}

// ValidateMenteeSession adapts ValidateSession to middleware.MenteeSessionValidator.
func (s *MenteeService) ValidateMenteeSession(ctx context.Context, token string) (*models.MenteeSession, error) {
	session, err := s.ValidateSession(ctx, token)
	if errors.Is(err, ErrMenteeSessionInvalid) || errors.Is(err, ErrMenteeNotConfigured) {
		return nil, fmt.Errorf("%w: %v", middleware.ErrMenteeSessionRejected, err)
	}
	return session, err
}

// Logout revokes every session of the mentee who owns the given (still valid) token.
// An invalid token is not an error: there is nothing to revoke.
func (s *MenteeService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	session, err := s.ValidateSession(ctx, token)
	if errors.Is(err, ErrMenteeSessionInvalid) || errors.Is(err, ErrMenteeNotConfigured) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.store.RevokeSessions(ctx, session.MenteeID); err != nil && !errors.Is(err, repository.ErrMenteeNotFound) {
		return err
	}
	s.tracker.Track(ctx, analytics.EventMenteeAuthLogout, analytics.MenteeDistinctID(session.MenteeID), map[string]interface{}{"outcome": "success"})
	return nil
}

// ListRequests returns the dashboard cards with all visibility rules already applied.
func (s *MenteeService) ListRequests(ctx context.Context, menteeID string) (*models.MenteeRequestsResponse, error) {
	rows, err := s.store.ListRequests(ctx, menteeID)
	if err != nil {
		return nil, err
	}
	resp := &models.MenteeRequestsResponse{Requests: make([]models.MenteeDashboardRequest, 0, len(rows))}
	groups := map[models.MenteeRequestGroup]int{}
	for i := range rows {
		card := rows[i].ToDashboardRequest()
		groups[card.Group]++
		resp.Requests = append(resp.Requests, card)
	}
	s.tracker.Track(ctx, analytics.EventMenteeDashboardViewed, analytics.MenteeDistinctID(menteeID), map[string]interface{}{
		"requests_total":   len(rows),
		"requests_pending": groups[models.MenteeGroupPending],
		"requests_active":  groups[models.MenteeGroupActive],
		"requests_done":    groups[models.MenteeGroupDone],
		"requests_closed":  groups[models.MenteeGroupClosed],
	})
	return resp, nil
}

// SubmitReview stores a review for one of the mentee's done requests. The session
// authorizes it; the age of the emailed review link does not matter.
func (s *MenteeService) SubmitReview(ctx context.Context, menteeID string, req *models.MenteeSubmitReviewRequest) (string, error) {
	outcome := func(o string) {
		// Same series as the emailed-link flow; "dashboard_" values tell the two apart.
		metrics.ReviewSubmissions.WithLabelValues("dashboard_" + o).Inc()
		// No request_id here: request ids are capabilities and stay out of analytics.
		s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.MenteeDistinctID(menteeID), map[string]interface{}{
			"source":              "mentee_dashboard",
			"outcome":             o,
			"has_platform_review": strings.TrimSpace(req.PlatformReview) != "",
			"has_improvements":    strings.TrimSpace(req.Improvements) != "",
		})
	}

	if err := s.captcha.Verify(req.RecaptchaToken); err != nil {
		outcome("captcha_failed")
		return "", ErrMenteeReviewCaptchaError
	}

	reviewID, err := s.store.CreateReview(ctx, menteeID, req.RequestID,
		strings.TrimSpace(req.MentorReview), strings.TrimSpace(req.PlatformReview), strings.TrimSpace(req.Improvements))
	switch {
	case errors.Is(err, repository.ErrMenteeRequestNotFound):
		outcome("not_found")
		return "", ErrMenteeRequestNotFound
	case errors.Is(err, repository.ErrMenteeReviewNotAllowed):
		outcome("not_allowed")
		return "", ErrMenteeReviewNotAllowed
	case errors.Is(err, repository.ErrMenteeReviewExists):
		outcome("already_exists")
		return "", ErrMenteeReviewExists
	case err != nil:
		outcome("db_error")
		return "", err
	}

	trigger.CallAsync(s.config.EventTriggers.ReviewCreatedTriggerURL, reviewID, s.httpClient)
	outcome("success")
	return reviewID, nil
}

// GetSessionTTL returns the session cookie lifetime in seconds.
func (s *MenteeService) GetSessionTTL() int {
	return s.config.MentorSession.SessionTTLHours * 3600
}

// GetCookieDomain returns the cookie domain.
func (s *MenteeService) GetCookieDomain() string {
	return s.config.MentorSession.CookieDomain
}

// GetCookieSecure returns whether cookies are Secure.
func (s *MenteeService) GetCookieSecure() bool {
	return s.config.MentorSession.CookieSecure
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generateMenteeLoginToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return menteeLoginTokenPrefix + hex.EncodeToString(b), nil
}
