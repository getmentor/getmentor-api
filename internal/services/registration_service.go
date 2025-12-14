package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/azure"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

// RegistrationService handles mentor registration
type RegistrationService struct {
	mentorRepo  *repository.MentorRepository
	azureClient *azure.StorageClient
	config      *config.Config
	httpClient  httpclient.Client
}

// NewRegistrationService creates a new registration service instance
func NewRegistrationService(
	mentorRepo *repository.MentorRepository,
	azureClient *azure.StorageClient,
	cfg *config.Config,
	httpClient httpclient.Client,
) *RegistrationService {
	return &RegistrationService{
		mentorRepo:  mentorRepo,
		azureClient: azureClient,
		config:      cfg,
		httpClient:  httpClient,
	}
}

// RegisterMentor handles the complete mentor registration flow
func (s *RegistrationService) RegisterMentor(ctx context.Context, req *models.RegisterMentorRequest) (*models.RegisterMentorResponse, error) {
	// 1. Verify ReCAPTCHA
	if err := s.verifyRecaptcha(req.RecaptchaToken); err != nil {
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
	tagIDs := []string{}
	for _, tagName := range req.Tags {
		tagID, err := s.mentorRepo.GetTagIDByName(ctx, tagName)
		if err == nil && tagID != "" {
			tagIDs = append(tagIDs, tagID)
		} else {
			logger.Warn("Tag not found", zap.String("tag_name", tagName))
		}
	}

	// 4. Create minimal Airtable record
	airtableFields := map[string]interface{}{
		"Name":         strings.TrimSpace(req.Name),
		"Email":        req.Email,
		"Telegram":     telegram,
		"JobTitle":     req.Job,
		"Workplace":    req.Workplace,
		"Experience":   req.Experience,
		"Price":        req.Price,
		"Tags Links":   tagIDs,
		"About":        req.About,
		"Details":      req.Description,
		"Competencies": req.Competencies,
	}

	if req.CalendarURL != "" {
		airtableFields["Calendly Url"] = req.CalendarURL
	}

	recordID, mentorID, err := s.mentorRepo.CreateMentor(ctx, airtableFields)
	if err != nil {
		metrics.MentorRegistrations.WithLabelValues("airtable_error").Inc()
		logger.Error("Failed to create mentor in Airtable", zap.Error(err))
		return &models.RegisterMentorResponse{
			Success: false,
			Error:   "Failed to create mentor profile",
		}, nil
	}

	logger.Info("Mentor created in Airtable",
		zap.String("record_id", recordID),
		zap.Int("mentor_id", mentorID),
		zap.String("email", req.Email))

	// 5. Upload profile picture (non-blocking on failure)
	if err := s.uploadProfilePicture(ctx, mentorID, recordID, &req.ProfilePicture); err != nil {
		logger.Error("Failed to upload profile picture",
			zap.Error(err),
			zap.Int("mentor_id", mentorID))
		// Don't fail registration if image upload fails - can upload later via edit profile
	} else {
		logger.Info("Profile picture uploaded", zap.Int("mentor_id", mentorID))
	}

	// 6. Call new-mentor-watcher Azure Function (non-blocking on failure)
	if err := s.triggerNewMentorWatcher(recordID); err != nil {
		logger.Error("Failed to trigger new-mentor-watcher",
			zap.Error(err),
			zap.String("record_id", recordID))
		// Don't fail registration - admin can manually trigger if needed
	}

	metrics.MentorRegistrations.WithLabelValues("success").Inc()

	return &models.RegisterMentorResponse{
		Success:  true,
		Message:  "Registration successful. We'll review your application and contact you soon.",
		MentorID: mentorID,
	}, nil
}

// uploadProfilePicture handles the image upload to Azure Storage
func (s *RegistrationService) uploadProfilePicture(ctx context.Context, mentorID int, recordID string, picture *models.ProfilePictureData) error {
	// Validate file type
	if err := s.azureClient.ValidateImageType(picture.ContentType); err != nil {
		return err
	}

	// Validate file size
	if err := s.azureClient.ValidateImageSize(picture.Image); err != nil {
		return err
	}

	// Generate filename
	fileName := s.azureClient.GenerateFileName(mentorID, picture.FileName)

	// Upload to Azure
	imageURL, err := s.azureClient.UploadImage(picture.Image, fileName, picture.ContentType)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Update Airtable record with image URL
	if err := s.mentorRepo.UpdateImage(ctx, recordID, imageURL); err != nil {
		logger.Error("Failed to update image URL in Airtable",
			zap.Error(err),
			zap.String("record_id", recordID))
		// Image is uploaded but not linked - admin can fix manually
	}

	return nil
}

// triggerNewMentorWatcher calls the Azure Function to process new mentor
func (s *RegistrationService) triggerNewMentorWatcher(recordID string) error {
	// Build URL with mentorId query parameter
	funcURL := s.config.AzureFunctions.NewMentorWatcherURL
	if funcURL == "" {
		logger.Warn("Azure Function URL not configured, skipping new-mentor-watcher trigger")
		return nil
	}

	targetURL := fmt.Sprintf("%s?mentorId=%s", funcURL, recordID)

	// Make HTTP GET request to trigger the function
	resp, err := s.httpClient.Get(targetURL)
	if err != nil {
		return fmt.Errorf("failed to call new-mentor-watcher: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("new-mentor-watcher returned status %d", resp.StatusCode)
	}

	logger.Info("new-mentor-watcher triggered successfully", zap.String("record_id", recordID))
	return nil
}

// verifyRecaptcha verifies the ReCAPTCHA token with Google's API
func (s *RegistrationService) verifyRecaptcha(token string) error {
	// Prepare form data
	data := url.Values{}
	data.Set("secret", s.config.ReCAPTCHA.SecretKey)
	data.Set("response", token)

	// Send POST request to Google's verification endpoint
	resp, err := s.httpClient.Post(
		"https://www.google.com/recaptcha/api/siteverify",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("failed to verify recaptcha: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result models.ReCAPTCHAResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode recaptcha response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("recaptcha verification failed")
	}

	return nil
}
