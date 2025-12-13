package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBotAPIAuthMiddleware_ValidAPIKey(t *testing.T) {
	// Setup
	router := gin.New()
	validAPIKey := "bot-secret-key-123"

	handlerCalled := false
	router.Use(middleware.BotAPIAuthMiddleware(validAPIKey))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with valid API key
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("X-Bot-API-Key", validAPIKey)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.True(t, handlerCalled, "Handler should be called for valid API key")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBotAPIAuthMiddleware_InvalidAPIKey(t *testing.T) {
	// Setup
	router := gin.New()
	validAPIKey := "bot-secret-key-123"

	handlerCalled := false
	router.Use(middleware.BotAPIAuthMiddleware(validAPIKey))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with invalid API key
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("X-Bot-API-Key", "wrong-key")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called for invalid API key")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBotAPIAuthMiddleware_MissingAPIKey(t *testing.T) {
	// Setup
	router := gin.New()
	validAPIKey := "bot-secret-key-123"

	handlerCalled := false
	router.Use(middleware.BotAPIAuthMiddleware(validAPIKey))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request without API key
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", http.NoBody)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called when API key is missing")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBotAPIAuthMiddleware_EmptyAPIKey(t *testing.T) {
	// Setup
	router := gin.New()
	validAPIKey := "bot-secret-key-123"

	handlerCalled := false
	router.Use(middleware.BotAPIAuthMiddleware(validAPIKey))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with empty API key
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("X-Bot-API-Key", "")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called for empty API key")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBotAPIAuthMiddleware_EmptyConfiguredKey(t *testing.T) {
	// Setup
	router := gin.New()
	validAPIKey := "" // Empty configured key

	handlerCalled := false
	router.Use(middleware.BotAPIAuthMiddleware(validAPIKey))
	router.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	// Create request with some API key
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("X-Bot-API-Key", "some-key")

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.False(t, handlerCalled, "Handler should not be called when no valid API key is configured")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
