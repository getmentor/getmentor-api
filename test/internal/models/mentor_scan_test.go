package models_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// mockRow implements pgx.Row interface for testing
type mockRow struct {
	values []interface{}
	err    error
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}

	for i, v := range m.values {
		if i >= len(dest) {
			continue
		}

		switch d := dest[i].(type) {
		case *string:
			if str, ok := v.(string); ok {
				*d = str
			}
		case **string:
			// Handle nullable string fields
			if v == nil {
				*d = nil
			} else if str, ok := v.(string); ok {
				temp := str
				*d = &temp
			}
		case *int:
			if num, ok := v.(int); ok {
				*d = num
			}
		case *int64:
			if num, ok := v.(int64); ok {
				*d = num
			}
		case **int64:
			// Handle nullable int64 fields
			if v == nil {
				*d = nil
			} else if num, ok := v.(int64); ok {
				temp := num
				*d = &temp
			}
		case *time.Time:
			if t, ok := v.(time.Time); ok {
				*d = t
			}
		}
	}

	return nil
}

// TestScanMentor verifies that ScanMentor correctly scans a PostgreSQL row
func TestScanMentor(t *testing.T) {
	// Prepare test data
	mentorID := "550e8400-e29b-41d4-a716-446655440000"
	airtableID := "rec123456" // Still in database schema, scanned but not stored in model
	legacyID := 42
	slug := "ivan-ivanov"
	name := "Иван Иванов"
	job := "Senior Engineer"
	workplace := "Tech Corp"
	about := "About me"
	description := "Description"
	competencies := "Go, PostgreSQL"
	experience := "5-10"
	price := "5000"
	status := "active"
	tags := "Golang,Backend,Databases" // Will be scanned as *string
	var telegramChatID int64 = 123456789
	calendarURL := "https://calendly.com/ivan"
	sortOrder := 1
	createdAt := time.Now().AddDate(0, 0, -7) // 7 days ago (should be IsNew)
	updatedAt := time.Now().AddDate(0, 0, -1)
	menteeCount := 12
	openmentorSlug := "ivan-ivanov"

	// Create mock row.
	// Column order must match the SELECT list used by FetchAllMentorsFromDB,
	// FetchSingleMentorFromDB and fetchMentorByUUIDFromDB.
	row := &mockRow{
		values: []interface{}{
			mentorID,
			airtableID, // Still in database schema, scanned but not stored in model
			legacyID,
			slug,
			name,
			job,
			workplace,
			about,
			description,
			competencies,
			experience,
			price,
			status,
			tags,           // Will be scanned as *string
			telegramChatID, // Will be scanned as *int64
			calendarURL,
			sortOrder,
			createdAt,
			updatedAt,
			menteeCount,
			openmentorSlug, // Will be scanned as *string
		},
	}

	// Scan the row
	mentor, err := models.ScanMentor(row)
	if err != nil {
		t.Fatalf("ScanMentor failed: %v", err)
	}

	// Verify fields
	if mentor.MentorID != mentorID {
		t.Errorf("expected MentorID %s, got %s", mentorID, mentor.MentorID)
	}

	if mentor.LegacyID != legacyID {
		t.Errorf("expected LegacyID %d, got %d", legacyID, mentor.LegacyID)
	}

	if mentor.Slug != slug {
		t.Errorf("expected Slug %s, got %s", slug, mentor.Slug)
	}

	if mentor.Name != name {
		t.Errorf("expected Name %s, got %s", name, mentor.Name)
	}

	if !mentor.UpdatedAt.Equal(updatedAt) {
		t.Errorf("expected UpdatedAt %v, got %v", updatedAt, mentor.UpdatedAt)
	}

	if mentor.MenteeCount != menteeCount {
		t.Errorf("expected MenteeCount %d, got %d", menteeCount, mentor.MenteeCount)
	}

	if mentor.OpenmentorSlug != openmentorSlug {
		t.Errorf("expected OpenmentorSlug %s, got %s", openmentorSlug, mentor.OpenmentorSlug)
	}

	// Verify computed IsVisible: status = 'active' AND telegram_chat_id IS NOT NULL
	if !mentor.IsVisible {
		t.Errorf("expected IsVisible to be true (status=active, telegram_chat_id set)")
	}

	// Verify computed IsNew: created_at > NOW() - 14 days (7 days ago should be new)
	if !mentor.IsNew {
		t.Errorf("expected IsNew to be true (created 7 days ago)")
	}

	// Verify tags parsing
	expectedTags := []string{"Golang", "Backend", "Databases"}
	if len(mentor.Tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(mentor.Tags))
	}
	for i, tag := range expectedTags {
		if i >= len(mentor.Tags) || mentor.Tags[i] != tag {
			t.Errorf("expected tag[%d] = %s, got %s", i, tag, mentor.Tags[i])
		}
	}

	// Verify calendar type
	if mentor.CalendarType != "calendly" {
		t.Errorf("expected CalendarType 'calendly', got %s", mentor.CalendarType)
	}
}

// TestScanMentor_InactiveMentor verifies IsVisible computation for inactive mentors
func TestScanMentor_InactiveMentor(t *testing.T) {
	mentorID := "550e8400-e29b-41d4-a716-446655440000"
	createdAt := time.Now().AddDate(0, 0, -20) // 20 days ago (should NOT be IsNew)

	row := &mockRow{
		values: []interface{}{
			mentorID,      // mentor_id
			nil,           // airtable_id (null)
			1,             // legacy_id
			"test",        // slug
			"Test",        // name
			"Engineer",    // job
			"Company",     // workplace
			"About",       // about
			"Description", // description
			"Skills",      // competencies
			"0-2",         // experience
			"free",        // price
			"inactive",    // status (inactive)
			nil,           // tags (null)
			nil,           // telegram_chat_id (null)
			"",            // calendar_url
			0,             // sort_order
			createdAt,     // created_at
			createdAt,     // updated_at
			0,             // mentee_count
			nil,           // openmentor_slug (null - no openmentor.io profile)
		},
	}

	mentor, err := models.ScanMentor(row)
	if err != nil {
		t.Fatalf("ScanMentor failed: %v", err)
	}

	// IsVisible should be false (status != active)
	if mentor.IsVisible {
		t.Errorf("expected IsVisible to be false for inactive mentor")
	}

	// IsNew should be false (created 20 days ago)
	if mentor.IsNew {
		t.Errorf("expected IsNew to be false for mentor created 20 days ago")
	}

	// OpenmentorSlug should stay empty when the mapping row is missing
	if mentor.OpenmentorSlug != "" {
		t.Errorf("expected empty OpenmentorSlug, got %s", mentor.OpenmentorSlug)
	}
}

// mentorRowValues builds a full mentor row in the exact column order produced by the
// mentor SELECTs, with the trailing openmentor_slug column parameterized.
func mentorRowValues(openmentorSlug interface{}) []interface{} {
	return []interface{}{
		"550e8400-e29b-41d4-a716-446655440000", // mentor_id
		nil,                                    // airtable_id
		1,                                      // legacy_id
		"test-mentor-1",                        // slug
		"Test Mentor",                          // name
		"Engineer",                             // job_title
		"Company",                              // workplace
		"About",                                // about
		"Description",                          // details
		"Skills",                               // competencies
		"3-5",                                  // experience
		"free",                                 // price
		"active",                               // status
		"Golang",                               // tags
		int64(123456789),                       // telegram_chat_id
		"https://calendly.com/test",            // calendar_url
		0,                                      // sort_order
		time.Now().AddDate(0, 0, -1),           // created_at
		time.Now(),                             // updated_at
		0,                                      // mentee_count
		openmentorSlug,                         // openmentor_slug
	}
}

// TestScanMentor_OpenmentorSlug verifies the openmentor.io cross-link slug is scanned
// from the last SELECT column and defaults to empty when no mapping exists.
func TestScanMentor_OpenmentorSlug(t *testing.T) {
	tests := []struct {
		name           string
		openmentorSlug interface{}
		expected       string
	}{
		{
			name:           "mapped mentor exposes openmentor slug",
			openmentorSlug: "ivan-ivanov",
			expected:       "ivan-ivanov",
		},
		{
			name:           "unmapped mentor coalesced to empty string",
			openmentorSlug: "",
			expected:       "",
		},
		{
			name:           "null openmentor slug treated as empty",
			openmentorSlug: nil,
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentor, err := models.ScanMentor(&mockRow{values: mentorRowValues(tt.openmentorSlug)})
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, mentor.OpenmentorSlug)
		})
	}
}

// TestScanMentor_OpenmentorSlugJSON verifies the cross-link field is present in the API
// response only when a mapping exists (json tag uses omitempty).
func TestScanMentor_OpenmentorSlugJSON(t *testing.T) {
	tests := []struct {
		name           string
		openmentorSlug interface{}
		shouldContain  bool
	}{
		{
			name:           "mapped mentor serializes openmentorSlug",
			openmentorSlug: "ivan-ivanov",
			shouldContain:  true,
		},
		{
			name:           "unmapped mentor omits openmentorSlug",
			openmentorSlug: nil,
			shouldContain:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentor, err := models.ScanMentor(&mockRow{values: mentorRowValues(tt.openmentorSlug)})
			assert.NoError(t, err)

			payload, err := json.Marshal(mentor)
			assert.NoError(t, err)

			assert.Equal(t, tt.shouldContain, strings.Contains(string(payload), `"openmentorSlug"`))
		})
	}
}

// TestScanMentor_Error verifies error handling
func TestScanMentor_Error(t *testing.T) {
	row := &mockRow{
		err: pgx.ErrNoRows,
	}

	_, err := models.ScanMentor(row)
	if err != pgx.ErrNoRows {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}
}
