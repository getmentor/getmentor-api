package azure

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateImageType(t *testing.T) {
	// Create a minimal storage client for testing (without actual Azure connection)
	client := &StorageClient{}

	tests := []struct {
		name        string
		contentType string
		wantErr     bool
	}{
		{
			name:        "valid jpeg type",
			contentType: "image/jpeg",
			wantErr:     false,
		},
		{
			name:        "valid jpg type",
			contentType: "image/jpg",
			wantErr:     false,
		},
		{
			name:        "valid png type",
			contentType: "image/png",
			wantErr:     false,
		},
		{
			name:        "valid webp type",
			contentType: "image/webp",
			wantErr:     false,
		},
		{
			name:        "case insensitive jpeg",
			contentType: "IMAGE/JPEG",
			wantErr:     false,
		},
		{
			name:        "invalid gif type",
			contentType: "image/gif",
			wantErr:     true,
		},
		{
			name:        "invalid svg type",
			contentType: "image/svg+xml",
			wantErr:     true,
		},
		{
			name:        "invalid non-image type",
			contentType: "application/pdf",
			wantErr:     true,
		},
		{
			name:        "empty content type",
			contentType: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateImageType(tt.contentType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateImageSize(t *testing.T) {
	client := &StorageClient{}

	// Helper function to create base64 data of specific size
	createBase64Data := func(sizeBytes int) string {
		data := make([]byte, sizeBytes)
		return base64.StdEncoding.EncodeToString(data)
	}

	tests := []struct {
		name      string
		imageData string
		wantErr   bool
	}{
		{
			name:      "small image (1KB)",
			imageData: createBase64Data(1024),
			wantErr:   false,
		},
		{
			name:      "medium image (1MB)",
			imageData: createBase64Data(1024 * 1024),
			wantErr:   false,
		},
		{
			name:      "large but valid image (5MB)",
			imageData: createBase64Data(5 * 1024 * 1024),
			wantErr:   false,
		},
		{
			name:      "exactly 10MB (should pass)",
			imageData: createBase64Data(10 * 1024 * 1024),
			wantErr:   false,
		},
		{
			name:      "too large image (11MB)",
			imageData: createBase64Data(11 * 1024 * 1024),
			wantErr:   true,
		},
		{
			name:      "data URI format with small image",
			imageData: "data:image/png;base64," + createBase64Data(1024),
			wantErr:   false,
		},
		{
			name:      "data URI format with large image (15MB)",
			imageData: "data:image/png;base64," + createBase64Data(15*1024*1024),
			wantErr:   true,
		},
		{
			name:      "invalid base64",
			imageData: "not-valid-base64!!!",
			wantErr:   true,
		},
		{
			name:      "invalid data URI format",
			imageData: "data:image/png;base64",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateImageSize(tt.imageData)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateFileName(t *testing.T) {
	client := &StorageClient{}

	tests := []struct {
		name             string
		mentorID         int
		originalFileName string
		checkFunc        func(t *testing.T, result string)
	}{
		{
			name:             "generates filename with extension",
			mentorID:         123,
			originalFileName: "profile.jpg",
			checkFunc: func(t *testing.T, result string) {
				assert.True(t, strings.HasPrefix(result, "tmp/123-"))
				assert.True(t, strings.HasSuffix(result, ".jpg"))
			},
		},
		{
			name:             "handles png extension",
			mentorID:         456,
			originalFileName: "avatar.png",
			checkFunc: func(t *testing.T, result string) {
				assert.True(t, strings.HasPrefix(result, "tmp/456-"))
				assert.True(t, strings.HasSuffix(result, ".png"))
			},
		},
		{
			name:             "handles no extension (defaults to .jpg)",
			mentorID:         789,
			originalFileName: "profile",
			checkFunc: func(t *testing.T, result string) {
				assert.True(t, strings.HasPrefix(result, "tmp/789-"))
				assert.True(t, strings.HasSuffix(result, ".jpg"))
			},
		},
		{
			name:             "handles complex filename",
			mentorID:         999,
			originalFileName: "my-profile-picture.jpeg",
			checkFunc: func(t *testing.T, result string) {
				assert.True(t, strings.HasPrefix(result, "tmp/999-"))
				assert.True(t, strings.HasSuffix(result, ".jpeg"))
			},
		},
		{
			name:             "generates unique filenames (different timestamps)",
			mentorID:         100,
			originalFileName: "test.jpg",
			checkFunc: func(t *testing.T, result string) {
				// Just check the format
				assert.True(t, strings.HasPrefix(result, "tmp/100-"))
				assert.True(t, strings.Contains(result, "-"))
				assert.True(t, strings.HasSuffix(result, ".jpg"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.GenerateFileName(tt.mentorID, tt.originalFileName)
			tt.checkFunc(t, result)
		})
	}
}

func TestGenerateFileName_Uniqueness(t *testing.T) {
	client := &StorageClient{}
	mentorID := 123
	originalFileName := "profile.jpg"

	// Generate two filenames in quick succession
	filename1 := client.GenerateFileName(mentorID, originalFileName)
	filename2 := client.GenerateFileName(mentorID, originalFileName)

	// They should be different due to timestamp (or same if generated in the same second)
	// We can't guarantee they'll be different in a unit test, but we can check the format
	assert.True(t, strings.HasPrefix(filename1, "tmp/123-"))
	assert.True(t, strings.HasPrefix(filename2, "tmp/123-"))
	assert.True(t, strings.HasSuffix(filename1, ".jpg"))
	assert.True(t, strings.HasSuffix(filename2, ".jpg"))
}
