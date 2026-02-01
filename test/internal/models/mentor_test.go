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
			name:     "Сообщество Онтико sponsor tag",
			tags:     []string{"React", "Сообщество Онтико", "JavaScript"},
			expected: "Сообщество Онтико",
		},
		{
			name:     "Эксперт Авито sponsor tag",
			tags:     []string{"Backend", "Эксперт Авито", "Go"},
			expected: "Эксперт Авито",
		},
		{
			name:     "multiple sponsor tags",
			tags:     []string{"React", "Сообщество Онтико", "Эксперт Авито", "Go"},
			expected: "Сообщество Онтико|Эксперт Авито",
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

// TestAirtableRecordToMentor tests removed - Airtable conversion no longer used
// See mentor_scan_test.go for PostgreSQL row scanning tests

func TestMentorToPublicResponse(t *testing.T) {
	mentor := &models.Mentor{
		LegacyID:     1,
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
		Tags:         []string{"React", "JavaScript", "Frontend"},
		SortOrder:    1,
		IsVisible:    true,
		Sponsors:     "Сообщество Онтико",
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
		Tags:         "React,JavaScript,Frontend",
		Link:         "https://getmentor.dev/mentor/john-doe",
	}

	result := mentor.ToPublicResponse(baseURL)
	assert.Equal(t, expected, result)
}

func TestMentorToPublicResponseWithEmptyTags(t *testing.T) {
	mentor := &models.Mentor{
		LegacyID:    2,
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
