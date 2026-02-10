package services

import (
	"context"
	"fmt"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	apperrors "github.com/getmentor/getmentor-api/pkg/errors"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"github.com/getmentor/getmentor-api/pkg/yandex"
	"go.uber.org/zap"
)

type ProfileService struct {
	mentorRepo   *repository.MentorRepository
	yandexClient *yandex.StorageClient
	config       *config.Config
	httpClient   httpclient.Client
}

func NewProfileService(
	mentorRepo *repository.MentorRepository,
	yandexClient *yandex.StorageClient,
	cfg *config.Config,
	httpClient httpclient.Client,
) *ProfileService {

	return &ProfileService{
		mentorRepo:   mentorRepo,
		yandexClient: yandexClient,
		config:       cfg,
		httpClient:   httpClient,
	}
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

	// Update in database
	if err := s.mentorRepo.Update(ctx, mentorID, updates); err != nil {
		metrics.ProfileUpdates.WithLabelValues("error").Inc()
		logger.Error("Failed to update mentor profile",
			zap.Error(err),
			zap.String("mentor_id", mentorID))
		return fmt.Errorf("failed to update profile")
	}

	// Update tags in mentor_tags table
	if err := s.mentorRepo.UpdateMentorTags(ctx, mentorID, tagIDs); err != nil {
		logger.Error("Failed to update mentor tags",
			zap.Error(err),
			zap.String("mentor_id", mentorID))
		// Don't fail the whole update if tags fail - log and continue
	}

	metrics.ProfileUpdates.WithLabelValues("success").Inc()
	logger.Info("Mentor profile updated via session",
		zap.String("mentor_id", mentorID))

	return nil
}

// UploadPictureByMentorId uploads a profile picture using Mentor ID (UUID) for session-based auth
func (s *ProfileService) UploadPictureByMentorId(ctx context.Context, mentorID string, mentorSlug string, req *models.UploadProfilePictureRequest) (string, error) {
	// Validate file type
	if typeErr := s.yandexClient.ValidateImageType(req.ContentType); typeErr != nil {
		return "", typeErr
	}

	// Validate file size
	if sizeErr := s.yandexClient.ValidateImageSize(req.Image); sizeErr != nil {
		return "", sizeErr
	}

	// Upload to Yandex Object Storage in 3 sizes: full, large, small
	// NOTE: Currently uploading same image 3 times (tech debt - future: generate thumbnails)
	// This replicates the behavior from Azure Functions storageClient.ts
	sizes := []string{"full", "large", "small"}
	var fullImageURL string

	for _, size := range sizes {
		// Generate key: {slug}/{size} (e.g., "john-doe/full")
		key := fmt.Sprintf("%s/%s", mentorSlug, size)

		// Upload to Yandex
		imageURL, err := s.yandexClient.UploadImage(ctx, req.Image, key, req.ContentType)
		if err != nil {
			metrics.ProfilePictureUploads.WithLabelValues("error").Inc()
			logger.Error("Failed to upload profile picture to Yandex",
				zap.Error(err),
				zap.String("mentor_id", mentorID),
				zap.String("size", size))
			return "", fmt.Errorf("failed to upload image")
		}

		// Store the 'full' URL for database
		if size == "full" {
			fullImageURL = imageURL
		}

		logger.Info("Uploaded profile picture size",
			zap.String("mentor_id", mentorID),
			zap.String("size", size),
			zap.String("url", imageURL))
	}

	// TODO: Re-enable webhook trigger when thumbnail generation is implemented
	// Update database asynchronously
	go func() {
		// This webhook will trigger Azure Function to generate thumbnails
		// trigger.CallAsync(s.config.EventTriggers.MentorUpdatedTriggerURL, mentorID, s.httpClient)
		_ = s.config.EventTriggers.MentorUpdatedTriggerURL // Keep for future use
		_ = s.httpClient                                   // Keep for future use
		_ = trigger.CallAsync                              // Keep for future use
	}()

	metrics.ProfilePictureUploads.WithLabelValues("success").Inc()
	logger.Info("Profile picture uploaded via session",
		zap.String("mentor_id", mentorID),
		zap.String("url", fullImageURL))

	return fullImageURL, nil
}
