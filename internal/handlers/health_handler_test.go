package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthHandler_Healthcheck(t *testing.T) {
	// Setup
	handler := NewHealthHandler()
	router := gin.New()
	router.GET("/healthcheck", handler.Healthcheck)

	// Create request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthcheck", http.NoBody)

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache, no-store, max-age=0, must-revalidate", w.Header().Get("Cache-Control"))
	assert.JSONEq(t, "{}", w.Body.String())
}
