package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "development environment",
			config: &Config{
				Server: ServerConfig{AppEnv: "development"},
			},
			expected: true,
		},
		{
			name: "debug gin mode",
			config: &Config{
				Server: ServerConfig{GinMode: "debug"},
			},
			expected: true,
		},
		{
			name: "production environment",
			config: &Config{
				Server: ServerConfig{AppEnv: "production"},
			},
			expected: false,
		},
		{
			name: "release mode",
			config: &Config{
				Server: ServerConfig{GinMode: "release", AppEnv: "production"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsDevelopment()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "production environment",
			config: &Config{
				Server: ServerConfig{AppEnv: "production"},
			},
			expected: true,
		},
		{
			name: "development environment",
			config: &Config{
				Server: ServerConfig{AppEnv: "development"},
			},
			expected: false,
		},
		{
			name: "staging environment",
			config: &Config{
				Server: ServerConfig{AppEnv: "staging"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsProduction()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid offline config",
			config: &Config{
				Airtable: AirtableConfig{
					WorkOffline: true,
				},
				Auth: AuthConfig{
					InternalMentorsAPI: "test-token",
				},
			},
			expectError: false,
		},
		{
			name: "valid online config",
			config: &Config{
				Airtable: AirtableConfig{
					WorkOffline: false,
					APIKey:      "test-key",
					BaseID:      "test-base",
				},
				Auth: AuthConfig{
					InternalMentorsAPI: "test-token",
				},
			},
			expectError: false,
		},
		{
			name: "missing airtable API key",
			config: &Config{
				Airtable: AirtableConfig{
					WorkOffline: false,
					BaseID:      "test-base",
				},
				Auth: AuthConfig{
					InternalMentorsAPI: "test-token",
				},
			},
			expectError: true,
			errorMsg:    "AIRTABLE_API_KEY is required",
		},
		{
			name: "missing airtable base ID",
			config: &Config{
				Airtable: AirtableConfig{
					WorkOffline: false,
					APIKey:      "test-key",
				},
				Auth: AuthConfig{
					InternalMentorsAPI: "test-token",
				},
			},
			expectError: true,
			errorMsg:    "AIRTABLE_BASE_ID is required",
		},
		{
			name: "missing internal API token",
			config: &Config{
				Airtable: AirtableConfig{
					WorkOffline: true,
				},
				Auth: AuthConfig{},
			},
			expectError: true,
			errorMsg:    "INTERNAL_MENTORS_API is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	// Clean environment
	os.Clearenv()

	// Set only required fields
	os.Setenv("AIRTABLE_WORK_OFFLINE", "true")
	os.Setenv("INTERNAL_MENTORS_API", "test-token")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check defaults
	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "release", cfg.Server.GinMode)
	assert.Equal(t, "production", cfg.Server.AppEnv)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "/app/logs", cfg.Logging.Dir)
	assert.Equal(t, "http://localhost:3000", cfg.NextJS.BaseURL)
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Clean environment
	os.Clearenv()

	// Set environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("GIN_MODE", "debug")
	os.Setenv("APP_ENV", "development")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("AIRTABLE_WORK_OFFLINE", "false")
	os.Setenv("AIRTABLE_API_KEY", "test-key-123")
	os.Setenv("AIRTABLE_BASE_ID", "test-base-456")
	os.Setenv("INTERNAL_MENTORS_API", "internal-token-789")
	os.Setenv("MENTORS_API_LIST_AUTH_TOKEN", "token1")
	os.Setenv("MENTORS_API_LIST_AUTH_TOKEN_INNO", "token2")
	os.Setenv("MENTORS_API_LIST_AUTH_TOKEN_AIKB", "token3")
	os.Setenv("RECAPTCHA_V2_SECRET_KEY", "recaptcha-secret")
	os.Setenv("NEXTJS_BASE_URL", "https://example.com")

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify values from environment
	assert.Equal(t, "9000", cfg.Server.Port)
	assert.Equal(t, "debug", cfg.Server.GinMode)
	assert.Equal(t, "development", cfg.Server.AppEnv)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "test-key-123", cfg.Airtable.APIKey)
	assert.Equal(t, "test-base-456", cfg.Airtable.BaseID)
	assert.False(t, cfg.Airtable.WorkOffline)
	assert.Equal(t, "internal-token-789", cfg.Auth.InternalMentorsAPI)
	assert.Equal(t, "token1", cfg.Auth.MentorsAPIToken)
	assert.Equal(t, "token2", cfg.Auth.MentorsAPITokenInno)
	assert.Equal(t, "token3", cfg.Auth.MentorsAPITokenAIKB)
	assert.Equal(t, "recaptcha-secret", cfg.ReCAPTCHA.SecretKey)
	assert.Equal(t, "https://example.com", cfg.NextJS.BaseURL)
}

func TestLoad_ValidationFailure(t *testing.T) {
	// Save current directory and change to a temp directory without .env file
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	tempDir := t.TempDir()
	os.Chdir(tempDir)

	// Clean environment - missing required fields
	os.Clearenv()
	os.Setenv("AIRTABLE_WORK_OFFLINE", "false")
	// Missing AIRTABLE_API_KEY, AIRTABLE_BASE_ID, and INTERNAL_MENTORS_API

	cfg, err := Load()

	assert.Error(t, err)
	assert.Nil(t, cfg)
}
