package services

import (
	"context"
	"crypto/subtle"
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

func (s *ProfileService) SaveProfile(ctx context.Context, id int, token string, req *models.SaveProfileRequest) error {
	// Get mentor and verify auth token
	mentor, err := s.mentorRepo.GetByID(ctx, id, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return apperrors.NotFoundError("mentor")
	}

	// SECURITY: Use timing-safe comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(mentor.AuthToken), []byte(token)) != 1 {
		logger.Warn("Access denied - invalid token", zap.Int("mentor_id", id))
		return apperrors.ErrAccessDenied
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
		tagID, err := s.mentorRepo.GetTagIDByName(ctx, tagName)
		if err == nil && tagID != "" {
			tagIDs = append(tagIDs, tagID)
		}
	}

	// Prepare updates
	updates := map[string]interface{}{
		"Name":         req.Name,
		"JobTitle":     req.Job,
		"Workplace":    req.Workplace,
		"Experience":   req.Experience,
		"Price":        req.Price,
		"Tags Links":   tagIDs,
		"Details":      req.Description,
		"About":        req.About,
		"Competencies": req.Competencies,
	}

	if req.CalendarURL != "" {
		updates["Calendly Url"] = req.CalendarURL
	}

	// Update in Airtable
	if err := s.mentorRepo.Update(ctx, mentor.AirtableID, updates); err != nil {
		metrics.ProfileUpdates.WithLabelValues("error").Inc()
		logger.Error("Failed to update mentor profile", zap.Error(err), zap.Int("mentor_id", id))
		return fmt.Errorf("failed to update profile")
	}

	// Update mentor object for cache
	mentor.Name = req.Name
	mentor.Job = req.Job
	mentor.Workplace = req.Workplace
	mentor.Experience = req.Experience
	mentor.Price = req.Price
	mentor.Tags = userTags
	mentor.Description = req.Description
	mentor.About = req.About
	mentor.Competencies = req.Competencies
	if req.CalendarURL != "" {
		mentor.CalendarURL = req.CalendarURL
		mentor.CalendarType = models.GetCalendarType(req.CalendarURL)
	}
	mentor.Sponsors = models.GetMentorSponsor(userTags)

	// Update cache locally (O(1) instead of fetching from Airtable)
	if err := s.mentorRepo.UpdateMentorInCache(mentor); err != nil {
		logger.Error("Failed to update mentor cache after profile save", zap.Error(err), zap.Int("mentor_id", id))
		// We don't return error here because the Airtable update was successful
	}

	metrics.ProfileUpdates.WithLabelValues("success").Inc()
	logger.Info("Mentor profile updated", zap.Int("mentor_id", id))

	return nil
}

func (s *ProfileService) UploadProfilePicture(ctx context.Context, id int, token string, req *models.UploadProfilePictureRequest) (string, error) {
	// Get mentor and verify auth token
	mentor, err := s.mentorRepo.GetByID(ctx, id, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return "", apperrors.NotFoundError("mentor")
	}

	// SECURITY: Use timing-safe comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(mentor.AuthToken), []byte(token)) != 1 {
		logger.Warn("Access denied - invalid token", zap.Int("mentor_id", id))
		return "", apperrors.ErrAccessDenied
	}

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

	// Update Airtable asynchronously
	go func() {
		if err := s.mentorRepo.UpdateImage(context.Background(), mentor.AirtableID, imageURL); err != nil {
			logger.Error("Failed to update mentor image in Airtable", zap.Error(err), zap.Int("mentor_id", id))
		} else {
			// Trigger mentor updated webhook after successful Airtable update
			trigger.CallAsync(s.config.EventTriggers.MentorUpdatedTriggerURL, mentor.AirtableID, s.httpClient)
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

// SaveProfileByAirtableID updates a mentor's profile using Airtable ID (for session-based auth)
func (s *ProfileService) SaveProfileByAirtableID(ctx context.Context, airtableID string, req *models.SaveProfileRequest) error {
	// Get mentor to get current tags (for sponsor preservation)
	mentor, err := s.mentorRepo.GetByRecordID(ctx, airtableID, models.FilterOptions{ShowHidden: true})
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

	// Prepare updates
	updates := map[string]interface{}{
		"Name":         req.Name,
		"JobTitle":     req.Job,
		"Workplace":    req.Workplace,
		"Experience":   req.Experience,
		"Price":        req.Price,
		"Tags Links":   tagIDs,
		"Details":      req.Description,
		"About":        req.About,
		"Competencies": req.Competencies,
	}

	if req.CalendarURL != "" {
		updates["Calendly Url"] = req.CalendarURL
	}

	// Update in Airtable
	if err := s.mentorRepo.Update(ctx, airtableID, updates); err != nil {
		metrics.ProfileUpdates.WithLabelValues("error").Inc()
		logger.Error("Failed to update mentor profile",
			zap.Error(err),
			zap.String("airtable_id", airtableID))
		return fmt.Errorf("failed to update profile")
	}

	metrics.ProfileUpdates.WithLabelValues("success").Inc()
	logger.Info("Mentor profile updated via session",
		zap.String("airtable_id", airtableID))

	return nil
}

// UploadPictureByAirtableID uploads a profile picture using Airtable ID (for session-based auth)
func (s *ProfileService) UploadPictureByAirtableID(ctx context.Context, airtableID string, mentorSlug string, req *models.UploadProfilePictureRequest) (string, error) {
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
			zap.String("airtable_id", airtableID))
		return "", fmt.Errorf("failed to upload image")
	}

	// Update Airtable asynchronously
	go func() {
		if updateErr := s.mentorRepo.UpdateImage(context.Background(), airtableID, imageURL); updateErr != nil {
			logger.Error("Failed to update mentor image in Airtable",
				zap.Error(updateErr),
				zap.String("airtable_id", airtableID))
		} else {
			// Trigger mentor updated webhook after successful Airtable update
			trigger.CallAsync(s.config.EventTriggers.MentorUpdatedTriggerURL, airtableID, s.httpClient)
		}
	}()

	metrics.ProfilePictureUploads.WithLabelValues("success").Inc()
	logger.Info("Profile picture uploaded via session",
		zap.String("airtable_id", airtableID),
		zap.String("url", imageURL))

	return imageURL, nil
}
