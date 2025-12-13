package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/getmentor/getmentor-api/internal/database/postgres"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

// BotHandler handles requests from the Telegram bot
type BotHandler struct {
	service *services.BotService
}

// NewBotHandler creates a new bot handler
func NewBotHandler(service *services.BotService) *BotHandler {
	return &BotHandler{
		service: service,
	}
}

// parseIDParam parses an integer ID from a path parameter
func parseIDParam(c *gin.Context, paramName string) (int, bool) { //nolint:unparam // paramName kept for future flexibility
	idStr := c.Param(paramName)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return 0, false
	}
	return id, true
}

// GetMentorByTgSecret returns a mentor by their TgSecret authentication code
// POST /api/v1/bot/auth
// Body: {"code": "ABC12345"}
func (h *BotHandler) GetMentorByTgSecret(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}

	mentor, err := h.service.GetMentorByTgSecret(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mentor not found"})
		return
	}

	c.JSON(http.StatusOK, mentor)
}

// GetMentorByTelegramChatID returns a mentor by their Telegram chat ID
// GET /api/v1/bot/mentor/chat/:chatId
func (h *BotHandler) GetMentorByTelegramChatID(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat ID is required"})
		return
	}

	mentor, err := h.service.GetMentorByTelegramChatID(c.Request.Context(), chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mentor not found"})
		return
	}

	c.JSON(http.StatusOK, mentor)
}

// GetMentorByID returns a mentor by their numeric ID
// GET /api/v1/bot/mentor/:id
func (h *BotHandler) GetMentorByID(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	mentor, err := h.service.GetMentorByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mentor not found"})
		return
	}

	c.JSON(http.StatusOK, mentor)
}

// SetMentorTelegramChatID sets the Telegram chat ID for a mentor
// POST /api/v1/bot/mentor/:id/telegram
// Body: {"chatId": "123456789"}
//
//nolint:dupl // Similar structure to SetMentorStatus but different request/service
func (h *BotHandler) SetMentorTelegramChatID(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		ChatID string `json:"chatId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chatId is required"})
		return
	}

	if err := h.service.SetMentorTelegramChatID(c.Request.Context(), id, req.ChatID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SetMentorStatus updates the status of a mentor
// POST /api/v1/bot/mentor/:id/status
// Body: {"status": "active"}
//
//nolint:dupl // Similar structure to SetMentorTelegramChatID but different request/service
func (h *BotHandler) SetMentorStatus(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	if err := h.service.SetMentorStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// getMentorRequests is a helper for fetching mentor requests
func (h *BotHandler) getMentorRequests(c *gin.Context, fetchFn func(context.Context, int) ([]*postgres.BotClientRequest, error)) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	requests, err := fetchFn(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

// GetActiveRequestsForMentor returns active requests for a mentor
// GET /api/v1/bot/mentor/:id/requests/active
func (h *BotHandler) GetActiveRequestsForMentor(c *gin.Context) {
	h.getMentorRequests(c, h.service.GetActiveRequestsForMentor)
}

// GetArchivedRequestsForMentor returns archived requests for a mentor
// GET /api/v1/bot/mentor/:id/requests/archived
func (h *BotHandler) GetArchivedRequestsForMentor(c *gin.Context) {
	h.getMentorRequests(c, h.service.GetArchivedRequestsForMentor)
}

// GetRequestByID returns a single request by ID
// GET /api/v1/bot/request/:id
func (h *BotHandler) GetRequestByID(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	request, err := h.service.GetRequestByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}

	c.JSON(http.StatusOK, request)
}

// UpdateRequestStatus updates the status of a client request
// POST /api/v1/bot/request/:id/status
// Body: {"status": "contacted"}
//
//nolint:dupl // Similar structure to SetMentorStatus but for request entities
func (h *BotHandler) UpdateRequestStatus(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	if err := h.service.UpdateRequestStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
