package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/gin-gonic/gin"
)

type AdminMentorsHandler struct {
	service services.AdminMentorsServiceInterface
}

func NewAdminMentorsHandler(service services.AdminMentorsServiceInterface) *AdminMentorsHandler {
	return &AdminMentorsHandler{service: service}
}

func (h *AdminMentorsHandler) ListMentors(c *gin.Context) {
	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	filter := models.MentorModerationFilter(c.DefaultQuery("status", string(models.MentorModerationFilterPending)))
	if !filter.IsValid() {
		respondError(c, http.StatusBadRequest, "Invalid status filter", errors.New("status must be pending, approved, declined, or deleted"))
		return
	}

	mentors, err := h.service.ListMentors(c.Request.Context(), session, filter)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorsListResponse{
		Mentors: mentors,
		Total:   len(mentors),
	})
}

func (h *AdminMentorsHandler) GetMentor(c *gin.Context) {
	h.withAdminMentor(c, h.service.GetMentor)
}

func (h *AdminMentorsHandler) UpdateMentor(c *gin.Context) {
	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	mentorID := c.Param("id")
	if mentorID == "" {
		respondError(c, http.StatusBadRequest, "Invalid mentor ID", errors.New("missing route param: id"))
		return
	}

	var req models.AdminMentorProfileUpdateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	mentor, err := h.service.UpdateMentorProfile(c.Request.Context(), session, mentorID, &req)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
}

func (h *AdminMentorsHandler) ApproveMentor(c *gin.Context) {
	h.withAdminMentor(c, h.service.ApproveMentor)
}

func (h *AdminMentorsHandler) DeclineMentor(c *gin.Context) {
	h.withAdminMentor(c, h.service.DeclineMentor)
}

// DeleteMentor handles POST /api/v1/admin/mentors/:id/delete
// Body: {"slug": "<current mentor slug>"} to confirm the action.
func (h *AdminMentorsHandler) DeleteMentor(c *gin.Context) {
	var req models.MentorDeleteRequest
	session, mentorID, ok := h.parseAdminMentorRequest(c, &req)
	if !ok {
		return
	}

	mentor, err := h.service.DeleteMentor(c.Request.Context(), session, mentorID, req.Slug)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
}

// RestoreMentor handles POST /api/v1/admin/mentors/:id/restore
func (h *AdminMentorsHandler) RestoreMentor(c *gin.Context) {
	h.withAdminMentor(c, h.service.RestoreMentor)
}

func (h *AdminMentorsHandler) withAdminMentor(
	c *gin.Context,
	action func(context.Context, *models.AdminSession, string) (*models.AdminMentorDetails, error),
) {

	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	mentorID := c.Param("id")
	if mentorID == "" {
		respondError(c, http.StatusBadRequest, "Invalid mentor ID", errors.New("missing route param: id"))
		return
	}

	mentor, err := action(c.Request.Context(), session, mentorID)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
}

func (h *AdminMentorsHandler) UpdateMentorStatus(c *gin.Context) {
	var req models.AdminMentorStatusUpdateRequest
	session, mentorID, ok := h.parseAdminMentorRequest(c, &req)
	if !ok {
		return
	}

	mentor, err := h.service.UpdateMentorStatus(c.Request.Context(), session, mentorID, req.Status)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
}

// parseAdminMentorRequest reads the admin session, the :id route param and the JSON body.
// On failure it writes the error response and returns ok=false.
func (h *AdminMentorsHandler) parseAdminMentorRequest(c *gin.Context, body interface{}) (*models.AdminSession, string, bool) {
	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return nil, "", false
	}

	mentorID := c.Param("id")
	if mentorID == "" {
		respondError(c, http.StatusBadRequest, "Invalid mentor ID", errors.New("missing route param: id"))
		return nil, "", false
	}

	if bindErr := c.ShouldBindJSON(body); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return nil, "", false
	}

	return session, mentorID, true
}

func (h *AdminMentorsHandler) UploadMentorPicture(c *gin.Context) {
	session, err := middleware.GetAdminSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	mentorID := c.Param("id")
	if mentorID == "" {
		respondError(c, http.StatusBadRequest, "Invalid mentor ID", errors.New("missing route param: id"))
		return
	}

	var req models.UploadProfilePictureRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	imageURL, err := h.service.UploadMentorPicture(c.Request.Context(), session, mentorID, &req)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.UploadProfilePictureResponse{
		Success:  true,
		Message:  "Profile picture uploaded successfully",
		ImageURL: imageURL,
	})
}

func (h *AdminMentorsHandler) respondServiceError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrAdminForbiddenAction) {
		respondError(c, http.StatusForbidden, "Access denied", err)
		return
	}
	if errors.Is(err, services.ErrMentorDeleted) {
		respondError(c, http.StatusConflict, "Mentor is deleted", err)
		return
	}
	if errors.Is(err, repository.ErrMentorAlreadyDeleted) {
		respondError(c, http.StatusConflict, "Mentor is already deleted", err)
		return
	}
	var emailConflict *repository.MentorEmailConflictError
	if errors.As(err, &emailConflict) {
		respondErrorWithDetails(c, http.StatusConflict, "Another mentor profile with this email already exists", models.AdminMentorEmailConflictDetails{
			Code:     "email_conflict",
			MentorID: emailConflict.MentorID,
			Name:     emailConflict.Name,
			Slug:     emailConflict.Slug,
			Status:   emailConflict.Status,
		}, err)
		return
	}
	if errors.Is(err, repository.ErrMentorNotDeleted) {
		respondError(c, http.StatusConflict, "Mentor is not deleted", err)
		return
	}
	if errors.Is(err, services.ErrDeleteConfirmationMismatch) {
		respondError(c, http.StatusBadRequest, "Slug does not match", err)
		return
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") {
		respondError(c, http.StatusNotFound, "Mentor not found", err)
		return
	}

	if strings.Contains(msg, "unsupported") || strings.Contains(msg, "required") || strings.Contains(msg, "available only") {
		respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	respondError(c, http.StatusInternalServerError, "Internal server error", err)
}
