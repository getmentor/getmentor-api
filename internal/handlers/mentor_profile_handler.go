package handlers

import (
	"errors"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MentorProfileHandler handles session-authenticated profile endpoints
type MentorProfileHandler struct {
	mentorService    services.MentorServiceInterface
	profileService   services.ProfileServiceInterface
	lifecycleService services.MentorLifecycleServiceInterface
	cookieDomain     string
	cookieSecure     bool
}

// NewMentorProfileHandler creates a new MentorProfileHandler.
// Cookie settings are used to clear the session cookie after a profile delete.
func NewMentorProfileHandler(
	mentorService services.MentorServiceInterface,
	profileService services.ProfileServiceInterface,
	lifecycleService services.MentorLifecycleServiceInterface,
	cookieDomain string,
	cookieSecure bool,
) *MentorProfileHandler {

	return &MentorProfileHandler{
		mentorService:    mentorService,
		profileService:   profileService,
		lifecycleService: lifecycleService,
		cookieDomain:     cookieDomain,
		cookieSecure:     cookieSecure,
	}
}

// GetProfile handles GET /api/v1/mentor/profile
// Returns the authenticated mentor's full profile including secure fields
func (h *MentorProfileHandler) GetProfile(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	mentor, err := h.mentorService.GetMentorByMentorId(c.Request.Context(), session.MentorID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		respondError(c, http.StatusNotFound, "Profile not found", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"mentor": mentor})
}

// UpdateProfile handles POST /api/v1/mentor/profile
// Updates the authenticated mentor's profile
func (h *MentorProfileHandler) UpdateProfile(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	var req models.SaveProfileRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	err = h.profileService.SaveProfileByMentorId(c.Request.Context(), session.MentorID, &req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to update profile", err)
		return
	}

	logger.Info("Profile updated via session",
		zap.String("mentor_id", session.MentorID),
		zap.String("mentor_name", session.Name))

	c.JSON(http.StatusOK, models.SaveProfileResponse{Success: true})
}

// UploadPicture handles POST /api/v1/mentor/profile/picture
// Uploads a new profile picture for the authenticated mentor
func (h *MentorProfileHandler) UploadPicture(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	var req models.UploadProfilePictureRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	mentor, err := h.mentorService.GetMentorByMentorId(c.Request.Context(), session.MentorID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to fetch mentor", err)
		return
	}

	imageURL, err := h.profileService.UploadPictureByMentorId(
		c.Request.Context(),
		session.MentorID,
		mentor.Slug,
		&req,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to upload picture", err)
		return
	}

	logger.Info("Profile picture uploaded via session",
		zap.String("mentor_id", session.MentorID),
		zap.String("mentor_name", session.Name),
		zap.String("image_url", imageURL))

	c.JSON(http.StatusOK, models.UploadProfilePictureResponse{
		Success:  true,
		Message:  "Profile picture uploaded successfully",
		ImageURL: imageURL,
	})
}

// DeleteProfile handles POST /api/v1/mentor/profile/delete
// Soft-deletes the authenticated mentor's profile and logs them out.
// Body: {"slug": "<current slug>"} to confirm the action.
func (h *MentorProfileHandler) DeleteProfile(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	var req models.MentorDeleteRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondErrorWithDetails(c, http.StatusBadRequest, "Invalid request body", gin.H{"message": bindErr.Error()}, bindErr)
		return
	}

	err = h.lifecycleService.DeleteMentor(c.Request.Context(), session.MentorID, req.Slug, services.MentorLifecycleInitiatorMentor, "")
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDeleteConfirmationMismatch):
			respondError(c, http.StatusBadRequest, "Slug does not match", err)
		case errors.Is(err, repository.ErrMentorAlreadyDeleted):
			middleware.ClearSessionCookie(c, h.cookieDomain, h.cookieSecure)
			respondError(c, http.StatusConflict, "Profile is already deleted", err)
		default:
			respondError(c, http.StatusInternalServerError, "Failed to delete profile", err)
		}
		return
	}

	middleware.ClearSessionCookie(c, h.cookieDomain, h.cookieSecure)

	logger.Info("Profile deleted via session",
		zap.String("mentor_id", session.MentorID),
		zap.String("mentor_name", session.Name))

	c.JSON(http.StatusOK, models.MentorDeleteResponse{Success: true})
}
