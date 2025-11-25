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

func (h *WebhookHandler) RevalidateNextJS(c *gin.Context) {
	slug := c.Query("slug")
	secret := c.Query("secret")

	if slug == "" || secret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing slug or secret"})
		return
	}

	if err := h.service.RevalidateNextJSManual(c.Request.Context(), slug, secret); err != nil {
		if err.Error() == "invalid secret" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid secret"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revalidate"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"revalidated": true})
}
