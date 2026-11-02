package models

import "time"

const (
	AdminRequestsDefaultPageSize = 20
	AdminRequestsMaxPageSize     = 100
)

// AdminRequestStatuses lists every status an admin may set on a client request.
// Admins are not bound by the mentor workflow: any status can move to any other.
var AdminRequestStatuses = []RequestStatus{
	StatusPending,
	StatusContacted,
	StatusWorking,
	StatusDone,
	StatusDeclined,
	StatusUnavailable,
}

// IsValidAdminRequestStatus reports whether status can be set by an admin.
func IsValidAdminRequestStatus(status RequestStatus) bool {
	for _, s := range AdminRequestStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// AdminRequestsFilter holds list filters and pagination for the admin requests list.
// Text filters are case-insensitive substring matches. MentorID matches either the
// mentor UUID or the numeric legacy ID exactly.
type AdminRequestsFilter struct {
	MentorID    string
	MentorEmail string
	MentorName  string
	MenteeEmail string
	Status      RequestStatus
	Page        int
	PageSize    int
}

// Offset returns the SQL offset for the current page (pages are 1-based).
func (f *AdminRequestsFilter) Offset() int {
	if f.Page < 1 {
		return 0
	}
	return (f.Page - 1) * f.PageSize
}

// AdminRequestMentor is the mentor summary attached to a request in admin views.
// It is nil when the mentor record has been deleted.
type AdminRequestMentor struct {
	MentorID string `json:"mentorId"`
	LegacyID int    `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// AdminRequestListItem is a row in the admin requests list.
type AdminRequestListItem struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Email     string              `json:"email"`
	Telegram  string              `json:"telegram"`
	Status    RequestStatus       `json:"status"`
	CreatedAt time.Time           `json:"createdAt"`
	HasReview bool                `json:"hasReview"`
	Mentor    *AdminRequestMentor `json:"mentor"`
}

// AdminRequestReview is the mentee review left for a request.
type AdminRequestReview struct {
	ID             string    `json:"id"`
	MentorReview   string    `json:"mentorReview"`
	PlatformReview string    `json:"platformReview"`
	Improvements   string    `json:"improvements"`
	CreatedAt      time.Time `json:"createdAt"`
}

// AdminRequestDetails is the full request view for admins.
type AdminRequestDetails struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Email           string              `json:"email"`
	Telegram        string              `json:"telegram"`
	Details         string              `json:"details"`
	Level           string              `json:"level"`
	Status          RequestStatus       `json:"status"`
	CreatedAt       time.Time           `json:"createdAt"`
	ModifiedAt      time.Time           `json:"modifiedAt"`
	StatusChangedAt *time.Time          `json:"statusChangedAt"`
	ScheduledAt     *time.Time          `json:"scheduledAt"`
	DeclineReason   string              `json:"declineReason"`
	DeclineComment  string              `json:"declineComment"`
	Mentor          *AdminRequestMentor `json:"mentor"`
	Review          *AdminRequestReview `json:"review"`
}

type AdminRequestsListResponse struct {
	Requests   []AdminRequestListItem `json:"requests"`
	Total      int                    `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"pageSize"`
	TotalPages int                    `json:"totalPages"`
}

type AdminRequestResponse struct {
	Request *AdminRequestDetails `json:"request"`
}

type AdminRequestStatusUpdateRequest struct {
	Status RequestStatus `json:"status" binding:"required,oneof=pending contacted working done declined unavailable"`
}
