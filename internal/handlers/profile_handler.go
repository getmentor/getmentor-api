package handlers

import (
	"net/http"
	"strconv"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	service *services.ProfileService
}

func NewProfileHandler(service *services.ProfileService) *ProfileHandler {
	return &ProfileHandler{service: service}
}

func (h *ProfileHandler) SaveProfile(c *gin.Context) {
	idStr := c.Query("id")
	token := c.Query("token")

	if idStr == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id or token"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req models.SaveProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if err := h.service.SaveProfile(id, token, &req); err != nil {
		switch err.Error() {
		case "mentor not found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
		case "access denied":
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to update profile"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ProfileHandler) UploadProfilePicture(c *gin.Context) {
	idStr := c.Query("id")
	token := c.Query("token")

	if idStr == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id or token"})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var req models.UploadProfilePictureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	imageURL, err := h.service.UploadProfilePicture(id, token, &req)
	if err != nil {
		switch err.Error() {
		case "mentor not found":
			c.JSON(http.StatusNotFound, gin.H{"error": "Mentor not found"})
		case "access denied":
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		default:
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
