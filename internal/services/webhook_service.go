package services

import (
	"context"
	"fmt"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

type WebhookService struct {
	mentorRepo *repository.MentorRepository
	config     *config.Config
}

func NewWebhookService(mentorRepo *repository.MentorRepository, cfg *config.Config) *WebhookService {
	return &WebhookService{
		mentorRepo: mentorRepo,
		config:     cfg,
	}
}

func (s *WebhookService) HandleAirtableWebhook(ctx context.Context, payload *models.WebhookPayload) error {
	logger.Info("Received Airtable webhook", zap.String("record_id", payload.RecordID))

	// Get slug from record ID to identify which mentor to update
	slug, err := s.getSlugFromRecordID(ctx, payload.RecordID)
	if err != nil {
		logger.Error("Failed to get slug for record",
			zap.String("record_id", payload.RecordID),
			zap.Error(err))
		// Fallback to full refresh if we can't identify the mentor
		logger.Warn("Falling back to full cache refresh")
		return s.mentorRepo.RefreshCache()
	}

	// Update single mentor in cache
	// Note: Current webhook payload doesn't distinguish between create/update/delete
	// We assume update/create action and update the mentor
	if err := s.mentorRepo.UpdateSingleMentorCache(slug); err != nil {
		logger.Error("Failed to update mentor cache",
			zap.String("slug", slug),
			zap.Error(err))
		return err
	}

	logger.Info("Mentor cache updated via webhook",
		zap.String("slug", slug),
		zap.String("record_id", payload.RecordID))

	return nil
}

// getSlugFromRecordID retrieves slug from Airtable record ID
func (s *WebhookService) getSlugFromRecordID(ctx context.Context, recordID string) (string, error) {
	// Try to get mentor from cache first (fast path)
	mentors, err := s.mentorRepo.GetAll(ctx, models.FilterOptions{ShowHidden: true})
	if err == nil {
		for _, mentor := range mentors {
			if mentor.AirtableID == recordID {
				return mentor.Slug, nil
			}
		}
	}

	// If not found in cache, fetch from Airtable (slow path)
	// This handles new mentors or cache misses
	mentor, err := s.mentorRepo.GetByRecordID(ctx, recordID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		return "", fmt.Errorf("failed to find mentor with record ID %s: %w", recordID, err)
	}

	return mentor.Slug, nil
}
