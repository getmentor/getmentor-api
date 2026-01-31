package models

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mehanizm/airtable"
)

// RequestStatus represents the status of a client request
type RequestStatus string

const (
	StatusPending     RequestStatus = "pending"
	StatusContacted   RequestStatus = "contacted"
	StatusWorking     RequestStatus = "working"
	StatusDone        RequestStatus = "done"
	StatusDeclined    RequestStatus = "declined"
	StatusUnavailable RequestStatus = "unavailable"
)

// ActiveStatuses are statuses shown on the active requests page
var ActiveStatuses = []RequestStatus{StatusPending, StatusContacted, StatusWorking}

// PastStatuses are statuses shown on the past requests page
var PastStatuses = []RequestStatus{StatusDone, StatusDeclined, StatusUnavailable}

// IsTerminalStatus returns true if the status is terminal (no further transitions allowed)
func (s RequestStatus) IsTerminalStatus() bool {
	return s == StatusDone || s == StatusDeclined || s == StatusUnavailable
}

// CanTransitionTo checks if a status transition is valid
func (s RequestStatus) CanTransitionTo(newStatus RequestStatus) bool {
	// Terminal statuses cannot transition
	if s.IsTerminalStatus() {
		return false
	}

	switch s {
	case StatusPending:
		return newStatus == StatusContacted || newStatus == StatusDeclined
	case StatusContacted:
		return newStatus == StatusWorking || newStatus == StatusDeclined
	case StatusWorking:
		return newStatus == StatusDone || newStatus == StatusDeclined
	default:
		return false
	}
}

// DeclineReason represents predefined decline reasons
type DeclineReason string

const (
	DeclineNoTime        DeclineReason = "no_time"
	DeclineTopicMismatch DeclineReason = "topic_mismatch"
	DeclineHelpingOthers DeclineReason = "helping_others"
	DeclineOnBreak       DeclineReason = "on_break"
	DeclineOther         DeclineReason = "other"
)

// MentorClientRequest represents a mentee's request to a mentor (full admin view)
type MentorClientRequest struct {
	ID              string        `json:"id"`
	Email           string        `json:"email"`
	Name            string        `json:"name"`
	Telegram        string        `json:"telegram"`
	Details         string        `json:"details"`
	Level           string        `json:"level"`
	CreatedAt       time.Time     `json:"createdAt"`
	ModifiedAt      time.Time     `json:"modifiedAt"`
	StatusChangedAt time.Time     `json:"statusChangedAt"`
	ScheduledAt     *time.Time    `json:"scheduledAt"`
	Review          *string       `json:"review"`
	ReviewURL       *string       `json:"reviewUrl"`
	Status          RequestStatus `json:"status"`
	MentorID        string        `json:"mentorId"`
	DeclineReason   string        `json:"declineReason"`
	DeclineComment  *string       `json:"declineComment"`
}

// UpdateStatusRequest is the payload for updating request status
type UpdateStatusRequest struct {
	Status RequestStatus `json:"status" binding:"required,oneof=pending contacted working done declined unavailable"`
}

// DeclineRequestPayload is the payload for declining a request
type DeclineRequestPayload struct {
	Reason  DeclineReason `json:"reason" binding:"required,oneof=no_time topic_mismatch helping_others on_break other"`
	Comment string        `json:"comment" binding:"max=1000"`
}

// ClientRequestsResponse is the response for listing requests
type ClientRequestsResponse struct {
	Requests []MentorClientRequest `json:"requests"`
	Total    int                   `json:"total"`
}

// RequestGroup represents the type of requests to fetch
type RequestGroup string

const (
	RequestGroupActive RequestGroup = "active"
	RequestGroupPast   RequestGroup = "past"
)

// GetStatuses returns the statuses for a request group
func (g RequestGroup) GetStatuses() []RequestStatus {
	switch g {
	case RequestGroupActive:
		return ActiveStatuses
	case RequestGroupPast:
		return PastStatuses
	default:
		return nil
	}
}

// ScanClientRequest scans a single PostgreSQL row into a MentorClientRequest struct
// Expected columns: id, mentor_id, email, name, telegram, description, level, status,
// created_at, updated_at, status_changed_at, scheduled_at, decline_reason, decline_comment,
// mentor_review (from LEFT JOIN reviews)
func ScanClientRequest(row pgx.Row) (*MentorClientRequest, error) {
	var r MentorClientRequest
	var scheduledAt *time.Time
	var review *string
	var declineComment *string

	err := row.Scan(
		&r.ID,
		&r.MentorID,
		&r.Email,
		&r.Name,
		&r.Telegram,
		&r.Details,
		&r.Level,
		&r.Status,
		&r.CreatedAt,
		&r.ModifiedAt,
		&r.StatusChangedAt,
		&scheduledAt,
		&r.DeclineReason,
		&declineComment,
		&review, // from LEFT JOIN reviews
	)
	if err != nil {
		return nil, err
	}

	// Set nullable fields
	r.ScheduledAt = scheduledAt
	r.DeclineComment = declineComment
	r.Review = review

	// Compute ReviewURL from constant base URL + request ID
	// Format: https://getmentor.dev/review?id={requestId}
	reviewURL := fmt.Sprintf("https://getmentor.dev/review?id=%s", r.ID)
	r.ReviewURL = &reviewURL

	return &r, nil
}

// ScanClientRequests scans multiple PostgreSQL rows into a slice of MentorClientRequest structs
func ScanClientRequests(rows pgx.Rows) ([]*MentorClientRequest, error) {
	defer rows.Close()

	requests := []*MentorClientRequest{}
	for rows.Next() {
		request, err := ScanClientRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

// Deprecated: AirtableRecordToMentorClientRequest is deprecated and will be removed in Task 2.11
// Use ScanClientRequest for PostgreSQL row scanning instead
// AirtableRecordToMentorClientRequest converts an Airtable record to MentorClientRequest
func AirtableRecordToMentorClientRequest(record *airtable.Record) *MentorClientRequest {
	getString := func(field string) string {
		if v, ok := record.Fields[field].(string); ok {
			return v
		}
		return ""
	}

	getStringPtr := func(field string) *string {
		if v, ok := record.Fields[field].(string); ok && v != "" {
			return &v
		}
		return nil
	}

	getLookupStringPtr := func(field string) *string {
		if arr, ok := record.Fields[field].([]interface{}); ok && len(arr) > 0 {
			if v, ok := arr[0].(string); ok && v != "" {
				return &v
			}
		}
		return nil
	}

	getTime := func(field string) time.Time {
		if v, ok := record.Fields[field].(string); ok && v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err == nil {
				return t
			}
		}
		return time.Time{}
	}

	getTimePtr := func(field string) *time.Time {
		if v, ok := record.Fields[field].(string); ok && v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err == nil {
				return &t
			}
		}
		return nil
	}

	getMentorID := func() string {
		if mentors, ok := record.Fields["Mentor"].([]interface{}); ok && len(mentors) > 0 {
			if mentorID, ok := mentors[0].(string); ok {
				return mentorID
			}
		}
		return ""
	}

	review := getStringPtr("Review")
	if review == nil {
		review = getLookupStringPtr("Review2")
	}

	return &MentorClientRequest{
		ID:              record.ID,
		Email:           getString("Email"),
		Name:            getString("Name"),
		Telegram:        getString("Telegram"),
		Details:         getString("Description"),
		Level:           getString("Level"),
		CreatedAt:       getTime("Created Time"),
		ModifiedAt:      getTime("Last Modified Time"),
		StatusChangedAt: getTime("Last Status Change"),
		ScheduledAt:     getTimePtr("Scheduled At"),
		Review:          review,
		ReviewURL:       getStringPtr("ReviewFormUrl"),
		Status:          RequestStatus(getString("Status")),
		MentorID:        getMentorID(),
		DeclineReason:   getString("DeclineReason"),
		DeclineComment:  getStringPtr("DeclineComment"),
	}
}
