package models

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Mentor represents a mentor in the system
type Mentor struct {
	MentorID     string   `json:"mentorId"`     // UUID primary key
	LegacyID     int      `json:"id"`           // Old integer ID (maps to legacy_id column)
	AirtableID   *string  `json:"airtableId"`   // Nullable - for backwards compatibility during migration
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Job          string   `json:"job"`
	Workplace    string   `json:"workplace"`
	Description  string   `json:"description"`
	About        string   `json:"about"`
	Competencies string   `json:"competencies"`
	Experience   string   `json:"experience"`
	Price        string   `json:"price"`
	MenteeCount  int      `json:"menteeCount"`
	Tags         []string `json:"tags"`
	SortOrder    int      `json:"sortOrder"`
	IsVisible    bool     `json:"isVisible"`    // Computed: status = 'active' AND telegram_chat_id IS NOT NULL
	Sponsors     string   `json:"sponsors"`
	CalendarType string   `json:"calendarType"`
	IsNew        bool     `json:"isNew"`        // Computed: created_at > NOW() - 14 days

	// Status field for login eligibility checks
	Status string `json:"status"`

	// Secure fields (cleared by repository unless ShowHidden is true)
	CalendarURL string `json:"calendarUrl"`

	// Internal fields (not exposed in JSON)
	TelegramChatID *int64    `json:"-"` // Used for IsVisible computation
	CreatedAt      time.Time `json:"-"` // Used for IsNew computation
}

// PublicMentorResponse represents the public API response format
type PublicMentorResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Workplace    string `json:"workplace"`
	About        string `json:"about"`
	Description  string `json:"description"`
	Competencies string `json:"competencies"`
	Experience   string `json:"experience"`
	Price        string `json:"price"`
	DoneSessions int    `json:"doneSessions"`
	Tags         string `json:"tags"`
	Link         string `json:"link"`
}

// ToPublicResponse converts a Mentor to PublicMentorResponse
func (m *Mentor) ToPublicResponse(baseURL string) PublicMentorResponse {
	return PublicMentorResponse{
		ID:           m.LegacyID, // Use LegacyID for backwards compatibility
		Name:         m.Name,
		Title:        m.Job,
		Workplace:    m.Workplace,
		About:        m.About,
		Description:  m.Description,
		Competencies: m.Competencies,
		Experience:   m.Experience,
		Price:        m.Price,
		DoneSessions: m.MenteeCount,
		Tags:         strings.Join(m.Tags, ","),
		Link:         baseURL + "/mentor/" + m.Slug,
	}
}

// FilterOptions represents options for filtering mentors
type FilterOptions struct {
	OnlyVisible    bool
	ShowHidden     bool
	DropLongFields bool
	ForceRefresh   bool
}

// ScanMentor scans a single PostgreSQL row into a Mentor struct
func ScanMentor(row pgx.Row) (*Mentor, error) {
	var m Mentor
	var tagsStr *string
	var airtableID *string
	var telegramChatID *int64

	err := row.Scan(
		&m.MentorID,
		&airtableID,
		&m.LegacyID,
		&m.Slug,
		&m.Name,
		&m.Job,
		&m.Workplace,
		&m.About,
		&m.Description,
		&m.Competencies,
		&m.Experience,
		&m.Price,
		&m.Status,
		&tagsStr,
		&telegramChatID,
		&m.CalendarURL,
		&m.SortOrder,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Set nullable fields
	m.AirtableID = airtableID
	m.TelegramChatID = telegramChatID

	// Parse tags from comma-separated string
	m.Tags = []string{}
	if tagsStr != nil && *tagsStr != "" {
		for _, tag := range strings.Split(*tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				m.Tags = append(m.Tags, tag)
			}
		}
	}

	// Compute IsVisible: status = 'active' AND telegram_chat_id IS NOT NULL
	m.IsVisible = m.Status == "active" && telegramChatID != nil

	// Compute IsNew: created_at > NOW() - 14 days
	fourteenDaysAgo := time.Now().AddDate(0, 0, -14)
	m.IsNew = m.CreatedAt.After(fourteenDaysAgo)

	// Determine calendar type
	m.CalendarType = GetCalendarType(m.CalendarURL)

	// Get sponsor from tags
	m.Sponsors = GetMentorSponsor(m.Tags)

	// Note: MenteeCount should be computed separately via COUNT query
	// We'll set it to 0 here and let the repository populate it if needed

	return &m, nil
}

// ScanMentors scans multiple PostgreSQL rows into a slice of Mentor structs
func ScanMentors(rows pgx.Rows) ([]*Mentor, error) {
	defer rows.Close()

	mentors := []*Mentor{}
	for rows.Next() {
		mentor, err := ScanMentor(rows)
		if err != nil {
			return nil, err
		}
		mentors = append(mentors, mentor)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return mentors, nil
}

// GetCalendarType determines the calendar service type from URL
func GetCalendarType(url string) string {
	if url == "" {
		return "none"
	}

	url = strings.ToLower(url)

	switch {
	case strings.Contains(url, "calendly.com"):
		return "calendly"
	case strings.Contains(url, "koalendar.com"):
		return "koalendar"
	case strings.Contains(url, "calendlab.com"):
		return "calendlab"
	default:
		return "url"
	}
}

// GetMentorSponsor extracts sponsor information from tags
func GetMentorSponsor(tags []string) string {
	sponsorTags := map[string]bool{
		"Сообщество Онтико": true,
		"Эксперт Авито":     true,
	}

	sponsors := []string{}
	for _, tag := range tags {
		if sponsorTags[tag] {
			sponsors = append(sponsors, tag)
		}
	}

	if len(sponsors) == 0 {
		return "none"
	}

	return strings.Join(sponsors, "|")
}


