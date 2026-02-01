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
