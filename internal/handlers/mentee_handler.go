package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

// MenteeServiceInterface is what the mentee handler needs.
type MenteeServiceInterface interface {
	RequestLogin(ctx context.Context, email string) (*models.RequestLoginResponse, error)
	VerifyLogin(ctx context.Context, token string) (*models.MenteeSession, string, error)
	Logout(ctx context.Context, token string) error
	ListRequests(ctx context.Context, menteeID string) (*models.MenteeRequestsResponse, error)
	SubmitReview(ctx context.Context, menteeID string, req *models.MenteeSubmitReviewRequest) (string, error)
	GetSessionTTL() int
	GetCookieDomain() string
	GetCookieSecure() bool
}

// MenteeHandler serves mentee login and the read-only mentee dashboard.
// Errors attached to the context never include the email or request id.
type MenteeHandler struct {
	service MenteeServiceInterface
}

// NewMenteeHandler creates a new MenteeHandler.
func NewMenteeHandler(service MenteeServiceInterface) *MenteeHandler {
	return &MenteeHandler{service: service}
}

// RequestLogin handles POST /api/v1/auth/mentee/request-login.
// Known and unknown emails get the same 200 response.
func (h *MenteeHandler) RequestLogin(c *gin.Context) {
	var req models.MenteeRequestLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Validation failed", []gin.H{
			{"field": "email", "message": "Invalid email format"},
		}, errors.New("invalid mentee login payload"))
		return
	}

	resp, err := h.service.RequestLogin(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, services.ErrMenteeRateLimited) {
			respondError(c, http.StatusTooManyRequests, "Rate limit exceeded. Please try again later.", err)
			return
		}
		respondError(c, http.StatusInternalServerError, "Error while sending auth link", err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// VerifyLogin handles POST /api/v1/auth/mentee/verify.
func (h *MenteeHandler) VerifyLogin(c *gin.Context) {
	var req models.MenteeVerifyLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid token format", errors.New("invalid mentee verify payload"))
		return
	}

	session, token, err := h.service.VerifyLogin(c.Request.Context(), req.Token)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMenteeInvalidLoginToken):
			respondError(c, http.StatusUnauthorized, "Invalid token", err)
		case errors.Is(err, services.ErrMenteeNotConfigured):
			respondError(c, http.StatusInternalServerError, "Service temporarily unavailable", err)
		default:
			respondError(c, http.StatusInternalServerError, "Error while verifying token", err)
		}
		return
	}

	middleware.SetMenteeSessionCookie(c, token, h.service.GetSessionTTL(), h.service.GetCookieDomain(), h.service.GetCookieSecure())
	c.JSON(http.StatusOK, models.MenteeVerifyLoginResponse{Success: true, Session: session})
}

// Logout handles POST /api/v1/auth/mentee/logout. It revokes every session of the
// mentee (session_version++), not only this cookie, then clears the cookie.
func (h *MenteeHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(middleware.MenteeSessionCookieName) //nolint:errcheck // missing cookie = nothing to revoke
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		respondError(c, http.StatusServiceUnavailable, "Service temporarily unavailable", err)
		return
	}
	middleware.ClearMenteeSessionCookie(c, h.service.GetCookieDomain(), h.service.GetCookieSecure())
	c.JSON(http.StatusOK, models.LogoutResponse{Success: true})
}

// GetSession handles GET /api/v1/auth/mentee/session (behind MenteeSessionMiddleware).
func (h *MenteeHandler) GetSession(c *gin.Context) {
	session, err := middleware.GetMenteeSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Not authenticated", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "session": session})
}

// ListRequests handles GET /api/v1/mentee/requests.
func (h *MenteeHandler) ListRequests(c *gin.Context) {
	session, err := middleware.GetMenteeSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Not authenticated", err)
		return
	}
	resp, err := h.service.ListRequests(c.Request.Context(), session.MenteeID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to load requests", err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// SubmitReview handles POST /api/v1/mentee/reviews. The request id is in the body.
func (h *MenteeHandler) SubmitReview(c *gin.Context) {
	session, err := middleware.GetMenteeSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Not authenticated", err)
		return
	}

	var req models.MenteeSubmitReviewRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Validation failed", ParseValidationErrors(bindErr), errors.New("invalid mentee review payload"))
		return
	}

	reviewID, err := h.service.SubmitReview(c.Request.Context(), session.MenteeID, &req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMenteeReviewCaptchaError):
			respondError(c, http.StatusBadRequest, "Captcha verification failed", err)
		case errors.Is(err, services.ErrMenteeRequestNotFound):
			// Same response for "not yours" and "does not exist".
			respondError(c, http.StatusNotFound, "Заявка не найдена", err)
		case errors.Is(err, services.ErrMenteeReviewNotAllowed), errors.Is(err, services.ErrMenteeReviewExists):
			respondError(c, http.StatusConflict, "Отзыв уже оставлен или заявка ещё не завершена", err)
		default:
			respondError(c, http.StatusInternalServerError, "Failed to save review", err)
		}
		return
	}
	c.JSON(http.StatusOK, models.SubmitReviewResponse{Success: true, ReviewID: reviewID})
}
