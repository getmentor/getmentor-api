package handlers

import (
	"errors"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

// MentorRequestsHandler handles mentor request management endpoints
type MentorRequestsHandler struct {
	service services.MentorRequestsServiceInterface
}

// NewMentorRequestsHandler creates a new MentorRequestsHandler
func NewMentorRequestsHandler(service services.MentorRequestsServiceInterface) *MentorRequestsHandler {
	return &MentorRequestsHandler{
		service: service,
	}
}

// GetRequests handles GET /api/v1/mentor/requests
// Returns mentor's requests filtered by group (active/past)
func (h *MentorRequestsHandler) GetRequests(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	group := c.Query("group")
	if group == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required parameter: group",
		})
		return
	}

	if group != "active" && group != "past" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid group value. Must be 'active' or 'past'",
		})
		return
	}

	response, err := h.service.GetRequests(c.Request.Context(), session.MentorID, group)
	if err != nil {
		if errors.Is(err, services.ErrInvalidRequestGroup) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request group",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch requests",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRequestByID handles GET /api/v1/mentor/requests/:id
// Returns a single request by ID
func (h *MentorRequestsHandler) GetRequestByID(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	requestID := c.Param("id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request ID",
		})
		return
	}

	request, err := h.service.GetRequestByID(c.Request.Context(), session.MentorID, requestID)
	if err != nil {
		if errors.Is(err, services.ErrRequestNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Request not found",
			})
			return
		}
		if errors.Is(err, services.ErrAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch request",
		})
		return
	}

	c.JSON(http.StatusOK, request)
}

// UpdateStatus handles POST /api/v1/mentor/requests/:id/status
// Updates the status of a request
func (h *MentorRequestsHandler) UpdateStatus(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	requestID := c.Param("id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request ID",
		})
		return
	}

	var req models.UpdateStatusRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": gin.H{
				"message": "Status must be one of: pending, contacted, working, done, declined, unavailable",
			},
		})
		return
	}

	request, err := h.service.UpdateStatus(c.Request.Context(), session.MentorID, requestID, req.Status)
	if err != nil {
		h.handleRequestError(c, err, "Failed to update status")
		return
	}

	c.JSON(http.StatusOK, request)
}

// DeclineRequest handles POST /api/v1/mentor/requests/:id/decline
// Declines a request with a reason
func (h *MentorRequestsHandler) DeclineRequest(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	requestID := c.Param("id")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request ID",
		})
		return
	}

	var payload models.DeclineRequestPayload
	if bindErr := c.ShouldBindJSON(&payload); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": gin.H{
				"message": "Reason must be one of: no_time, topic_mismatch, helping_others, on_break, other",
			},
		})
		return
	}

	request, err := h.service.DeclineRequest(c.Request.Context(), session.MentorID, requestID, &payload)
	if err != nil {
		h.handleRequestError(c, err, "Failed to decline request")
		return
	}

	c.JSON(http.StatusOK, request)
}

// handleRequestError handles common request operation errors
func (h *MentorRequestsHandler) handleRequestError(c *gin.Context, err error, defaultMsg string) {
	if errors.Is(err, services.ErrRequestNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Request not found",
		})
		return
	}
	if errors.Is(err, services.ErrAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}
	if errors.Is(err, services.ErrInvalidStatusTransition) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid status transition",
			"details": err.Error(),
		})
		return
	}
	if errors.Is(err, services.ErrCannotDeclineRequest) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Cannot decline request",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": defaultMsg,
	})
}
