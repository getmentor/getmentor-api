package config_test

import (
	"os"
	"testing"

	"github.com/getmentor/getmentor-api/config"
	"github.com/stretchr/testify/assert"
)

func TestLoad_WithPostgresConfig(t *testing.T) {
	// Clean environment
	os.Clearenv()

	// Set required fields
	os.Setenv("AIRTABLE_WORK_OFFLINE", "true")
	os.Setenv("INTERNAL_MENTORS_API", "test-token")
	os.Setenv("MENTORS_API_LIST_AUTH_TOKEN", "public-token")
	os.Setenv("WEBHOOK_SECRET", "webhook-secret")
	os.Setenv("MCP_AUTH_TOKEN", "test-mcp-token")
	os.Setenv("RECAPTCHA_V2_SECRET_KEY", "recaptcha-secret")

	// Set PostgreSQL fields
	os.Setenv("POSTGRES_ENABLED", "true")
	os.Setenv("POSTGRES_HOST", "db.test.com")
	os.Setenv("POSTGRES_PORT", "5433")
	os.Setenv("POSTGRES_DB", "mentors_test")
	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("POSTGRES_PASSWORD", "testpass123")
	os.Setenv("POSTGRES_SSLMODE", "require")
	os.Setenv("BOT_API_KEY", "bot-test-key")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify PostgreSQL config
	assert.True(t, cfg.Postgres.Enabled)
	assert.Equal(t, "db.test.com", cfg.Postgres.Host)
	assert.Equal(t, 5433, cfg.Postgres.Port)
	assert.Equal(t, "mentors_test", cfg.Postgres.Database)
	assert.Equal(t, "testuser", cfg.Postgres.User)
	assert.Equal(t, "testpass123", cfg.Postgres.Password)
	assert.Equal(t, "require", cfg.Postgres.SSLMode)
	assert.Equal(t, "bot-test-key", cfg.Auth.BotAPIKey)
}

func TestLoad_PostgresDefaults(t *testing.T) {
	// Clean environment
	os.Clearenv()

	// Set only required fields (no PostgreSQL env vars)
	os.Setenv("AIRTABLE_WORK_OFFLINE", "true")
	os.Setenv("INTERNAL_MENTORS_API", "test-token")
	os.Setenv("MENTORS_API_LIST_AUTH_TOKEN", "public-token")
	os.Setenv("WEBHOOK_SECRET", "webhook-secret")
	os.Setenv("MCP_AUTH_TOKEN", "test-mcp-token")
	os.Setenv("RECAPTCHA_V2_SECRET_KEY", "recaptcha-secret")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify PostgreSQL defaults
	assert.False(t, cfg.Postgres.Enabled)
	assert.Equal(t, "localhost", cfg.Postgres.Host)
	assert.Equal(t, 5432, cfg.Postgres.Port)
	assert.Equal(t, "getmentor", cfg.Postgres.Database)
	assert.Equal(t, "getmentor", cfg.Postgres.User)
	assert.Equal(t, "prefer", cfg.Postgres.SSLMode)
	assert.Equal(t, 10, cfg.Postgres.MaxConns)
	assert.Equal(t, 2, cfg.Postgres.MinConns)
}

func TestPostgresConfig_ConnectionPoolSettings(t *testing.T) {
	// Clean environment
	os.Clearenv()

	// Set required fields
	os.Setenv("AIRTABLE_WORK_OFFLINE", "true")
	os.Setenv("INTERNAL_MENTORS_API", "test-token")
	os.Setenv("MENTORS_API_LIST_AUTH_TOKEN", "public-token")
	os.Setenv("WEBHOOK_SECRET", "webhook-secret")
	os.Setenv("MCP_AUTH_TOKEN", "test-mcp-token")
	os.Setenv("RECAPTCHA_V2_SECRET_KEY", "recaptcha-secret")

	// Set PostgreSQL connection pool settings
	os.Setenv("POSTGRES_ENABLED", "true")
	os.Setenv("POSTGRES_MAX_CONNS", "25")
	os.Setenv("POSTGRES_MIN_CONNS", "5")
	os.Setenv("POSTGRES_MAX_CONN_LIFE", "3600")
	os.Setenv("POSTGRES_MAX_CONN_IDLE", "600")
	os.Setenv("POSTGRES_HEALTH_PERIOD", "30")

	cfg, err := config.Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify connection pool settings
	assert.Equal(t, 25, cfg.Postgres.MaxConns)
	assert.Equal(t, 5, cfg.Postgres.MinConns)
	assert.Equal(t, 3600, cfg.Postgres.MaxConnLife)
	assert.Equal(t, 600, cfg.Postgres.MaxConnIdle)
	assert.Equal(t, 30, cfg.Postgres.HealthPeriod)
}
