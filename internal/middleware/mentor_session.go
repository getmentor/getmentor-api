package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/jwt"
	"github.com/gin-gonic/gin"
)

const (
	// MentorSessionCookieName is the name of the session cookie
	MentorSessionCookieName = "mentor_session"

	// MentorSessionContextKey is the key used to store session in context
	MentorSessionContextKey = "mentor_session"
)

var (
	ErrSessionNotFound = errors.New("session not found in context")
	ErrInvalidSession  = errors.New("invalid session type")
)

// MentorAccessChecker reports whether a mentor may still use their session
// (the mentor exists and is not deleted).
type MentorAccessChecker interface {
	IsMentorAccessible(ctx context.Context, mentorID string) (bool, error)
}

// MentorSessionMiddleware validates JWT session cookie and adds session to context.
// When accessChecker is set, sessions of deleted mentors are rejected, so deleting
// a profile ends every session the mentor has open.
func MentorSessionMiddleware(tokenManager *jwt.TokenManager, cookieDomain string, cookieSecure bool, accessChecker MentorAccessChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session cookie
		cookie, err := c.Cookie(MentorSessionCookieName)
		if err != nil {
			_ = c.Error(fmt.Errorf("missing session cookie")) //nolint:errcheck
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Validate token
		claims, err := tokenManager.ValidateToken(cookie)
		if err != nil {
			_ = c.Error(fmt.Errorf("invalid session token: %w", err)) //nolint:errcheck

			// Clear invalid cookie
			clearSessionCookie(c, cookieDomain, cookieSecure)

			if errors.Is(err, jwt.ErrExpiredToken) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			}
			c.Abort()
			return
		}

		if accessChecker != nil {
			accessible, checkErr := accessChecker.IsMentorAccessible(c.Request.Context(), claims.MentorUUID)
			if checkErr != nil {
				_ = c.Error(fmt.Errorf("failed to check mentor access: %w", checkErr)) //nolint:errcheck
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
				c.Abort()
				return
			}
			if !accessible {
				_ = c.Error(fmt.Errorf("session for deleted or missing mentor %s", claims.MentorUUID)) //nolint:errcheck
				clearSessionCookie(c, cookieDomain, cookieSecure)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
				c.Abort()
				return
			}
		}

		// Create session from claims
		session := &models.MentorSession{
			LegacyID:  claims.LegacyID,
			MentorID:  claims.MentorUUID,
			Email:     claims.Email,
			Name:      claims.Name,
			ExpiresAt: claims.ExpiresAt.Unix(),
			IssuedAt:  claims.IssuedAt.Unix(),
		}

		// Add session to context
		c.Set(MentorSessionContextKey, session)
		c.Next()
	}
}

// GetMentorSession extracts session from context
func GetMentorSession(c *gin.Context) (*models.MentorSession, error) {
	val, exists := c.Get(MentorSessionContextKey)
	if !exists {
		return nil, ErrSessionNotFound
	}

	session, ok := val.(*models.MentorSession)
	if !ok {
		return nil, ErrInvalidSession
	}

	return session, nil
}

// SetSessionCookie sets the mentor session cookie
func SetSessionCookie(c *gin.Context, token string, ttlSeconds int, domain string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		MentorSessionCookieName,
		token,
		ttlSeconds,
		"/",
		domain,
		secure,
		true, // HttpOnly
	)
}

// ClearSessionCookie clears the mentor session cookie
func ClearSessionCookie(c *gin.Context, domain string, secure bool) {
	clearSessionCookie(c, domain, secure)
}

// clearSessionCookie is an internal helper to clear the cookie
func clearSessionCookie(c *gin.Context, domain string, secure bool) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		MentorSessionCookieName,
		"",
		-1,
		"/",
		domain,
		secure,
		true, // HttpOnly
	)
}
