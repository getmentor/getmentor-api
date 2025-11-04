package services

import (
	"fmt"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/azure"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

type ProfileService struct {
	mentorRepo  *repository.MentorRepository
	azureClient *azure.StorageClient
	config      *config.Config
}

func NewProfileService(
	mentorRepo *repository.MentorRepository,
	azureClient *azure.StorageClient,
	cfg *config.Config,
) *ProfileService {
	return &ProfileService{
		mentorRepo:  mentorRepo,
		azureClient: azureClient,
		config:      cfg,
	}
}

func (s *ProfileService) SaveProfile(id int, token string, req *models.SaveProfileRequest) error {
	// Get mentor and verify auth token
	mentor, err := s.mentorRepo.GetByID(id, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return fmt.Errorf("mentor not found")
	}

	if mentor.AuthToken != token {
		return fmt.Errorf("access denied")
	}

	// Get sponsor tags to preserve them
	sponsorTags := s.getSponsorTags()
	preservedSponsors := []string{}
	for _, tag := range mentor.Tags {
		if sponsorTags[tag] {
			preservedSponsors = append(preservedSponsors, tag)
		}
	}

	// Merge user tags with preserved sponsor tags
	allTags := req.Tags
	allTags = append(allTags, preservedSponsors...)

	// Get tag IDs
	tagIDs := []string{}
	for _, tagName := range allTags {
		tagID, err := s.mentorRepo.GetTagIDByName(tagName)
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
		"Tags":         tagIDs,
		"Details":      req.Description,
		"About":        req.About,
		"Competencies": req.Competencies,
	}

	if req.CalendarURL != "" {
		updates["Calendly Url"] = req.CalendarURL
	}

	// Update in Airtable
	if err := s.mentorRepo.Update(mentor.AirtableID, updates); err != nil {
		metrics.ProfileUpdates.WithLabelValues("error").Inc()
		logger.Error("Failed to update mentor profile", zap.Error(err), zap.Int("mentor_id", id))
		return fmt.Errorf("failed to update profile")
	}

	metrics.ProfileUpdates.WithLabelValues("success").Inc()
	logger.Info("Mentor profile updated", zap.Int("mentor_id", id))

	return nil
}

func (s *ProfileService) UploadProfilePicture(id int, token string, req *models.UploadProfilePictureRequest) (string, error) {
	// Get mentor and verify auth token
	mentor, err := s.mentorRepo.GetByID(id, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return "", fmt.Errorf("mentor not found")
	}

	if mentor.AuthToken != token {
		return "", fmt.Errorf("access denied")
	}

	// Validate file type
	if err := s.azureClient.ValidateImageType(req.ContentType); err != nil {
		return "", err
	}

	// Validate file size
	if err := s.azureClient.ValidateImageSize(req.Image); err != nil {
		return "", err
	}

	// Generate filename
	fileName := s.azureClient.GenerateFileName(id, req.FileName)

	// Upload to Azure
	imageURL, err := s.azureClient.UploadImage(req.Image, fileName, req.ContentType)
	if err != nil {
		metrics.ProfilePictureUploads.WithLabelValues("error").Inc()
		logger.Error("Failed to upload profile picture", zap.Error(err), zap.Int("mentor_id", id))
		return "", fmt.Errorf("failed to upload image")
	}

	// Update Airtable asynchronously
	go func() {
		if err := s.mentorRepo.UpdateImage(mentor.AirtableID, imageURL); err != nil {
			logger.Error("Failed to update mentor image in Airtable", zap.Error(err), zap.Int("mentor_id", id))
		}
	}()

	metrics.ProfilePictureUploads.WithLabelValues("success").Inc()
	logger.Info("Profile picture uploaded", zap.Int("mentor_id", id), zap.String("url", imageURL))

	return imageURL, nil
}

func (s *ProfileService) getSponsorTags() map[string]bool {
	return map[string]bool{
		"Ontico":     true,
		"ТензорСофт": true,
	}
}
