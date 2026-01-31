package handlers

import (
	"net/http"

	"github.com/getmentor/getmentor-api/internal/middleware"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MentorProfileHandler handles session-authenticated profile endpoints
type MentorProfileHandler struct {
	mentorService  services.MentorServiceInterface
	profileService services.ProfileServiceInterface
}

// NewMentorProfileHandler creates a new MentorProfileHandler
func NewMentorProfileHandler(
	mentorService services.MentorServiceInterface,
	profileService services.ProfileServiceInterface,
) *MentorProfileHandler {

	return &MentorProfileHandler{
		mentorService:  mentorService,
		profileService: profileService,
	}
}

// GetProfile handles GET /api/v1/mentor/profile
// Returns the authenticated mentor's full profile including secure fields
func (h *MentorProfileHandler) GetProfile(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Fetch mentor with secure fields (showHidden: true)
	mentor, err := h.mentorService.GetMentorByMentorId(c.Request.Context(), session.MentorID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		logger.Warn("Failed to fetch mentor profile",
			zap.String("mentor_id", session.MentorID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Profile not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mentor": mentor})
}

// UpdateProfile handles POST /api/v1/mentor/profile
// Updates the authenticated mentor's profile
func (h *MentorProfileHandler) UpdateProfile(c *gin.Context) {
	session, err := middleware.GetMentorSession(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.SaveProfileRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		logger.Warn("Invalid profile update request",
			zap.String("mentor_id", session.MentorID),
			zap.Error(bindErr))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": gin.H{"message": bindErr.Error()},
		})
		return
	}

	err = h.profileService.SaveProfileByMentorId(c.Request.Context(), session.MentorID, &req)
	if err != nil {
		logger.Error("Failed to update profile",
			zap.String("mentor_id", session.MentorID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.UploadProfilePictureRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		logger.Warn("Invalid picture upload request",
			zap.String("mentor_id", session.MentorID),
			zap.Error(bindErr))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": gin.H{"message": bindErr.Error()},
		})
		return
	}

	// Get mentor to fetch slug for storage path
	mentor, err := h.mentorService.GetMentorByMentorId(c.Request.Context(), session.MentorID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		logger.Error("Failed to fetch mentor for picture upload",
			zap.String("mentor_id", session.MentorID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch mentor"})
		return
	}

	imageURL, err := h.profileService.UploadPictureByMentorId(
		c.Request.Context(),
		session.MentorID,
		mentor.Slug,
		&req,
	)
	if err != nil {
		logger.Error("Failed to upload profile picture",
			zap.String("mentor_id", session.MentorID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload picture"})
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
