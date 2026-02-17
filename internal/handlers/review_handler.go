package handlers

import (
	"errors"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

// ReviewHandler handles review-related HTTP requests
type ReviewHandler struct {
	service services.ReviewServiceInterface
}

// NewReviewHandler creates a new review handler
func NewReviewHandler(service services.ReviewServiceInterface) *ReviewHandler {
	return &ReviewHandler{service: service}
}

// CheckReview handles GET /api/v1/reviews/:requestId/check
func (h *ReviewHandler) CheckReview(c *gin.Context) {
	requestID := c.Param("requestId")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing request ID"})
		return
	}

	resp, err := h.service.CheckReview(c.Request.Context(), requestID)
	if err != nil {
		if errors.Is(err, services.ErrReviewRequestNotFound) {
			c.JSON(http.StatusNotFound, resp)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check review eligibility"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SubmitReview handles POST /api/v1/reviews/:requestId
func (h *ReviewHandler) SubmitReview(c *gin.Context) {
	requestID := c.Param("requestId")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing request ID"})
		return
	}

	var req models.SubmitReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErrors := ParseValidationErrors(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": validationErrors,
		})
		return
	}

	resp, err := h.service.SubmitReview(c.Request.Context(), requestID, &req)
	if err != nil {
		if resp != nil && resp.Error != "" {
			if errors.Is(err, services.ErrReviewRequestNotFound) {
				c.JSON(http.StatusNotFound, resp)
				return
			}
			if errors.Is(err, services.ErrReviewAlreadyExists) || errors.Is(err, services.ErrReviewRequestNotDone) {
				c.JSON(http.StatusConflict, resp)
				return
			}
			if errors.Is(err, services.ErrReviewCaptchaFailed) {
				c.JSON(http.StatusBadRequest, resp)
				return
			}
			c.JSON(http.StatusBadRequest, resp)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
