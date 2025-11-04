package services

import (
	"fmt"
	"net/http"

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

func (s *WebhookService) HandleAirtableWebhook(payload *models.WebhookPayload) error {
	logger.Info("Received Airtable webhook", zap.String("record_id", payload.RecordID))

	// Invalidate mentor cache
	s.mentorRepo.InvalidateCache()

	// Get mentor to find slug
	mentor, err := s.mentorRepo.GetByRecordID(payload.RecordID, models.FilterOptions{})
	if err != nil {
		logger.Warn("Failed to get mentor for webhook", zap.Error(err))
		return nil // Don't fail the webhook
	}

	// Trigger Next.js ISR revalidation
	if err := s.revalidateNextJS(mentor.Slug); err != nil {
		logger.Warn("Failed to trigger Next.js revalidation", zap.Error(err))
		// Don't fail the webhook
	}

	return nil
}

func (s *WebhookService) revalidateNextJS(slug string) error {
	url := fmt.Sprintf("%s/api/revalidate?secret=%s&slug=%s",
		s.config.NextJS.BaseURL,
		s.config.NextJS.RevalidateSecret,
		slug)

	//nolint:gosec // URL is constructed from trusted configuration
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to call Next.js revalidate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Next.js revalidate returned status %d", resp.StatusCode)
	}

	logger.Info("Next.js revalidation triggered", zap.String("slug", slug))
	return nil
}

func (s *WebhookService) RevalidateNextJSManual(slug, secret string) error {
	if secret != s.config.Auth.RevalidateSecret {
		return fmt.Errorf("invalid secret")
	}

	return s.revalidateNextJS(slug)
}
