package services

import (
	"context"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/recaptcha"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

// ContactService handles contact form submissions and mentor contact requests
type ContactService struct {
	clientRequestRepo *repository.ClientRequestRepository
	mentorRepo        *repository.MentorRepository
	config            *config.Config
	httpClient        httpclient.Client
	recaptchaVerifier *recaptcha.Verifier
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
		recaptchaVerifier: recaptcha.NewVerifier(cfg.ReCAPTCHA.SecretKey, httpClient),
	}
}

func (s *ContactService) SubmitContactForm(ctx context.Context, req *models.ContactMentorRequest) (*models.ContactMentorResponse, error) {
	// Verify ReCAPTCHA
	if err := s.recaptchaVerifier.Verify(req.RecaptchaToken); err != nil {
		metrics.ContactFormSubmissions.WithLabelValues("captcha_failed").Inc()
		logger.Warn("ReCAPTCHA verification failed", zap.Error(err))
		return &models.ContactMentorResponse{
			Success: false,
			Error:   "Captcha verification failed",
		}, nil
	}

	// Create client request in PostgreSQL (skip in development)
	if !s.config.IsDevelopment() {
		clientReq := &models.ClientRequest{
			Email:       req.Email,
			Name:        req.Name,
			Level:       req.Experience,
			MentorID:    req.MentorID,
			Description: req.Intro,
			Telegram:    req.TelegramUsername,
		}

		requestID, err := s.clientRequestRepo.Create(ctx, clientReq)
		if err != nil {
			metrics.ContactFormSubmissions.WithLabelValues("error").Inc()
			logger.Error("Failed to create client request", zap.Error(err))
			return &models.ContactMentorResponse{
				Success: false,
				Error:   "Failed to save contact request",
			}, nil
		}

		// Trigger contact created webhook (non-blocking)
		trigger.CallAsync(s.config.EventTriggers.MentorRequestCreatedTriggerURL, requestID, s.httpClient)
	} else {
		metrics.ContactFormSubmissions.WithLabelValues("success_dev").Inc()
	}

	// Get mentor to retrieve calendar URL
	mentor, err := s.mentorRepo.GetByMentorId(ctx, req.MentorID, models.FilterOptions{ShowHidden: true})
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
