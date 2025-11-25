package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

// WebhookServiceInterface defines the interface for webhook business logic operations.
type WebhookServiceInterface interface {
	HandleAirtableWebhook(ctx context.Context, payload *models.WebhookPayload) error
	RevalidateNextJSManual(ctx context.Context, slug, secret string) error
}

type WebhookService struct {
	mentorRepo  repository.MentorRepositoryInterface
	config      *config.Config
	HTTPClient  *http.Client // Exported field
}

func NewWebhookService(mentorRepo repository.MentorRepositoryInterface, cfg *config.Config) WebhookServiceInterface {
	return &WebhookService{
		mentorRepo: mentorRepo,
		config:     cfg,
		HTTPClient: &http.Client{Timeout: 5 * time.Second}, // Add a default HTTP client with a timeout
	}
}

func (s *WebhookService) HandleAirtableWebhook(ctx context.Context, payload *models.WebhookPayload) error {
	logger.Info("Received Airtable webhook", zap.String("record_id", payload.RecordID))

	// Invalidate mentor cache
	s.mentorRepo.InvalidateCache()

	// Get mentor to find slug
	mentor, err := s.mentorRepo.GetByRecordID(ctx, payload.RecordID, models.FilterOptions{})
	if err != nil {
		logger.Warn("Failed to get mentor for webhook", zap.Error(err))
		return nil // Don't fail the webhook
	}

	// Trigger Next.js ISR revalidation
	if err := s.revalidateNextJS(ctx, mentor.Slug); err != nil {
		logger.Warn("Failed to trigger Next.js revalidation", zap.Error(err))
		// Don't fail the webhook
	}

	return nil
}

func (s *WebhookService) revalidateNextJS(ctx context.Context, slug string) error {
	url := fmt.Sprintf("%s/api/revalidate?secret=%s&slug=%s",
		s.config.NextJS.BaseURL,
		s.config.NextJS.RevalidateSecret,
		slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create Next.js revalidate request: %w", err)
	}

	resp, err := s.HTTPClient.Do(req)
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

func (s *WebhookService) RevalidateNextJSManual(ctx context.Context, slug, secret string) error {
	if secret != s.config.Auth.RevalidateSecret {
		return fmt.Errorf("invalid secret")
	}

	return s.revalidateNextJS(ctx, slug)
}
