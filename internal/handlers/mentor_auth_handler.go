package handlers

import (
	"errors"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

// MentorAuthHandler handles mentor authentication endpoints
type MentorAuthHandler struct {
	service services.MentorAuthServiceInterface
}

// NewMentorAuthHandler creates a new MentorAuthHandler
func NewMentorAuthHandler(service services.MentorAuthServiceInterface) *MentorAuthHandler {
	return &MentorAuthHandler{
		service: service,
	}
}

// RequestLogin handles POST /api/v1/auth/mentor/request-login
// Generates a login token and sends it via email
func (h *MentorAuthHandler) RequestLogin(c *gin.Context) {
	var req models.RequestLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Validation failed",
			"details": []gin.H{
				{"field": "email", "message": "Invalid email format"},
			},
		})
		return
	}

	resp, err := h.service.RequestLogin(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, services.ErrMentorNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Ментор с таким email не найден",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Ошибка при отправке ссылки для входа",
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// VerifyLogin handles POST /api/v1/auth/mentor/verify
// Verifies the login token and creates a session
func (h *MentorAuthHandler) VerifyLogin(c *gin.Context) {
	var req models.VerifyLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid token format",
		})
		return
	}

	session, jwtToken, err := h.service.VerifyLogin(c.Request.Context(), req.Token)
	if err != nil {
		if errors.Is(err, services.ErrInvalidLoginToken) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Недействительный или просроченный токен",
			})
			return
		}
		if errors.Is(err, services.ErrJWTSecretNotSet) {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Сервис временно недоступен",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка при проверке токена",
		})
		return
	}

	// Set session cookie
	middleware.SetSessionCookie(
		c,
		jwtToken,
		h.service.GetSessionTTL(),
		h.service.GetCookieDomain(),
		h.service.GetCookieSecure(),
	)

	c.JSON(http.StatusOK, models.VerifyLoginResponse{
		Success: true,
		Session: session,
	})
}

// Logout handles POST /api/v1/auth/mentor/logout
// Clears the session cookie
func (h *MentorAuthHandler) Logout(c *gin.Context) {
	middleware.ClearSessionCookie(
		c,
		h.service.GetCookieDomain(),
		h.service.GetCookieSecure(),
	)

	c.JSON(http.StatusOK, models.LogoutResponse{
		Success: true,
	})
}

// GetSession handles GET /api/v1/auth/mentor/session
// Returns the current session info (for session validation)
func (h *MentorAuthHandler) GetSession(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Not authenticated",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"session": session,
	})
}
