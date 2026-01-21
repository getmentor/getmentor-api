package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/getmentor/getmentor-api/pkg/trigger"
	"go.uber.org/zap"
)

var (
	ErrRequestNotFound         = errors.New("request not found")
	ErrAccessDenied            = errors.New("access denied")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrCannotDeclineRequest    = errors.New("cannot decline request")
	ErrInvalidRequestGroup     = errors.New("invalid request group")
)

// MentorRequestsService handles mentor request operations
type MentorRequestsService struct {
	requestRepo *repository.ClientRequestRepository
	config      *config.Config
	httpClient  httpclient.Client
}

// NewMentorRequestsService creates a new MentorRequestsService
func NewMentorRequestsService(requestRepo *repository.ClientRequestRepository, cfg *config.Config, httpClient httpclient.Client) *MentorRequestsService {
	return &MentorRequestsService{
		requestRepo: requestRepo,
		config:      cfg,
		httpClient:  httpClient,
	}
}

// GetRequests retrieves requests for a mentor filtered by group
func (s *MentorRequestsService) GetRequests(ctx context.Context, mentorAirtableID string, group string) (*models.ClientRequestsResponse, error) {
	start := time.Now()

	// Validate group
	requestGroup := models.RequestGroup(group)
	statuses := requestGroup.GetStatuses()
	if statuses == nil {
		return nil, ErrInvalidRequestGroup
	}

	// Fetch requests from repository
	requests, err := s.requestRepo.GetByMentor(ctx, mentorAirtableID, statuses)
	if err != nil {
		logger.Error("Failed to fetch requests",
			zap.String("mentor_id", mentorAirtableID),
			zap.String("group", group),
			zap.Error(err))
		return nil, fmt.Errorf("failed to fetch requests: %w", err)
	}

	// Convert to response format
	responseRequests := make([]models.MentorClientRequest, 0, len(requests))
	for _, req := range requests {
		responseRequests = append(responseRequests, *req)
	}

	duration := metrics.MeasureDuration(start)
	metrics.MentorRequestsListDuration.Observe(duration)
	metrics.MentorRequestsListTotal.WithLabelValues(group).Inc()

	logger.Info("Fetched mentor requests",
		zap.String("mentor_id", mentorAirtableID),
		zap.String("group", group),
		zap.Int("count", len(responseRequests)),
		zap.Duration("duration", time.Since(start)))

	return &models.ClientRequestsResponse{
		Requests: responseRequests,
		Total:    len(responseRequests),
	}, nil
}

// GetRequestByID retrieves a single request and verifies ownership
func (s *MentorRequestsService) GetRequestByID(ctx context.Context, mentorAirtableID string, requestID string) (*models.MentorClientRequest, error) {
	// Fetch request
	request, err := s.requestRepo.GetByID(ctx, requestID)
	if err != nil {
		logger.Warn("Request not found",
			zap.String("request_id", requestID),
			zap.Error(err))
		return nil, ErrRequestNotFound
	}

	// Verify ownership
	if request.MentorID != mentorAirtableID {
		logger.Warn("Access denied to request",
			zap.String("request_id", requestID),
			zap.String("request_mentor", request.MentorID),
			zap.String("requesting_mentor", mentorAirtableID))
		return nil, ErrAccessDenied
	}

	return request, nil
}

// UpdateStatus updates the status of a request with workflow validation
func (s *MentorRequestsService) UpdateStatus(ctx context.Context, mentorAirtableID string, requestID string, newStatus models.RequestStatus) (*models.MentorClientRequest, error) {
	// Fetch and verify ownership
	request, err := s.GetRequestByID(ctx, mentorAirtableID, requestID)
	if err != nil {
		return nil, err
	}

	// Validate status transition
	if !request.Status.CanTransitionTo(newStatus) {
		logger.Warn("Invalid status transition",
			zap.String("request_id", requestID),
			zap.String("from_status", string(request.Status)),
			zap.String("to_status", string(newStatus)))
		return nil, fmt.Errorf("%w: cannot transition from '%s' to '%s'", ErrInvalidStatusTransition, request.Status, newStatus)
	}

	oldStatus := request.Status

	// Update in repository
	if err := s.requestRepo.UpdateStatus(ctx, requestID, newStatus); err != nil {
		logger.Error("Failed to update request status",
			zap.String("request_id", requestID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	// Trigger email sending via webhook
	if newStatus == models.StatusDone && s.config.EventTriggers.RequestProcessFinishedTriggerURL != "" {
		trigger.CallAsync(s.config.EventTriggers.RequestProcessFinishedTriggerURL, requestID, s.httpClient)
	}

	// Record metrics
	metrics.MentorRequestsStatusUpdates.WithLabelValues(string(oldStatus), string(newStatus)).Inc()

	logger.Info("Request status updated",
		zap.String("request_id", requestID),
		zap.String("from_status", string(oldStatus)),
		zap.String("to_status", string(newStatus)))

	// Fetch updated request
	return s.requestRepo.GetByID(ctx, requestID)
}

// DeclineRequest declines a request with reason
func (s *MentorRequestsService) DeclineRequest(ctx context.Context, mentorAirtableID string, requestID string, payload *models.DeclineRequestPayload) (*models.MentorClientRequest, error) {
	// Fetch and verify ownership
	request, err := s.GetRequestByID(ctx, mentorAirtableID, requestID)
	if err != nil {
		return nil, err
	}

	// Check if request can be declined
	if request.Status == models.StatusDone {
		logger.Warn("Cannot decline completed request",
			zap.String("request_id", requestID),
			zap.String("status", string(request.Status)))
		return nil, fmt.Errorf("%w: request with status '%s' cannot be declined", ErrCannotDeclineRequest, request.Status)
	}

	if request.Status.IsTerminalStatus() {
		logger.Warn("Cannot decline request with terminal status",
			zap.String("request_id", requestID),
			zap.String("status", string(request.Status)))
		return nil, fmt.Errorf("%w: request with status '%s' cannot be declined", ErrCannotDeclineRequest, request.Status)
	}

	// Update in repository
	if err := s.requestRepo.UpdateDecline(ctx, requestID, payload.Reason, payload.Comment); err != nil {
		logger.Error("Failed to decline request",
			zap.String("request_id", requestID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to decline request: %w", err)
	}

	// Trigger email sending via webhook
	if s.config.EventTriggers.RequestProcessFinishedTriggerURL != "" {
		trigger.CallAsync(s.config.EventTriggers.RequestProcessFinishedTriggerURL, requestID, s.httpClient)
	}

	// Record metrics
	metrics.MentorRequestsDeclines.WithLabelValues(string(payload.Reason)).Inc()

	logger.Info("Request declined",
		zap.String("request_id", requestID),
		zap.String("reason", string(payload.Reason)))

	// Fetch updated request
	return s.requestRepo.GetByID(ctx, requestID)
}
