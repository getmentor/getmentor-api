package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
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
		respondError(c, http.StatusBadRequest, "Invalid status filter", errors.New("status must be pending, approved, or declined"))
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

	mentor, err := h.service.GetMentor(c.Request.Context(), session, mentorID)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
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

	mentor, err := h.service.ApproveMentor(c.Request.Context(), session, mentorID)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
}

func (h *AdminMentorsHandler) DeclineMentor(c *gin.Context) {
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

	mentor, err := h.service.DeclineMentor(c.Request.Context(), session, mentorID)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
}

func (h *AdminMentorsHandler) UpdateMentorStatus(c *gin.Context) {
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

	var req models.AdminMentorStatusUpdateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	mentor, err := h.service.UpdateMentorStatus(c.Request.Context(), session, mentorID, req.Status)
	if err != nil {
		h.respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.AdminMentorResponse{Mentor: mentor})
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
