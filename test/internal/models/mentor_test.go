package models_test

import (
	"testing"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestGetCalendarType(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "empty URL returns none",
			url:      "",
			expected: "none",
		},
		{
			name:     "calendly URL detected",
			url:      "https://calendly.com/johndoe",
			expected: "calendly",
		},
		{
			name:     "calendly URL with uppercase",
			url:      "https://Calendly.com/johndoe",
			expected: "calendly",
		},
		{
			name:     "koalendar URL detected",
			url:      "https://koalendar.com/johndoe",
			expected: "koalendar",
		},
		{
			name:     "calendlab URL detected",
			url:      "https://calendlab.com/johndoe",
			expected: "calendlab",
		},
		{
			name:     "unknown calendar service returns url",
			url:      "https://example.com/calendar",
			expected: "url",
		},
		{
			name:     "partial match calendly",
			url:      "https://app.calendly.com/johndoe/30min",
			expected: "calendly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := models.GetCalendarType(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMentorSponsor(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected string
	}{
		{
			name:     "no sponsor tags returns none",
			tags:     []string{"React", "JavaScript", "Frontend"},
			expected: "none",
		},
		{
			name:     "Ontico sponsor tag",
			tags:     []string{"React", "Ontico", "JavaScript"},
			expected: "Ontico",
		},
		{
			name:     "TensorSoft sponsor tag",
			tags:     []string{"Backend", "ТензорСофт", "Go"},
			expected: "ТензорСофт",
		},
		{
			name:     "multiple sponsor tags",
			tags:     []string{"React", "Ontico", "ТензорСофт", "Go"},
			expected: "Ontico|ТензорСофт",
		},
		{
			name:     "empty tags returns none",
			tags:     []string{},
			expected: "none",
		},
		{
			name:     "nil tags returns none",
			tags:     nil,
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := models.GetMentorSponsor(tt.tags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAirtableRecordToMentor(t *testing.T) {
	tests := []struct {
		name     string
		record   *models.AirtableRecord
		expected *models.Mentor
	}{
		{
			name: "complete record conversion",
			record: &models.AirtableRecord{
				ID: "rec123",
				Fields: struct {
					Id                int    `json:"Id"`
					Alias             string `json:"Alias"`
					Name              string `json:"Name"`
					Description       string `json:"Description"`
					JobTitle          string `json:"JobTitle"`
					Workplace         string `json:"Workplace"`
					Details           string `json:"Details"`
					About             string `json:"About"`
					Competencies      string `json:"Competencies"`
					Experience        string `json:"Experience"`
					Price             string `json:"Price"`
					DoneSessionsCount int    `json:"Done Sessions Count"`
					ImageAttachment   []struct {
						URL string `json:"url"`
					} `json:"Image_Attachment"`
					Image       string `json:"Image"`
					Tags        string `json:"Tags"`
					SortOrder   int    `json:"SortOrder"`
					OnSite      int    `json:"OnSite"`
					Status      string `json:"Status"`
					AuthToken   string `json:"AuthToken"`
					CalendlyURL string `json:"Calendly Url"`
					IsNew       int    `json:"Is New"`
				}{
					Id:                1,
					Alias:             "john-doe",
					Name:              "John Doe",
					Description:       "Short description",
					JobTitle:          "Senior Engineer",
					Workplace:         "TechCorp",
					Details:           "Detailed description",
					About:             "About me",
					Competencies:      "React, Go, TypeScript",
					Experience:        "10 years",
					Price:             "$100/hour",
					DoneSessionsCount: 25,
					Image:             "https://example.com/photo.jpg",
					Tags:              "React, JavaScript, Ontico",
					SortOrder:         1,
					OnSite:            1,
					Status:            "active",
					AuthToken:         "token123",
					CalendlyURL:       "https://calendly.com/johndoe",
					IsNew:             1,
				},
			},
			expected: &models.Mentor{
				ID:           1,
				AirtableID:   "rec123",
				Slug:         "john-doe",
				Name:         "John Doe",
				Job:          "Senior Engineer",
				Workplace:    "TechCorp",
				Description:  "Detailed description",
				About:        "About me",
				Competencies: "React, Go, TypeScript",
				Experience:   "10 years",
				Price:        "$100/hour",
				MenteeCount:  25,
				PhotoURL:     "https://example.com/photo.jpg",
				Tags:         []string{"React", "JavaScript", "Ontico"},
				SortOrder:    1,
				IsVisible:    true,
				Sponsors:     "Ontico",
				CalendarType: "calendly",
				IsNew:        true,
				AuthToken:    "token123",
				CalendarURL:  "https://calendly.com/johndoe",
			},
		},
		{
			name: "inactive mentor should not be visible",
			record: &models.AirtableRecord{
				ID: "rec456",
				Fields: struct {
					Id                int    `json:"Id"`
					Alias             string `json:"Alias"`
					Name              string `json:"Name"`
					Description       string `json:"Description"`
					JobTitle          string `json:"JobTitle"`
					Workplace         string `json:"Workplace"`
					Details           string `json:"Details"`
					About             string `json:"About"`
					Competencies      string `json:"Competencies"`
					Experience        string `json:"Experience"`
					Price             string `json:"Price"`
					DoneSessionsCount int    `json:"Done Sessions Count"`
					ImageAttachment   []struct {
						URL string `json:"url"`
					} `json:"Image_Attachment"`
					Image       string `json:"Image"`
					Tags        string `json:"Tags"`
					SortOrder   int    `json:"SortOrder"`
					OnSite      int    `json:"OnSite"`
					Status      string `json:"Status"`
					AuthToken   string `json:"AuthToken"`
					CalendlyURL string `json:"Calendly Url"`
					IsNew       int    `json:"Is New"`
				}{
					Id:        2,
					Alias:     "jane-doe",
					Name:      "Jane Doe",
					OnSite:    1,
					Status:    "inactive",
					Tags:      "",
					SortOrder: 2,
				},
			},
			expected: &models.Mentor{
				ID:           2,
				AirtableID:   "rec456",
				Slug:         "jane-doe",
				Name:         "Jane Doe",
				Tags:         []string{},
				SortOrder:    2,
				IsVisible:    false,
				Sponsors:     "none",
				CalendarType: "none",
				IsNew:        false,
			},
		},
		{
			name: "photo from attachment when Image is empty",
			record: &models.AirtableRecord{
				ID: "rec789",
				Fields: struct {
					Id                int    `json:"Id"`
					Alias             string `json:"Alias"`
					Name              string `json:"Name"`
					Description       string `json:"Description"`
					JobTitle          string `json:"JobTitle"`
					Workplace         string `json:"Workplace"`
					Details           string `json:"Details"`
					About             string `json:"About"`
					Competencies      string `json:"Competencies"`
					Experience        string `json:"Experience"`
					Price             string `json:"Price"`
					DoneSessionsCount int    `json:"Done Sessions Count"`
					ImageAttachment   []struct {
						URL string `json:"url"`
					} `json:"Image_Attachment"`
					Image       string `json:"Image"`
					Tags        string `json:"Tags"`
					SortOrder   int    `json:"SortOrder"`
					OnSite      int    `json:"OnSite"`
					Status      string `json:"Status"`
					AuthToken   string `json:"AuthToken"`
					CalendlyURL string `json:"Calendly Url"`
					IsNew       int    `json:"Is New"`
				}{
					Id:    3,
					Alias: "bob-smith",
					Name:  "Bob Smith",
					ImageAttachment: []struct {
						URL string `json:"url"`
					}{
						{URL: "https://example.com/attachment.jpg"},
					},
					Image:     "",
					OnSite:    1,
					Status:    "active",
					Tags:      "Go, Backend",
					SortOrder: 3,
				},
			},
			expected: &models.Mentor{
				ID:           3,
				AirtableID:   "rec789",
				Slug:         "bob-smith",
				Name:         "Bob Smith",
				PhotoURL:     "https://example.com/attachment.jpg",
				Tags:         []string{"Go", "Backend"},
				SortOrder:    3,
				IsVisible:    true,
				Sponsors:     "none",
				CalendarType: "none",
				IsNew:        false,
			},
		},
		{
			name: "tags with extra whitespace are trimmed",
			record: &models.AirtableRecord{
				ID: "rec101",
				Fields: struct {
					Id                int    `json:"Id"`
					Alias             string `json:"Alias"`
					Name              string `json:"Name"`
					Description       string `json:"Description"`
					JobTitle          string `json:"JobTitle"`
					Workplace         string `json:"Workplace"`
					Details           string `json:"Details"`
					About             string `json:"About"`
					Competencies      string `json:"Competencies"`
					Experience        string `json:"Experience"`
					Price             string `json:"Price"`
					DoneSessionsCount int    `json:"Done Sessions Count"`
					ImageAttachment   []struct {
						URL string `json:"url"`
					} `json:"Image_Attachment"`
					Image       string `json:"Image"`
					Tags        string `json:"Tags"`
					SortOrder   int    `json:"SortOrder"`
					OnSite      int    `json:"OnSite"`
					Status      string `json:"Status"`
					AuthToken   string `json:"AuthToken"`
					CalendlyURL string `json:"Calendly Url"`
					IsNew       int    `json:"Is New"`
				}{
					Id:        4,
					Alias:     "alice",
					Name:      "Alice",
					Tags:      " React ,  Vue  , Angular ",
					OnSite:    1,
					Status:    "active",
					SortOrder: 4,
				},
			},
			expected: &models.Mentor{
				ID:           4,
				AirtableID:   "rec101",
				Slug:         "alice",
				Name:         "Alice",
				Tags:         []string{"React", "Vue", "Angular"},
				SortOrder:    4,
				IsVisible:    true,
				Sponsors:     "none",
				CalendarType: "none",
				IsNew:        false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.record.ToMentor()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMentorToPublicResponse(t *testing.T) {
	mentor := &models.Mentor{
		ID:           1,
		AirtableID:   "rec123",
		Slug:         "john-doe",
		Name:         "John Doe",
		Job:          "Senior Engineer",
		Workplace:    "TechCorp",
		Description:  "Detailed description",
		About:        "About me",
		Competencies: "React, Go, TypeScript",
		Experience:   "10 years",
		Price:        "$100/hour",
		MenteeCount:  25,
		PhotoURL:     "https://example.com/photo.jpg",
		Tags:         []string{"React", "JavaScript", "Frontend"},
		SortOrder:    1,
		IsVisible:    true,
		Sponsors:     "Ontico",
		CalendarType: "calendly",
		IsNew:        true,
	}

	baseURL := "https://getmentor.dev"

	expected := models.PublicMentorResponse{
		ID:           1,
		Name:         "John Doe",
		Title:        "Senior Engineer",
		Workplace:    "TechCorp",
		About:        "About me",
		Description:  "Detailed description",
		Competencies: "React, Go, TypeScript",
		Experience:   "10 years",
		Price:        "$100/hour",
		DoneSessions: 25,
		Photo:        "https://example.com/photo.jpg",
		Tags:         "React,JavaScript,Frontend",
		Link:         "https://getmentor.dev/mentor/john-doe",
	}

	result := mentor.ToPublicResponse(baseURL)
	assert.Equal(t, expected, result)
}

func TestMentorToPublicResponseWithEmptyTags(t *testing.T) {
	mentor := &models.Mentor{
		ID:          2,
		Slug:        "jane-doe",
		Name:        "Jane Doe",
		Job:         "Engineer",
		Tags:        []string{},
		MenteeCount: 5,
	}

	baseURL := "https://getmentor.dev"

	result := mentor.ToPublicResponse(baseURL)
	assert.Equal(t, "", result.Tags, "Empty tags should result in empty string")
	assert.Equal(t, "https://getmentor.dev/mentor/jane-doe", result.Link)
}
