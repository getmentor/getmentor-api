package config_test

import (
	"os"
	"testing"

	"github.com/getmentor/getmentor-api/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name: "development environment",
			cfg: &config.Config{
				Server: config.ServerConfig{AppEnv: "development"},
			},
			expected: true,
		},
		{
			name: "debug gin mode",
			cfg: &config.Config{
				Server: config.ServerConfig{GinMode: "debug"},
			},
			expected: true,
		},
		{
			name: "production environment",
			cfg: &config.Config{
				Server: config.ServerConfig{AppEnv: "production"},
			},
			expected: false,
		},
		{
			name: "release mode",
			cfg: &config.Config{
				Server: config.ServerConfig{GinMode: "release", AppEnv: "production"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.IsDevelopment()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name: "production environment",
			cfg: &config.Config{
				Server: config.ServerConfig{AppEnv: "production"},
			},
			expected: true,
		},
		{
			name: "development environment",
			cfg: &config.Config{
				Server: config.ServerConfig{AppEnv: "development"},
			},
			expected: false,
		},
		{
			name: "staging environment",
			cfg: &config.Config{
				Server: config.ServerConfig{AppEnv: "staging"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.IsProduction()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid offline config",
			cfg: &config.Config{
				Airtable: config.AirtableConfig{
					WorkOffline: true,
				},
				Auth: config.AuthConfig{
					InternalMentorsAPI: "test-token",
					MCPAuthToken:       "test-mcp-token",
				},
			},
			expectError: false,
		},
		{
			name: "valid online config",
			cfg: &config.Config{
				Airtable: config.AirtableConfig{
					WorkOffline: false,
					APIKey:      "test-key",
					BaseID:      "test-base",
				},
				Auth: config.AuthConfig{
					InternalMentorsAPI: "test-token",
					MCPAuthToken:       "test-mcp-token",
				},
			},
			expectError: false,
		},
		{
			name: "missing airtable API key",
			cfg: &config.Config{
				Airtable: config.AirtableConfig{
					WorkOffline: false,
					BaseID:      "test-base",
				},
				Auth: config.AuthConfig{
					InternalMentorsAPI: "test-token",
					MCPAuthToken:       "test-mcp-token",
				},
			},
			expectError: true,
			errorMsg:    "AIRTABLE_API_KEY is required",
		},
		{
			name: "missing airtable base ID",
			cfg: &config.Config{
				Airtable: config.AirtableConfig{
					WorkOffline: false,
					APIKey:      "test-key",
				},
				Auth: config.AuthConfig{
					InternalMentorsAPI: "test-token",
					MCPAuthToken:       "test-mcp-token",
				},
			},
			expectError: true,
			errorMsg:    "AIRTABLE_BASE_ID is required",
		},
		{
			name: "missing internal API token",
			cfg: &config.Config{
				Airtable: config.AirtableConfig{
					WorkOffline: true,
				},
				Auth: config.AuthConfig{
					MCPAuthToken: "test-mcp-token",
				},
			},
			expectError: true,
			errorMsg:    "INTERNAL_MENTORS_API is required",
		},
		{
			name: "missing MCP auth token",
			cfg: &config.Config{
				Airtable: config.AirtableConfig{
					WorkOffline: true,
				},
				Auth: config.AuthConfig{
					InternalMentorsAPI: "test-token",
				},
			},
			expectError: true,
			errorMsg:    "MCP_AUTH_TOKEN is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
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
	_ = os.Setenv("AIRTABLE_WORK_OFFLINE", "true")
	_ = os.Setenv("INTERNAL_MENTORS_API", "test-token")
	_ = os.Setenv("MCP_AUTH_TOKEN", "test-mcp-token")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Check defaults
	assert.Equal(t, "8081", cfg.Server.Port)
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
	_ = os.Setenv("PORT", "9000")
	_ = os.Setenv("GIN_MODE", "debug")
	_ = os.Setenv("APP_ENV", "development")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("AIRTABLE_WORK_OFFLINE", "false")
	_ = os.Setenv("AIRTABLE_API_KEY", "test-key-123")
	_ = os.Setenv("AIRTABLE_BASE_ID", "test-base-456")
	_ = os.Setenv("INTERNAL_MENTORS_API", "internal-token-789")
	_ = os.Setenv("MCP_AUTH_TOKEN", "mcp-token-xyz")
	_ = os.Setenv("MENTORS_API_LIST_AUTH_TOKEN", "token1")
	_ = os.Setenv("MENTORS_API_LIST_AUTH_TOKEN_INNO", "token2")
	_ = os.Setenv("MENTORS_API_LIST_AUTH_TOKEN_AIKB", "token3")
	_ = os.Setenv("RECAPTCHA_V2_SECRET_KEY", "recaptcha-secret")
	_ = os.Setenv("NEXTJS_BASE_URL", "https://example.com")

	cfg, err := config.Load()

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
	assert.Equal(t, "mcp-token-xyz", cfg.Auth.MCPAuthToken)
	assert.Equal(t, "token1", cfg.Auth.MentorsAPIToken)
	assert.Equal(t, "token2", cfg.Auth.MentorsAPITokenInno)
	assert.Equal(t, "token3", cfg.Auth.MentorsAPITokenAIKB)
	assert.Equal(t, "recaptcha-secret", cfg.ReCAPTCHA.SecretKey)
	assert.Equal(t, "https://example.com", cfg.NextJS.BaseURL)
}

func TestLoad_ValidationFailure(t *testing.T) {
	// Save current directory and change to a temp directory without .env file
	originalDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(originalDir)
	}()

	tempDir := t.TempDir()
	_ = os.Chdir(tempDir)

	// Clean environment - missing required fields
	os.Clearenv()
	_ = os.Setenv("AIRTABLE_WORK_OFFLINE", "false")
	// Missing AIRTABLE_API_KEY, AIRTABLE_BASE_ID, and INTERNAL_MENTORS_API

	cfg, err := config.Load()

	assert.Error(t, err)
	assert.Nil(t, cfg)
}
