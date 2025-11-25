package handlers

import (
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	service services.WebhookServiceInterface
}

func NewWebhookHandler(service services.WebhookServiceInterface) *WebhookHandler {
	return &WebhookHandler{service: service}
}

func (h *WebhookHandler) HandleAirtableWebhook(c *gin.Context) {
	var payload models.WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if err := h.service.HandleAirtableWebhook(c.Request.Context(), &payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
