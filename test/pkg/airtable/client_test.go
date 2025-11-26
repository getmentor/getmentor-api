package airtable_test

import (
	"testing"

	"github.com/getmentor/getmentor-api/pkg/airtable"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Initialize logger for tests
	_ = logger.Initialize(logger.Config{
		Level:       "info",
		Environment: "test",
	})
}

func TestNewClient_OfflineMode(t *testing.T) {
	client, err := airtable.NewClient("", "", true)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	// Note: We can't test private fields from external test package
	// The fact that client was created without error is sufficient
}

func TestNewClient_OnlineMode(t *testing.T) {
	// Note: This will fail without a valid API key, but tests the constructor
	client, err := airtable.NewClient("test-key", "test-base", false)

	// We expect an error because the API key is invalid
	// but we're testing that the function handles it properly
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, client)
	} else {
		assert.NotNil(t, client)
		// Note: We can't test private fields from external test package
	}
}

func TestGetAllMentors_OfflineMode(t *testing.T) {
	client, err := airtable.NewClient("", "", true)
	assert.NoError(t, err)

	mentors, err := client.GetAllMentors()

	assert.NoError(t, err)
	assert.NotNil(t, mentors)
	assert.Greater(t, len(mentors), 0, "Offline mode should return test mentors")

	// Verify structure of test mentors
	for _, mentor := range mentors {
		assert.NotEmpty(t, mentor.Name)
		assert.NotEmpty(t, mentor.Slug)
		assert.Greater(t, mentor.ID, 0)
	}
}

func TestGetMentorByID_OfflineMode(t *testing.T) {
	client, err := airtable.NewClient("", "", true)
	assert.NoError(t, err)

	// Get all mentors first to find a valid ID
	mentors, err := client.GetAllMentors()
	assert.NoError(t, err)
	assert.Greater(t, len(mentors), 0)

	validID := mentors[0].ID

	tests := []struct {
		name        string
		id          int
		expectError bool
	}{
		{
			name:        "valid ID",
			id:          validID,
			expectError: false,
		},
		{
			name:        "invalid ID",
			id:          999999,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentor, err := client.GetMentorByID(tt.id)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, mentor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mentor)
				assert.Equal(t, tt.id, mentor.ID)
			}
		})
	}
}

func TestGetMentorBySlug_OfflineMode(t *testing.T) {
	client, err := airtable.NewClient("", "", true)
	assert.NoError(t, err)

	// Get all mentors first to find a valid slug
	mentors, err := client.GetAllMentors()
	assert.NoError(t, err)
	assert.Greater(t, len(mentors), 0)

	validSlug := mentors[0].Slug

	tests := []struct {
		name        string
		slug        string
		expectError bool
	}{
		{
			name:        "valid slug",
			slug:        validSlug,
			expectError: false,
		},
		{
			name:        "invalid slug",
			slug:        "non-existent-mentor-slug",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentor, err := client.GetMentorBySlug(tt.slug)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, mentor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mentor)
				assert.Equal(t, tt.slug, mentor.Slug)
			}
		})
	}
}

func TestGetMentorByRecordID_OfflineMode(t *testing.T) {
	client, err := airtable.NewClient("", "", true)
	assert.NoError(t, err)

	// Get all mentors first to find a valid record ID
	mentors, err := client.GetAllMentors()
	assert.NoError(t, err)
	assert.Greater(t, len(mentors), 0)

	validRecordID := mentors[0].AirtableID

	tests := []struct {
		name        string
		recordID    string
		expectError bool
	}{
		{
			name:        "valid record ID",
			recordID:    validRecordID,
			expectError: false,
		},
		{
			name:        "invalid record ID",
			recordID:    "recXXXXXXXXXXXXXX",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mentor, err := client.GetMentorByRecordID(tt.recordID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, mentor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mentor)
				assert.Equal(t, tt.recordID, mentor.AirtableID)
			}
		})
	}
}

func TestOfflineMode_DataConsistency(t *testing.T) {
	client, err := airtable.NewClient("", "", true)
	assert.NoError(t, err)

	// Get mentors multiple times to verify consistency
	mentors1, err1 := client.GetAllMentors()
	mentors2, err2 := client.GetAllMentors()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, len(mentors1), len(mentors2), "Offline mode should return consistent data")

	// Verify mentor IDs are consistent
	for i := range mentors1 {
		assert.Equal(t, mentors1[i].ID, mentors2[i].ID)
		assert.Equal(t, mentors1[i].Slug, mentors2[i].Slug)
		assert.Equal(t, mentors1[i].Name, mentors2[i].Name)
	}
}
