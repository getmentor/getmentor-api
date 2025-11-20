package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct{
	mentorCacheReady func() bool
}

func NewHealthHandler(mentorCacheReady func() bool) *HealthHandler {
	return &HealthHandler{
		mentorCacheReady: mentorCacheReady,
	}
}

func (h *HealthHandler) Healthcheck(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")

	// Check if mentor cache is ready
	if !h.mentorCacheReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"reason": "mentor cache not initialized",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
