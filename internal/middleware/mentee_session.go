package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/gin-gonic/gin"
)

const (
	// MenteeSessionCookieName is the mentee realm cookie, separate from mentor_session and admin_session.
	MenteeSessionCookieName = "mentee_session"

	// MenteeSessionContextKey stores the authenticated mentee session in the gin context.
	MenteeSessionContextKey = "mentee_session"
)

var (
	ErrMenteeSessionNotFound = errors.New("mentee session not found in context")
	// ErrMenteeSessionRejected is what a MenteeSessionValidator returns for a bad,
	// expired or revoked session. Any other error is treated as an outage (503).
	ErrMenteeSessionRejected = errors.New("mentee session rejected")
)

// MenteeSessionValidator validates a mentee session token, including the DB session_version check.
type MenteeSessionValidator interface {
	ValidateMenteeSession(ctx context.Context, token string) (*models.MenteeSession, error)
}

// MenteeSessionMiddleware is the only entry point to the mentee realm. It accepts only the
// mentee cookie and only tokens the mentee validator accepts; mentor/admin cookies are ignored.
func MenteeSessionMiddleware(validator MenteeSessionValidator, cookieDomain string, cookieSecure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(MenteeSessionCookieName)
		if err != nil || cookie == "" {
			_ = c.Error(fmt.Errorf("missing mentee session cookie")) //nolint:errcheck
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		session, err := validator.ValidateMenteeSession(c.Request.Context(), cookie)
		if err != nil {
			_ = c.Error(fmt.Errorf("mentee session: %w", err)) //nolint:errcheck
			if errors.Is(err, ErrMenteeSessionRejected) {
				ClearMenteeSessionCookie(c, cookieDomain, cookieSecure)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				return
			}
			// Fail closed when the session version can't be checked.
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
			return
		}

		c.Set(MenteeSessionContextKey, session)
		c.Next()
	}
}

// GetMenteeSession extracts the mentee session from context.
func GetMenteeSession(c *gin.Context) (*models.MenteeSession, error) {
	val, exists := c.Get(MenteeSessionContextKey)
	if !exists {
		return nil, ErrMenteeSessionNotFound
	}
	session, ok := val.(*models.MenteeSession)
	if !ok {
		return nil, ErrMenteeSessionNotFound
	}
	return session, nil
}

// SetMenteeSessionCookie sets the HttpOnly mentee session cookie.
func SetMenteeSessionCookie(c *gin.Context, token string, ttlSeconds int, domain string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(MenteeSessionCookieName, token, ttlSeconds, "/", domain, secure, true)
}

// ClearMenteeSessionCookie expires the mentee session cookie.
func ClearMenteeSessionCookie(c *gin.Context, domain string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(MenteeSessionCookieName, "", -1, "/", domain, secure, true)
}
