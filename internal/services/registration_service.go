package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/azure"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/recaptcha"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

// RegistrationService handles mentor registration
type RegistrationService struct {
	mentorRepo        *repository.MentorRepository
	azureClient       *azure.StorageClient
	config            *config.Config
	httpClient        httpclient.Client
	recaptchaVerifier *recaptcha.Verifier
}

// NewRegistrationService creates a new registration service instance
func NewRegistrationService(
	mentorRepo *repository.MentorRepository,
	azureClient *azure.StorageClient,
	cfg *config.Config,
	httpClient httpclient.Client,
) *RegistrationService {

	return &RegistrationService{
		mentorRepo:        mentorRepo,
		azureClient:       azureClient,
		config:            cfg,
		httpClient:        httpClient,
		recaptchaVerifier: recaptcha.NewVerifier(cfg.ReCAPTCHA.SecretKey, httpClient),
	}
}

// RegisterMentor handles the complete mentor registration flow
func (s *RegistrationService) RegisterMentor(ctx context.Context, req *models.RegisterMentorRequest) (*models.RegisterMentorResponse, error) {
	// 1. Verify ReCAPTCHA
	if err := s.recaptchaVerifier.Verify(req.RecaptchaToken); err != nil {
		metrics.MentorRegistrations.WithLabelValues("captcha_failed").Inc()
		logger.Warn("ReCAPTCHA verification failed", zap.Error(err))
		return &models.RegisterMentorResponse{
			Success: false,
			Error:   "Captcha verification failed",
		}, nil
	}

	// 2. Clean telegram handle (remove @ and t.me/ prefix)
	telegram := strings.TrimSpace(req.Telegram)
	telegram = strings.TrimPrefix(telegram, "@")
	telegram = strings.TrimPrefix(telegram, "https://t.me/")
	telegram = strings.TrimPrefix(telegram, "t.me/")

	// 3. Get tag IDs for selected tags
	var tagIDs []string
	for _, tagName := range req.Tags {
		tagID, err := s.mentorRepo.GetTagIDByName(ctx, tagName)
		if err == nil && tagID != "" {
			tagIDs = append(tagIDs, tagID)
		} else {
			logger.Warn("Tag not found", zap.String("tag_name", tagName))
		}
	}

	// 4. Create mentor record in PostgreSQL
	fields := map[string]interface{}{
		"name":         strings.TrimSpace(req.Name),
		"email":        req.Email,
		"telegram":     telegram,
		"job_title":    req.Job,
		"workplace":    req.Workplace,
		"experience":   req.Experience,
		"price":        req.Price,
		"about":        req.About,
		"details":      req.Description,
		"competencies": req.Competencies,
		"status":       "pending",
	}

	if req.CalendarURL != "" {
		fields["calendar_url"] = req.CalendarURL
	}

	// Note: Tags will be inserted separately into mentor_tags table
	// This is handled by the repository CreateMentor method

	mentorID, legacyID, err := s.mentorRepo.CreateMentor(ctx, fields)
	if err != nil {
		metrics.MentorRegistrations.WithLabelValues("db_error").Inc()
		logger.Error("Failed to create mentor in database", zap.Error(err))
		return &models.RegisterMentorResponse{
			Success: false,
			Error:   "Failed to create mentor profile",
		}, nil
	}

	logger.Info("Mentor created in database",
		zap.String("mentor_id", mentorID),
		zap.Int("legacy_id", legacyID),
		zap.String("email", req.Email))

	// Set mentor tags if any were provided
	if len(tagIDs) > 0 {
		if err := s.mentorRepo.UpdateMentorTags(ctx, mentorID, tagIDs); err != nil {
			logger.Error("Failed to set mentor tags", zap.Error(err))
			// Don't fail registration if tags fail - continue
		}
	}

	// 5. Upload profile picture (non-blocking on failure)
	if err := s.uploadProfilePicture(ctx, legacyID, mentorID, &req.ProfilePicture); err != nil {
		logger.Error("Failed to upload profile picture",
			zap.Error(err),
			zap.String("mentor_id", mentorID),
			zap.Int("legacy_id", legacyID))
		// Don't fail registration if image upload fails - can upload later via edit profile
	} else {
		logger.Info("Profile picture uploaded",
			zap.String("mentor_id", mentorID),
			zap.Int("legacy_id", legacyID))
	}

	// 6. Trigger mentor created webhook (non-blocking)
	trigger.CallAsync(s.config.EventTriggers.MentorCreatedTriggerURL, mentorID, s.httpClient)

	metrics.MentorRegistrations.WithLabelValues("success").Inc()

	return &models.RegisterMentorResponse{
		Success:  true,
		Message:  "Registration successful. We'll review your application and contact you soon.",
		MentorID: legacyID, // Return legacy ID for backwards compatibility
	}, nil
}

// uploadProfilePicture handles the image upload to Azure Storage
func (s *RegistrationService) uploadProfilePicture(ctx context.Context, legacyID int, mentorID string, picture *models.ProfilePictureData) error {
	// Validate file type
	if err := s.azureClient.ValidateImageType(picture.ContentType); err != nil {
		return err
	}

	// Validate file size
	if err := s.azureClient.ValidateImageSize(picture.Image); err != nil {
		return err
	}

	// Generate filename using legacy ID for backwards compatibility
	fileName := s.azureClient.GenerateFileName(legacyID, picture.FileName)

	// Upload to Azure
	imageURL, err := s.azureClient.UploadImage(ctx, picture.Image, fileName, picture.ContentType)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Update mentor record with image URL
	if err := s.mentorRepo.UpdateImage(ctx, mentorID, imageURL); err != nil {
		logger.Error("Failed to update image URL in database",
			zap.Error(err),
			zap.String("mentor_id", mentorID))
		// Image is uploaded but not linked - admin can fix manually
	}

	return nil
}
