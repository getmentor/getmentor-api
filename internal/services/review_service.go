package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/analytics"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/recaptcha"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

var (
	ErrReviewRequestNotFound = errors.New("request not found")
	ErrReviewRequestNotDone  = errors.New("request is not in done status")
	ErrReviewAlreadyExists   = errors.New("review already exists for this request")
	ErrReviewCaptchaFailed   = errors.New("captcha verification failed")
)

// ReviewService handles review submissions
type ReviewService struct {
	reviewRepo        *repository.ReviewRepository
	config            *config.Config
	httpClient        httpclient.Client
	recaptchaVerifier *recaptcha.Verifier
	tracker           analytics.Tracker
}

// NewReviewService creates a new review service instance
func NewReviewService(
	reviewRepo *repository.ReviewRepository,
	cfg *config.Config,
	httpClient httpclient.Client,
	tracker analytics.Tracker,
) *ReviewService {
	if tracker == nil {
		tracker = analytics.NoopTracker{}
	}

	return &ReviewService{
		reviewRepo:        reviewRepo,
		config:            cfg,
		httpClient:        httpClient,
		recaptchaVerifier: recaptcha.NewVerifier(cfg.ReCAPTCHA.SecretKey, httpClient),
		tracker:           tracker,
	}
}

// CheckReview checks if a review can be submitted for a given request ID
func (s *ReviewService) CheckReview(ctx context.Context, requestID string) (*models.ReviewCheckResponse, error) {
	result, err := s.reviewRepo.CheckCanSubmitReview(ctx, requestID)
	if err != nil {
		metrics.ReviewChecks.WithLabelValues("error").Inc()
		s.tracker.Track(ctx, analytics.EventReviewEligibilityChecked, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id": requestID,
			"outcome":    "error",
			"can_submit": false,
		})
		logger.Error("Failed to check review eligibility",
			zap.String("request_id", requestID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to check review: %w", err)
	}

	if result.MentorName == "" && !result.CanSubmit {
		metrics.ReviewChecks.WithLabelValues("not_found").Inc()
		s.tracker.Track(ctx, analytics.EventReviewEligibilityChecked, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id": requestID,
			"outcome":    "not_found",
			"can_submit": false,
		})
		logger.Info("Review check: request not found",
			zap.String("request_id", requestID))
		return &models.ReviewCheckResponse{
			CanSubmit: false,
			Error:     "Заявка не найдена",
		}, ErrReviewRequestNotFound
	}

	if !result.CanSubmit {
		metrics.ReviewChecks.WithLabelValues("ineligible").Inc()
		s.tracker.Track(ctx, analytics.EventReviewEligibilityChecked, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id": requestID,
			"outcome":    "ineligible",
			"can_submit": false,
		})
		logger.Info("Review check: not eligible",
			zap.String("request_id", requestID),
			zap.String("mentor_name", result.MentorName))
		return &models.ReviewCheckResponse{
			CanSubmit:  false,
			MentorName: result.MentorName,
			Error:      "Отзыв уже оставлен или заявка ещё не завершена",
		}, nil
	}

	metrics.ReviewChecks.WithLabelValues("eligible").Inc()
	s.tracker.Track(ctx, analytics.EventReviewEligibilityChecked, analytics.RequestDistinctID(requestID), map[string]interface{}{
		"request_id": requestID,
		"outcome":    "eligible",
		"can_submit": true,
	})
	logger.Info("Review check: eligible",
		zap.String("request_id", requestID),
		zap.String("mentor_name", result.MentorName))

	return &models.ReviewCheckResponse{
		CanSubmit:  true,
		MentorName: result.MentorName,
	}, nil
}

// SubmitReview creates a new review after verifying captcha and eligibility
func (s *ReviewService) SubmitReview(ctx context.Context, requestID string, req *models.SubmitReviewRequest) (*models.SubmitReviewResponse, error) {
	start := time.Now()
	baseProperties := map[string]interface{}{
		"request_id":           requestID,
		"has_platform_review":  strings.TrimSpace(req.PlatformReview) != "",
		"has_improvements":     strings.TrimSpace(req.Improvements) != "",
		"has_mentor_review":    strings.TrimSpace(req.MentorReview) != "",
		"review_payload_size":  len(req.MentorReview) + len(req.PlatformReview) + len(req.Improvements),
		"captcha_token_length": len(req.RecaptchaToken),
	}

	// Verify ReCAPTCHA
	if err := s.recaptchaVerifier.Verify(req.RecaptchaToken); err != nil {
		metrics.ReviewSubmissions.WithLabelValues("captcha_failed").Inc()
		s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id":          requestID,
			"has_platform_review": strings.TrimSpace(req.PlatformReview) != "",
			"has_improvements":    strings.TrimSpace(req.Improvements) != "",
			"has_mentor_review":   strings.TrimSpace(req.MentorReview) != "",
			"outcome":             "captcha_failed",
		})
		logger.Warn("ReCAPTCHA verification failed for review",
			zap.String("request_id", requestID),
			zap.Error(err))
		return &models.SubmitReviewResponse{
			Success: false,
			Error:   "Captcha verification failed",
		}, ErrReviewCaptchaFailed
	}

	// Check eligibility
	checkResult, err := s.reviewRepo.CheckCanSubmitReview(ctx, requestID)
	if err != nil {
		metrics.ReviewSubmissions.WithLabelValues("error").Inc()
		s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id":          requestID,
			"has_platform_review": strings.TrimSpace(req.PlatformReview) != "",
			"has_improvements":    strings.TrimSpace(req.Improvements) != "",
			"has_mentor_review":   strings.TrimSpace(req.MentorReview) != "",
			"outcome":             "validation_error",
		})
		logger.Error("Failed to check review eligibility",
			zap.String("request_id", requestID),
			zap.Error(err))
		return &models.SubmitReviewResponse{
			Success: false,
			Error:   "Failed to validate request",
		}, fmt.Errorf("failed to check review eligibility: %w", err)
	}

	if checkResult.MentorName == "" && !checkResult.CanSubmit {
		metrics.ReviewSubmissions.WithLabelValues("not_found").Inc()
		s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id":          requestID,
			"has_platform_review": strings.TrimSpace(req.PlatformReview) != "",
			"has_improvements":    strings.TrimSpace(req.Improvements) != "",
			"has_mentor_review":   strings.TrimSpace(req.MentorReview) != "",
			"outcome":             "not_found",
		})
		return &models.SubmitReviewResponse{
			Success: false,
			Error:   "Заявка не найдена",
		}, ErrReviewRequestNotFound
	}

	if !checkResult.CanSubmit {
		metrics.ReviewSubmissions.WithLabelValues("already_exists").Inc()
		s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id":          requestID,
			"has_platform_review": strings.TrimSpace(req.PlatformReview) != "",
			"has_improvements":    strings.TrimSpace(req.Improvements) != "",
			"has_mentor_review":   strings.TrimSpace(req.MentorReview) != "",
			"outcome":             "already_exists",
		})
		return &models.SubmitReviewResponse{
			Success: false,
			Error:   "Отзыв уже оставлен или заявка ещё не завершена",
		}, ErrReviewAlreadyExists
	}

	// Create review
	reviewID, err := s.reviewRepo.CreateReview(ctx, requestID, req.MentorReview, req.PlatformReview, req.Improvements)
	if err != nil {
		metrics.ReviewSubmissions.WithLabelValues("db_error").Inc()
		s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.RequestDistinctID(requestID), map[string]interface{}{
			"request_id":          requestID,
			"has_platform_review": strings.TrimSpace(req.PlatformReview) != "",
			"has_improvements":    strings.TrimSpace(req.Improvements) != "",
			"has_mentor_review":   strings.TrimSpace(req.MentorReview) != "",
			"outcome":             "db_error",
		})
		logger.Error("Failed to create review",
			zap.String("request_id", requestID),
			zap.Error(err))
		return &models.SubmitReviewResponse{
			Success: false,
			Error:   "Failed to save review",
		}, fmt.Errorf("failed to create review: %w", err)
	}

	// Trigger Azure Function notification (non-blocking)
	trigger.CallAsync(s.config.EventTriggers.ReviewCreatedTriggerURL, reviewID, s.httpClient)

	duration := metrics.MeasureDuration(start)
	metrics.ReviewDuration.Observe(duration)
	metrics.ReviewSubmissions.WithLabelValues("success").Inc()
	successProperties := make(map[string]interface{}, len(baseProperties)+3)
	for key, value := range baseProperties {
		successProperties[key] = value
	}
	successProperties["review_id"] = reviewID
	successProperties["duration_seconds"] = duration
	successProperties["outcome"] = "success"
	s.tracker.Track(ctx, analytics.EventReviewSubmitted, analytics.RequestDistinctID(requestID), successProperties)
	logger.Info("Review submitted successfully",
		zap.String("request_id", requestID),
		zap.String("review_id", reviewID),
		zap.Duration("duration", time.Since(start)))

	return &models.SubmitReviewResponse{
		Success:  true,
		ReviewID: reviewID,
	}, nil
}
