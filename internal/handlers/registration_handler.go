package handlers

import (
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

// RegistrationHandler handles mentor registration endpoints
type RegistrationHandler struct {
	service services.RegistrationServiceInterface
}

// NewRegistrationHandler creates a new registration handler
func NewRegistrationHandler(service services.RegistrationServiceInterface) *RegistrationHandler {
	return &RegistrationHandler{service: service}
}

// RegisterMentor handles POST /api/v1/register-mentor
func (h *RegistrationHandler) RegisterMentor(c *gin.Context) {
	var req models.RegisterMentorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErrors := ParseValidationErrors(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": validationErrors,
		})
		return
	}

	resp, err := h.service.RegisterMentor(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}
