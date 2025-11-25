package handlers

import (
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

type ContactHandler struct {
	service services.ContactServiceInterface
}

func NewContactHandler(service services.ContactServiceInterface) *ContactHandler {
	return &ContactHandler{service: service}
}

func (h *ContactHandler) ContactMentor(c *gin.Context) {
	var req models.ContactMentorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		validationErrors := ParseValidationErrors(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": validationErrors,
		})
		return
	}

	resp, err := h.service.SubmitContactForm(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusBadRequest, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}
