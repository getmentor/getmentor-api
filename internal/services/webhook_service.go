package services

import (
	"context"

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

	// Invalidate mentor cache to ensure fresh data
	s.mentorRepo.InvalidateCache()

	logger.Info("Mentor cache invalidated", zap.String("record_id", payload.RecordID))

	return nil
}
