package models

import (
	"strings"
)

// Mentor represents a mentor in the system
type Mentor struct {
	ID            int      `json:"id"`
	AirtableID    string   `json:"airtableId"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Job           string   `json:"job"`
	Workplace     string   `json:"workplace"`
	Description   string   `json:"description"`
	About         string   `json:"about"`
	Competencies  string   `json:"competencies"`
	Experience    string   `json:"experience"`
	Price         string   `json:"price"`
	MenteeCount   int      `json:"menteeCount"`
	PhotoURL      string   `json:"photo_url"`
	Tags          []string `json:"tags"`
	SortOrder     int      `json:"sortOrder"`
	IsVisible     bool     `json:"isVisible"`
	Sponsors      string   `json:"sponsors"`
	CalendarType  string   `json:"calendarType"`
	IsNew         bool     `json:"isNew"`

	// Secure fields (not serialized by default)
	AuthToken   string `json:"-"`
	CalendarURL string `json:"-"`
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
	Photo        string `json:"photo"`
	Tags         string `json:"tags"`
	Link         string `json:"link"`
}

// ToPublicResponse converts a Mentor to PublicMentorResponse
func (m *Mentor) ToPublicResponse(baseURL string) PublicMentorResponse {
	return PublicMentorResponse{
		ID:           m.ID,
		Name:         m.Name,
		Title:        m.Job,
		Workplace:    m.Workplace,
		About:        m.About,
		Description:  m.Description,
		Competencies: m.Competencies,
		Experience:   m.Experience,
		Price:        m.Price,
		DoneSessions: m.MenteeCount,
		Photo:        m.PhotoURL,
		Tags:         strings.Join(m.Tags, ","),
		Link:         baseURL + "/mentor/" + m.Slug,
	}
}

// FilterOptions represents options for filtering mentors
type FilterOptions struct {
	OnlyVisible     bool
	ShowHidden      bool
	DropLongFields  bool
	ForceRefresh    bool
}

// AirtableRecord represents the raw Airtable mentor record
type AirtableRecord struct {
	ID     string
	Fields struct {
		Id                  int      `json:"Id"`
		Alias               string   `json:"Alias"`
		Name                string   `json:"Name"`
		Description         string   `json:"Description"`
		JobTitle            string   `json:"JobTitle"`
		Workplace           string   `json:"Workplace"`
		Details             string   `json:"Details"`
		About               string   `json:"About"`
		Competencies        string   `json:"Competencies"`
		Experience          string   `json:"Experience"`
		Price               string   `json:"Price"`
		DoneSessionsCount   int      `json:"Done Sessions Count"`
		ImageAttachment     []struct {
			URL string `json:"url"`
		} `json:"Image_Attachment"`
		Image               string   `json:"Image"`
		Tags                string   `json:"Tags"`
		SortOrder           int      `json:"SortOrder"`
		OnSite              int      `json:"OnSite"`
		Status              string   `json:"Status"`
		AuthToken           string   `json:"AuthToken"`
		CalendlyURL         string   `json:"Calendly Url"`
		IsNew               int      `json:"Is New"`
	}
}

// ToMentor converts an AirtableRecord to a Mentor
func (ar *AirtableRecord) ToMentor() *Mentor {
	// Parse tags
	tags := []string{}
	if ar.Fields.Tags != "" {
		for _, tag := range strings.Split(ar.Fields.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Determine visibility
	isVisible := ar.Fields.OnSite == 1 && ar.Fields.Status == "active"

	// Get photo URL
	photoURL := ar.Fields.Image
	if photoURL == "" && len(ar.Fields.ImageAttachment) > 0 {
		photoURL = ar.Fields.ImageAttachment[0].URL
	}

	// Determine calendar type
	calendarType := getCalendarType(ar.Fields.CalendlyURL)

	// Get sponsor
	sponsor := getMentorSponsor(tags)

	return &Mentor{
		ID:           ar.Fields.Id,
		AirtableID:   ar.ID,
		Slug:         ar.Fields.Alias,
		Name:         ar.Fields.Name,
		Job:          ar.Fields.JobTitle,
		Workplace:    ar.Fields.Workplace,
		Description:  ar.Fields.Details,
		About:        ar.Fields.About,
		Competencies: ar.Fields.Competencies,
		Experience:   ar.Fields.Experience,
		Price:        ar.Fields.Price,
		MenteeCount:  ar.Fields.DoneSessionsCount,
		PhotoURL:     photoURL,
		Tags:         tags,
		SortOrder:    ar.Fields.SortOrder,
		IsVisible:    isVisible,
		Sponsors:     sponsor,
		CalendarType: calendarType,
		IsNew:        ar.Fields.IsNew == 1,
		AuthToken:    ar.Fields.AuthToken,
		CalendarURL:  ar.Fields.CalendlyURL,
	}
}

// getCalendarType determines the calendar service type from URL
func getCalendarType(url string) string {
	if url == "" {
		return "none"
	}

	url = strings.ToLower(url)

	if strings.Contains(url, "calendly.com") {
		return "calendly"
	} else if strings.Contains(url, "koalendar.com") {
		return "koalendar"
	} else if strings.Contains(url, "calendlab.com") {
		return "calendlab"
	}

	return "url"
}

// getMentorSponsor extracts sponsor information from tags
func getMentorSponsor(tags []string) string {
	sponsorTags := map[string]bool{
		"Ontico":    true,
		"ТензорСофт": true,
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
