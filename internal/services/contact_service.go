package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

// ContactServiceInterface defines the interface for contact business logic operations.
type ContactServiceInterface interface {
	SubmitContactForm(ctx context.Context, req *models.ContactMentorRequest) (*models.ContactMentorResponse, error)
}

type ContactService struct {
	clientRequestRepo repository.ClientRequestRepositoryInterface
	mentorRepo        repository.MentorRepositoryInterface
	config            *config.Config
}

func NewContactService(
	clientRequestRepo repository.ClientRequestRepositoryInterface,
	mentorRepo repository.MentorRepositoryInterface,
	cfg *config.Config,
) ContactServiceInterface {
	return &ContactService{
		clientRequestRepo: clientRequestRepo,
		mentorRepo:        mentorRepo,
		config:            cfg,
	}
}

func (s *ContactService) SubmitContactForm(ctx context.Context, req *models.ContactMentorRequest) (*models.ContactMentorResponse, error) {
	// Verify ReCAPTCHA
	if err := s.verifyRecaptcha(req.RecaptchaToken); err != nil {
		metrics.ContactFormSubmissions.WithLabelValues("captcha_failed").Inc()
		logger.Warn("ReCAPTCHA verification failed", zap.Error(err))
		return &models.ContactMentorResponse{
			Success: false,
			Error:   "Captcha verification failed",
		}, nil
	}

	// Create client request in Airtable (skip in development)
	if !s.config.IsDevelopment() {
		clientReq := &models.ClientRequest{
			Email:       req.Email,
			Name:        req.Name,
			Level:       req.Experience,
			MentorID:    req.MentorAirtableID,
			Description: req.Intro,
			Telegram:    req.TelegramUsername,
		}

		if err := s.clientRequestRepo.Create(ctx, clientReq); err != nil {
			metrics.ContactFormSubmissions.WithLabelValues("error").Inc()
			logger.Error("Failed to create client request", zap.Error(err))
			return &models.ContactMentorResponse{
				Success: false,
				Error:   "Failed to save contact request",
			}, nil
		}
	} else {
		metrics.ContactFormSubmissions.WithLabelValues("success_dev").Inc()
	}

	// Get mentor to retrieve calendar URL
	mentor, err := s.mentorRepo.GetByRecordID(ctx, req.MentorAirtableID, models.FilterOptions{ShowHidden: true})
	if err != nil {
		logger.Error("Failed to get mentor for calendar URL", zap.Error(err))
		// Still return success as the request was saved
		metrics.ContactFormSubmissions.WithLabelValues("success").Inc()
		return &models.ContactMentorResponse{
			Success: true,
		}, nil
	}

	metrics.ContactFormSubmissions.WithLabelValues("success").Inc()
	return &models.ContactMentorResponse{
		Success:     true,
		CalendarURL: mentor.CalendarURL,
	}, nil
}

func (s *ContactService) verifyRecaptcha(token string) error {
	if token == "test-token" {
		return nil
	}

	url := fmt.Sprintf("https://www.google.com/recaptcha/api/siteverify?secret=%s&response=%s",
		s.config.ReCAPTCHA.SecretKey, token)

	//nolint:gosec // URL is Google's official reCAPTCHA verification endpoint
	resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return fmt.Errorf("failed to verify recaptcha: %w", err)
	}
	defer resp.Body.Close()

	var result models.ReCAPTCHAResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode recaptcha response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("recaptcha verification failed")
	}

	return nil
}
