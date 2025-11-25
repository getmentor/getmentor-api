package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"go.uber.org/zap"
)

// ContactService handles contact form submissions and mentor contact requests
type ContactService struct {
	clientRequestRepo *repository.ClientRequestRepository
	mentorRepo        *repository.MentorRepository
	config            *config.Config
	httpClient        httpclient.Client
}

// NewContactService creates a new contact service instance
func NewContactService(
	clientRequestRepo *repository.ClientRequestRepository,
	mentorRepo *repository.MentorRepository,
	cfg *config.Config,
	httpClient httpclient.Client,
) *ContactService {
	return &ContactService{
		clientRequestRepo: clientRequestRepo,
		mentorRepo:        mentorRepo,
		config:            cfg,
		httpClient:        httpClient,
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
	// Prepare form data with secret in POST body (not URL)
	data := url.Values{}
	data.Set("secret", s.config.ReCAPTCHA.SecretKey)
	data.Set("response", token)

	// Send POST request with form body
	resp, err := s.httpClient.Post(
		"https://www.google.com/recaptcha/api/siteverify",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
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
