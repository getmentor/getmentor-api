package services

import (
	"context"
	"fmt"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
	"github.com/getmentor/getmentor-api/pkg/analytics"
)

// AdminRequestsService exposes client requests to admins.
// Status changes made here are silent: no workflow validation and no triggers.
type AdminRequestsService struct {
	requestRepo *repository.ClientRequestRepository
	tracker     analytics.Tracker
}

func NewAdminRequestsService(
	requestRepo *repository.ClientRequestRepository,
	tracker analytics.Tracker,
) *AdminRequestsService {

	if tracker == nil {
		tracker = analytics.NoopTracker{}
	}
	return &AdminRequestsService{requestRepo: requestRepo, tracker: tracker}
}

// NormalizeAdminRequestsFilter applies pagination defaults and bounds in place.
func NormalizeAdminRequestsFilter(filter *models.AdminRequestsFilter) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = models.AdminRequestsDefaultPageSize
	}
	if filter.PageSize > models.AdminRequestsMaxPageSize {
		filter.PageSize = models.AdminRequestsMaxPageSize
	}
}

func (s *AdminRequestsService) ListRequests(
	ctx context.Context,
	session *models.AdminSession,
	filter *models.AdminRequestsFilter,
) (*models.AdminRequestsListResponse, error) {

	if session.Role != models.ModeratorRoleAdmin {
		return nil, ErrAdminForbiddenAction
	}
	if filter.Status != "" && !models.IsValidAdminRequestStatus(filter.Status) {
		return nil, fmt.Errorf("unsupported status: %s", filter.Status)
	}

	NormalizeAdminRequestsFilter(filter)
	items, total, err := s.requestRepo.ListForAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}

	totalPages := (total + filter.PageSize - 1) / filter.PageSize
	return &models.AdminRequestsListResponse{
		Requests:   items,
		Total:      total,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *AdminRequestsService) GetRequest(
	ctx context.Context,
	session *models.AdminSession,
	requestID string,
) (*models.AdminRequestDetails, error) {

	if session.Role != models.ModeratorRoleAdmin {
		return nil, ErrAdminForbiddenAction
	}
	return s.requestRepo.GetForAdminByID(ctx, requestID)
}

// UpdateRequestStatus sets any status on a request. No transition rules apply and
// no external triggers (emails, notifications) are fired.
func (s *AdminRequestsService) UpdateRequestStatus(
	ctx context.Context,
	session *models.AdminSession,
	requestID string,
	status models.RequestStatus,
) (*models.AdminRequestDetails, error) {

	track := func(fromStatus models.RequestStatus, outcome string) {
		s.tracker.Track(ctx, analytics.EventAdminRequestStatusUpdated, analytics.ModeratorDistinctID(session.ModeratorID), map[string]interface{}{
			"moderator_id":      session.ModeratorID,
			"moderator_role":    string(session.Role),
			"target_request_id": requestID,
			"from_status":       string(fromStatus),
			"requested_status":  string(status),
			"outcome":           outcome,
		})
	}

	if session.Role != models.ModeratorRoleAdmin {
		track("", "forbidden")
		return nil, ErrAdminForbiddenAction
	}
	if !models.IsValidAdminRequestStatus(status) {
		track("", "unsupported_status")
		return nil, fmt.Errorf("unsupported status: %s", status)
	}

	request, err := s.requestRepo.GetForAdminByID(ctx, requestID)
	if err != nil {
		track("", "request_not_found")
		return nil, err
	}
	if request.Status == status {
		track(request.Status, "unchanged")
		return request, nil
	}

	if err := s.requestRepo.UpdateStatus(ctx, requestID, status); err != nil {
		track(request.Status, "update_failed")
		return nil, err
	}
	track(request.Status, "success")

	return s.requestRepo.GetForAdminByID(ctx, requestID)
}
