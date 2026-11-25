package models

import (
	"net/url"
	"strings"
	"time"
)

// Mentee is a person who sent at least one request and has asked to log in.
// The row is keyed by email; it holds no profile data.
type Mentee struct {
	ID             string
	Email          string
	SessionVersion int
}

// MenteeSession is the authenticated mentee session. It carries no email or name.
type MenteeSession struct {
	MenteeID       string `json:"mentee_id"`
	SessionVersion int    `json:"-"`
	ExpiresAt      int64  `json:"exp"`
	IssuedAt       int64  `json:"iat"`
}

// MenteeRequestLoginRequest is the payload for requesting a mentee login link.
type MenteeRequestLoginRequest struct {
	Email string `json:"email" binding:"required,email,max=255"`
}

// MenteeVerifyLoginRequest is the payload for exchanging a login token for a session.
type MenteeVerifyLoginRequest struct {
	Token string `json:"token" binding:"required,min=20,max=200"`
}

// MenteeVerifyLoginResponse is returned after a successful verification.
type MenteeVerifyLoginResponse struct {
	Success bool           `json:"success"`
	Session *MenteeSession `json:"session,omitempty"`
}

// MenteeSubmitReviewRequest is a review left from the dashboard. The request id travels
// in the body, never in the URL: legacy review links use it as a capability.
// Field limits match SubmitReviewRequest.
type MenteeSubmitReviewRequest struct {
	RequestID      string `json:"requestId" binding:"required,uuid"`
	MentorReview   string `json:"mentorReview" binding:"required,min=10,max=5000"`
	PlatformReview string `json:"platformReview" binding:"max=5000"`
	Improvements   string `json:"improvements" binding:"max=5000"`
	RecaptchaToken string `json:"recaptchaToken" binding:"required"`
}

// MenteeRequestGroup is the dashboard bucket a request is shown in.
type MenteeRequestGroup string

const (
	MenteeGroupPending MenteeRequestGroup = "pending"
	MenteeGroupActive  MenteeRequestGroup = "active"
	MenteeGroupDone    MenteeRequestGroup = "done"
	MenteeGroupClosed  MenteeRequestGroup = "closed"
)

// MenteeGroupForStatus maps a client_requests.status value to its dashboard bucket.
// 'reschedule' is in the DB constraint (used by the Telegram bot) though not in RequestStatus.
func MenteeGroupForStatus(status string) MenteeRequestGroup {
	switch status {
	case "pending":
		return MenteeGroupPending
	case "contacted", "working", "reschedule":
		return MenteeGroupActive
	case "done":
		return MenteeGroupDone
	default: // declined, unavailable
		return MenteeGroupClosed
	}
}

// MenteeRequestRow is one row of the mentee dashboard query. Every column that can be
// NULL in the DB (or through a LEFT JOIN) is a pointer, so one odd row can't break the list.
type MenteeRequestRow struct {
	ID              string
	Status          string
	Description     *string
	CreatedAt       time.Time
	StatusChangedAt *time.Time
	DeclineReason   *string
	DeclineComment  *string

	MentorID             *string
	MentorName           *string
	MentorSlug           *string
	MentorCalendarURL    *string
	MentorStatus         *string
	MentorHasTelegramBot bool
	MentorDeletedAt      *time.Time

	HasReview            bool
	ReviewMentorReview   *string
	ReviewPlatformReview *string
	ReviewImprovements   *string
	ReviewCreatedAt      *time.Time
}

// MenteeDashboardMentor is what a mentee may see about a mentor: never email or messenger.
type MenteeDashboardMentor struct {
	Name        string  `json:"name"`
	Slug        *string `json:"slug"`        // only while the profile is publicly visible
	CalendarURL *string `json:"calendarUrl"` // only for pending/active requests, http(s) only
}

// MenteeDashboardReview is the mentee's own review, shown read-only.
type MenteeDashboardReview struct {
	MentorReview   string     `json:"mentorReview"`
	PlatformReview string     `json:"platformReview"`
	Improvements   string     `json:"improvements"`
	CreatedAt      *time.Time `json:"createdAt"`
}

// MenteeDashboardRequest is one card of the mentee dashboard.
type MenteeDashboardRequest struct {
	ID              string                 `json:"id"`
	Status          string                 `json:"status"`
	Group           MenteeRequestGroup     `json:"group"`
	Details         string                 `json:"details"`
	CreatedAt       time.Time              `json:"createdAt"`
	StatusChangedAt *time.Time             `json:"statusChangedAt"`
	DeclineReason   *string                `json:"declineReason"`
	DeclineComment  *string                `json:"declineComment"`
	Mentor          *MenteeDashboardMentor `json:"mentor"` // nil when the mentor deleted their profile
	CanReview       bool                   `json:"canReview"`
	Review          *MenteeDashboardReview `json:"review"`
}

// MenteeRequestsResponse is the dashboard list.
type MenteeRequestsResponse struct {
	Requests []MenteeDashboardRequest `json:"requests"`
}

// IsMentorGone reports whether the request's mentor no longer exists or deleted their profile.
func (r *MenteeRequestRow) IsMentorGone() bool {
	return r.MentorID == nil || r.MentorDeletedAt != nil || r.MentorName == nil
}

// IsMentorPubliclyVisible mirrors Mentor.IsVisible: active, bot connected, not deleted.
func (r *MenteeRequestRow) IsMentorPubliclyVisible() bool {
	return !r.IsMentorGone() && r.MentorStatus != nil && *r.MentorStatus == "active" && r.MentorHasTelegramBot
}

// CanReview reports whether the mentee may leave a review from the dashboard.
func (r *MenteeRequestRow) CanReview() bool {
	return r.Status == "done" && !r.HasReview && !r.IsMentorGone()
}

// SafeCalendarURL returns the URL only if it is an absolute http(s) URL.
func SafeCalendarURL(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	return &trimmed
}

// ToDashboardRequest applies every visibility rule server-side, so the API response
// itself never contains what the mentee must not see.
func (r *MenteeRequestRow) ToDashboardRequest() MenteeDashboardRequest {
	group := MenteeGroupForStatus(r.Status)
	out := MenteeDashboardRequest{
		ID:              r.ID,
		Status:          r.Status,
		Group:           group,
		Details:         derefString(r.Description),
		CreatedAt:       r.CreatedAt,
		StatusChangedAt: r.StatusChangedAt,
		CanReview:       r.CanReview(),
	}

	if group == MenteeGroupClosed {
		out.DeclineReason = r.DeclineReason
		out.DeclineComment = r.DeclineComment
	}

	if !r.IsMentorGone() {
		mentor := &MenteeDashboardMentor{Name: *r.MentorName}
		if r.IsMentorPubliclyVisible() && r.MentorSlug != nil && *r.MentorSlug != "" {
			mentor.Slug = r.MentorSlug
		}
		if group == MenteeGroupPending || group == MenteeGroupActive {
			mentor.CalendarURL = SafeCalendarURL(r.MentorCalendarURL)
		}
		out.Mentor = mentor
	}

	if r.HasReview {
		out.Review = &MenteeDashboardReview{
			MentorReview:   derefString(r.ReviewMentorReview),
			PlatformReview: derefString(r.ReviewPlatformReview),
			Improvements:   derefString(r.ReviewImprovements),
			CreatedAt:      r.ReviewCreatedAt,
		}
	}
	return out
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
