package services

import (
	"context"
	"fmt"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/azure"
	apperrors "github.com/getmentor/getmentor-api/pkg/errors"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

type ProfileService struct {
	mentorRepo  *repository.MentorRepository
	azureClient *azure.StorageClient
	config      *config.Config
	httpClient  httpclient.Client
}

func NewProfileService(
	mentorRepo *repository.MentorRepository,
	azureClient *azure.StorageClient,
	cfg *config.Config,
	httpClient httpclient.Client,
) *ProfileService {

	return &ProfileService{
		mentorRepo:  mentorRepo,
		azureClient: azureClient,
		config:      cfg,
		httpClient:  httpClient,
	}
}

// SaveProfile is deprecated - token-based auth has been replaced with login tokens
// This method is kept for backwards compatibility but should not be used
func (s *ProfileService) SaveProfile(ctx context.Context, id int, token string, req *models.SaveProfileRequest) error {
	// Get mentor
	mentor, err := s.mentorRepo.GetByID(ctx, id, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return apperrors.NotFoundError("mentor")
	}

	// NOTE: AuthToken field has been removed - this method is deprecated
	// Use SaveProfileByMentorId with session-based auth instead
	_ = token // Silence unused variable warning

	// Get sponsor tags to preserve them
	sponsorTags := s.getSponsorTags()
	preservedSponsors := []string{}
	for _, tag := range mentor.Tags {
		if sponsorTags[tag] {
			preservedSponsors = append(preservedSponsors, tag)
		}
	}

	// Filter out sponsor tags from user input (they shouldn't be able to modify these)
	userTags := []string{}
	for _, tag := range req.Tags {
		if !sponsorTags[tag] {
			userTags = append(userTags, tag)
		}
	}

	// Merge user tags with preserved sponsor tags
	userTags = append(userTags, preservedSponsors...)

	// Get tag IDs
	tagIDs := []string{}
	for _, tagName := range userTags {
		tagID, err := s.mentorRepo.GetTagIDByName(ctx, tagName)
		if err == nil && tagID != "" {
			tagIDs = append(tagIDs, tagID)
		}
	}

	// Prepare updates with PostgreSQL column names
	updates := map[string]interface{}{
		"name":         req.Name,
		"job_title":    req.Job,
		"workplace":    req.Workplace,
		"experience":   req.Experience,
		"price":        req.Price,
		"details":      req.Description,
		"about":        req.About,
		"competencies": req.Competencies,
	}

	if req.CalendarURL != "" {
		updates["calendar_url"] = req.CalendarURL
	}

	// Note: Tags will be handled separately via mentor_tags table
	_ = tagIDs // TODO: Implement tag updates in repository

	// Update in database
	if err := s.mentorRepo.Update(ctx, mentor.MentorID, updates); err != nil {
		metrics.ProfileUpdates.WithLabelValues("error").Inc()
		logger.Error("Failed to update mentor profile", zap.Error(err), zap.Int("mentor_id", id))
		return fmt.Errorf("failed to update profile")
	}

	metrics.ProfileUpdates.WithLabelValues("success").Inc()
	logger.Info("Mentor profile updated", zap.Int("mentor_id", id))

	return nil
}

// UploadProfilePicture is deprecated - token-based auth has been replaced with login tokens
// This method is kept for backwards compatibility but should not be used
func (s *ProfileService) UploadProfilePicture(ctx context.Context, id int, token string, req *models.UploadProfilePictureRequest) (string, error) {
	// Get mentor
	mentor, err := s.mentorRepo.GetByID(ctx, id, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return "", apperrors.NotFoundError("mentor")
	}

	// NOTE: AuthToken field has been removed - this method is deprecated
	// Use UploadPictureByMentorId with session-based auth instead
	_ = token // Silence unused variable warning

	// Validate file type
	if typeErr := s.azureClient.ValidateImageType(req.ContentType); typeErr != nil {
		return "", typeErr
	}

	// Validate file size
	if sizeErr := s.azureClient.ValidateImageSize(req.Image); sizeErr != nil {
		return "", sizeErr
	}

	// Generate filename
	fileName := s.azureClient.GenerateFileName(id, req.FileName)

	// Upload to Azure
	imageURL, err := s.azureClient.UploadImage(ctx, req.Image, fileName, req.ContentType)
	if err != nil {
		metrics.ProfilePictureUploads.WithLabelValues("error").Inc()
		logger.Error("Failed to upload profile picture", zap.Error(err), zap.Int("mentor_id", id))
		return "", fmt.Errorf("failed to upload image")
	}

	// Update database asynchronously
	go func() {
		if err := s.mentorRepo.UpdateImage(context.Background(), mentor.MentorID, imageURL); err != nil {
			logger.Error("Failed to update mentor image in database", zap.Error(err), zap.Int("mentor_id", id))
		} else {
			// Trigger mentor updated webhook after successful database update
			trigger.CallAsync(s.config.EventTriggers.MentorUpdatedTriggerURL, mentor.MentorID, s.httpClient)
		}
	}()

	metrics.ProfilePictureUploads.WithLabelValues("success").Inc()
	logger.Info("Profile picture uploaded", zap.Int("mentor_id", id), zap.String("url", imageURL))

	return imageURL, nil
}

func (s *ProfileService) getSponsorTags() map[string]bool {
	return map[string]bool{
		"Сообщество Онтико": true,
		"Эксперт Авито":     true,
	}
}

// SaveProfileByMentorId updates a mentor's profile using Mentor ID (UUID) for session-based auth
func (s *ProfileService) SaveProfileByMentorId(ctx context.Context, mentorID string, req *models.SaveProfileRequest) error {
	// Get mentor to get current tags (for sponsor preservation)
	mentor, err := s.mentorRepo.GetByMentorId(ctx, mentorID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return apperrors.NotFoundError("mentor")
	}

	// Get sponsor tags to preserve them
	sponsorTags := s.getSponsorTags()
	preservedSponsors := []string{}
	for _, tag := range mentor.Tags {
		if sponsorTags[tag] {
			preservedSponsors = append(preservedSponsors, tag)
		}
	}

	// Filter out sponsor tags from user input (they shouldn't be able to modify these)
	userTags := []string{}
	for _, tag := range req.Tags {
		if !sponsorTags[tag] {
			userTags = append(userTags, tag)
		}
	}

	// Merge user tags with preserved sponsor tags
	userTags = append(userTags, preservedSponsors...)

	// Get tag IDs
	tagIDs := []string{}
	for _, tagName := range userTags {
		tagID, tagErr := s.mentorRepo.GetTagIDByName(ctx, tagName)
		if tagErr == nil && tagID != "" {
			tagIDs = append(tagIDs, tagID)
		}
	}

	// Prepare updates with PostgreSQL column names
	updates := map[string]interface{}{
		"name":         req.Name,
		"job_title":    req.Job,
		"workplace":    req.Workplace,
		"experience":   req.Experience,
		"price":        req.Price,
		"details":      req.Description,
		"about":        req.About,
		"competencies": req.Competencies,
	}

	if req.CalendarURL != "" {
		updates["calendar_url"] = req.CalendarURL
	}

	// Note: Tags will be handled separately via mentor_tags table
	_ = tagIDs // TODO: Implement tag updates in repository

	// Update in database
	if err := s.mentorRepo.Update(ctx, mentorID, updates); err != nil {
		metrics.ProfileUpdates.WithLabelValues("error").Inc()
		logger.Error("Failed to update mentor profile",
			zap.Error(err),
			zap.String("mentor_id", mentorID))
		return fmt.Errorf("failed to update profile")
	}

	metrics.ProfileUpdates.WithLabelValues("success").Inc()
	logger.Info("Mentor profile updated via session",
		zap.String("mentor_id", mentorID))

	return nil
}

// UploadPictureByMentorId uploads a profile picture using Mentor ID (UUID) for session-based auth
func (s *ProfileService) UploadPictureByMentorId(ctx context.Context, mentorID string, mentorSlug string, req *models.UploadProfilePictureRequest) (string, error) {
	// Validate file type
	if typeErr := s.azureClient.ValidateImageType(req.ContentType); typeErr != nil {
		return "", typeErr
	}

	// Validate file size
	if sizeErr := s.azureClient.ValidateImageSize(req.Image); sizeErr != nil {
		return "", sizeErr
	}

	// Generate filename using slug
	fileName := fmt.Sprintf("%s/%s", mentorSlug, req.FileName)

	// Upload to Azure
	imageURL, err := s.azureClient.UploadImage(ctx, req.Image, fileName, req.ContentType)
	if err != nil {
		metrics.ProfilePictureUploads.WithLabelValues("error").Inc()
		logger.Error("Failed to upload profile picture",
			zap.Error(err),
			zap.String("mentor_id", mentorID))
		return "", fmt.Errorf("failed to upload image")
	}

	// Update database asynchronously
	go func() {
		if updateErr := s.mentorRepo.UpdateImage(context.Background(), mentorID, imageURL); updateErr != nil {
			logger.Error("Failed to update mentor image in database",
				zap.Error(updateErr),
				zap.String("mentor_id", mentorID))
		} else {
			// Trigger mentor updated webhook after successful database update
			trigger.CallAsync(s.config.EventTriggers.MentorUpdatedTriggerURL, mentorID, s.httpClient)
		}
	}()

	metrics.ProfilePictureUploads.WithLabelValues("success").Inc()
	logger.Info("Profile picture uploaded via session",
		zap.String("mentor_id", mentorID),
		zap.String("url", imageURL))

	return imageURL, nil
}
