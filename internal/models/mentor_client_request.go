package models

import (
	"time"

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
