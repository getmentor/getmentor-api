package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	apperrors "github.com/getmentor/getmentor-api/pkg/errors"
	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	service *services.ProfileService
}

func NewProfileHandler(service *services.ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

// extractMentorCredentials extracts and validates mentor ID and auth token from request headers
func extractMentorCredentials(c *gin.Context) (int, string, error) {
	idStr := c.GetHeader("X-Mentor-ID")
	token := c.GetHeader("X-Auth-Token")

	if idStr == "" || token == "" {
		return 0, "", errors.New("missing credentials")
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, "", errors.New("invalid id")
	}

	return id, token, nil
}

func (h *ProfileHandler) SaveProfile(c *gin.Context) {
	// SECURITY: Read auth credentials from headers instead of URL query parameters
	// This prevents credentials from being logged in access logs, browser history, referrer headers
	id, token, err := extractMentorCredentials(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req models.SaveProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErrors := ParseValidationErrors(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": validationErrors,
		})
		return
	}

	if err := h.service.SaveProfile(id, token, &req); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
		} else if errors.Is(err, apperrors.ErrAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to update profile"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ProfileHandler) UploadProfilePicture(c *gin.Context) {
	// SECURITY: Read auth credentials from headers instead of URL query parameters
	id, token, err := extractMentorCredentials(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req models.UploadProfilePictureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErrors := ParseValidationErrors(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": validationErrors,
		})
		return
	}

	imageURL, err := h.service.UploadProfilePicture(id, token, &req)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
		} else if errors.Is(err, apperrors.ErrAccessDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Image uploaded successfully",
		"imageUrl": imageURL,
	})
}
