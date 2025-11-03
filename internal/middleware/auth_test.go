package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Initialize logger for tests
	logger.Init("test", "test")
}

func TestTokenAuthMiddleware_ValidToken(t *testing.T) {
	// Setup
	router := gin.New()
	validTokens := []string{"token1", "token2", "token3"}

	// Track if handler was called
	handlerCalled := false
	router.Use(TokenAuthMiddleware(validTokens...))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with valid token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("mentors_api_auth_token", "token2")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.True(t, handlerCalled, "Handler should be called for valid token")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTokenAuthMiddleware_InvalidToken(t *testing.T) {
	// Setup
	router := gin.New()
	validTokens := []string{"token1", "token2"}

	// Track if handler was called
	handlerCalled := false
	router.Use(TokenAuthMiddleware(validTokens...))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with invalid token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("mentors_api_auth_token", "invalid-token")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called for invalid token")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTokenAuthMiddleware_MissingToken(t *testing.T) {
	// Setup
	router := gin.New()
	validTokens := []string{"token1", "token2"}

	// Track if handler was called
	handlerCalled := false
	router.Use(TokenAuthMiddleware(validTokens...))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request without token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called when token is missing")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTokenAuthMiddleware_EmptyTokenList(t *testing.T) {
	// Setup
	router := gin.New()

	// Track if handler was called
	handlerCalled := false
	router.Use(TokenAuthMiddleware())
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with a token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("mentors_api_auth_token", "some-token")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called when no valid tokens are configured")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestInternalAPIAuthMiddleware_ValidToken(t *testing.T) {
	// Setup
	router := gin.New()
	validToken := "internal-secret-token"

	// Track if handler was called
	handlerCalled := false
	router.Use(InternalAPIAuthMiddleware(validToken))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with valid token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-internal-mentors-api-auth-token", validToken)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.True(t, handlerCalled, "Handler should be called for valid internal token")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInternalAPIAuthMiddleware_InvalidToken(t *testing.T) {
	// Setup
	router := gin.New()
	validToken := "internal-secret-token"

	// Track if handler was called
	handlerCalled := false
	router.Use(InternalAPIAuthMiddleware(validToken))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with invalid token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-internal-mentors-api-auth-token", "wrong-token")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called for invalid internal token")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestInternalAPIAuthMiddleware_MissingToken(t *testing.T) {
	// Setup
	router := gin.New()
	validToken := "internal-secret-token"

	// Track if handler was called
	handlerCalled := false
	router.Use(InternalAPIAuthMiddleware(validToken))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request without token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called when internal token is missing")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWebhookAuthMiddleware_ValidSecret(t *testing.T) {
	// Setup
	router := gin.New()
	validSecret := "webhook-secret-123"

	// Track if handler was called
	handlerCalled := false
	router.Use(WebhookAuthMiddleware(validSecret))
	router.POST("/webhook", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with valid secret
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Webhook-Secret", validSecret)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.True(t, handlerCalled, "Handler should be called for valid webhook secret")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookAuthMiddleware_InvalidSecret(t *testing.T) {
	// Setup
	router := gin.New()
	validSecret := "webhook-secret-123"

	// Track if handler was called
	handlerCalled := false
	router.Use(WebhookAuthMiddleware(validSecret))
	router.POST("/webhook", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with invalid secret
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Webhook-Secret", "wrong-secret")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called for invalid webhook secret")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhookAuthMiddleware_MissingSecret(t *testing.T) {
	// Setup
	router := gin.New()
	validSecret := "webhook-secret-123"

	// Track if handler was called
	handlerCalled := false
	router.Use(WebhookAuthMiddleware(validSecret))
	router.POST("/webhook", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request without secret
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/webhook", nil)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called when webhook secret is missing")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
