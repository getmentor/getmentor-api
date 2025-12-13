package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration
//
//nolint:govet // Field alignment optimization would reduce readability
type Config struct {
	Server        ServerConfig
	Airtable      AirtableConfig
	Postgres      PostgresConfig
	Azure         AzureConfig
	Auth          AuthConfig
	ReCAPTCHA     ReCAPTCHAConfig
	NextJS        NextJSConfig
	Grafana       GrafanaConfig
	Logging       LoggingConfig
	Observability ObservabilityConfig
	Cache         CacheConfig
}

type ServerConfig struct {
	Port           string
	GinMode        string
	AppEnv         string
	BaseURL        string
	AllowedOrigins []string
}

type AirtableConfig struct {
	APIKey      string
	BaseID      string
	WorkOffline bool
}

type PostgresConfig struct {
	Enabled      bool
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	SSLMode      string
	MaxConns     int
	MinConns     int
	MaxConnLife  int // seconds
	MaxConnIdle  int // seconds
	HealthPeriod int // seconds
}

type AzureConfig struct {
	ConnectionString string
	ContainerName    string
	StorageDomain    string
}

type AuthConfig struct {
	MentorsAPIToken     string
	MentorsAPITokenInno string
	MentorsAPITokenAIKB string
	InternalMentorsAPI  string
	MCPAuthToken        string
	MCPAllowAll         bool
	RevalidateSecret    string
	WebhookSecret       string
	BotAPIKey           string
}

type ReCAPTCHAConfig struct {
	SecretKey string
	SiteKey   string
}

type NextJSConfig struct {
	BaseURL          string
	RevalidateSecret string
}

type GrafanaConfig struct {
	MetricsURL      string
	MetricsUsername string
	LogsURL         string
	LogsUsername    string
	APIKey          string
}

type LoggingConfig struct {
	Level string
	Dir   string
}

type ObservabilityConfig struct {
	AlloyEndpoint    string
	ServiceName      string
	ServiceNamespace string
	ServiceVersion   string
}

type CacheConfig struct {
	MentorTTLSeconds int // Mentor cache TTL in seconds
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("PORT", "8081")
	v.SetDefault("GIN_MODE", "release")
	v.SetDefault("APP_ENV", "production")
	v.SetDefault("BASE_URL", "https://getmentor.dev")
	v.SetDefault("ALLOWED_CORS_ORIGINS", "https://getmentor.dev,https://www.getmentor.dev")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_DIR", "/app/logs")
	v.SetDefault("AIRTABLE_WORK_OFFLINE", false)
	v.SetDefault("NEXTJS_BASE_URL", "http://localhost:3000")
	v.SetDefault("O11Y_EXPORTER_ENDPOINT", "alloy:4318") // OTLP over HTTP
	v.SetDefault("O11Y_BE_SERVICE_NAME", "getmentor-api")
	v.SetDefault("O11Y_SERVICE_NAMESPACE", "getmentor-dev")
	v.SetDefault("O11Y_BE_SERVICE_VERSION", "1.0.0")
	v.SetDefault("MENTOR_CACHE_TTL", 600) // 10 minutes in seconds
	v.SetDefault("MCP_ALLOW_ALL", false)

	// PostgreSQL defaults
	v.SetDefault("POSTGRES_ENABLED", false)
	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_DB", "getmentor")
	v.SetDefault("POSTGRES_USER", "getmentor")
	v.SetDefault("POSTGRES_SSLMODE", "prefer")
	v.SetDefault("POSTGRES_MAX_CONNS", 10)
	v.SetDefault("POSTGRES_MIN_CONNS", 2)
	v.SetDefault("POSTGRES_MAX_CONN_LIFE", 3600) // 1 hour
	v.SetDefault("POSTGRES_MAX_CONN_IDLE", 1800) // 30 minutes
	v.SetDefault("POSTGRES_HEALTH_PERIOD", 60)   // 1 minute

	// Automatically read environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read from .env file if it exists
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("..")
	_ = v.ReadInConfig() //nolint:errcheck // Ignore error if .env file doesn't exist

	// Parse allowed CORS origins (comma-separated)
	allowedOrigins := []string{}
	originsStr := v.GetString("ALLOWED_CORS_ORIGINS")
	if originsStr != "" {
		for _, origin := range strings.Split(originsStr, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:           v.GetString("PORT"),
			GinMode:        v.GetString("GIN_MODE"),
			AppEnv:         v.GetString("APP_ENV"),
			BaseURL:        v.GetString("BASE_URL"),
			AllowedOrigins: allowedOrigins,
		},
		Airtable: AirtableConfig{
			APIKey:      v.GetString("AIRTABLE_API_KEY"),
			BaseID:      v.GetString("AIRTABLE_BASE_ID"),
			WorkOffline: v.GetBool("AIRTABLE_WORK_OFFLINE"),
		},
		Postgres: PostgresConfig{
			Enabled:      v.GetBool("POSTGRES_ENABLED"),
			Host:         v.GetString("POSTGRES_HOST"),
			Port:         v.GetInt("POSTGRES_PORT"),
			Database:     v.GetString("POSTGRES_DB"),
			User:         v.GetString("POSTGRES_USER"),
			Password:     v.GetString("POSTGRES_PASSWORD"),
			SSLMode:      v.GetString("POSTGRES_SSLMODE"),
			MaxConns:     v.GetInt("POSTGRES_MAX_CONNS"),
			MinConns:     v.GetInt("POSTGRES_MIN_CONNS"),
			MaxConnLife:  v.GetInt("POSTGRES_MAX_CONN_LIFE"),
			MaxConnIdle:  v.GetInt("POSTGRES_MAX_CONN_IDLE"),
			HealthPeriod: v.GetInt("POSTGRES_HEALTH_PERIOD"),
		},
		Azure: AzureConfig{
			ConnectionString: v.GetString("AZURE_STORAGE_CONNECTION_STRING"),
			ContainerName:    v.GetString("AZURE_STORAGE_CONTAINER_NAME"),
			StorageDomain:    v.GetString("AZURE_STORAGE_DOMAIN"),
		},
		Auth: AuthConfig{
			MentorsAPIToken:     v.GetString("MENTORS_API_LIST_AUTH_TOKEN"),
			MentorsAPITokenInno: v.GetString("MENTORS_API_LIST_AUTH_TOKEN_INNO"),
			MentorsAPITokenAIKB: v.GetString("MENTORS_API_LIST_AUTH_TOKEN_AIKB"),
			InternalMentorsAPI:  v.GetString("INTERNAL_MENTORS_API"),
			MCPAuthToken:        v.GetString("MCP_AUTH_TOKEN"),
			MCPAllowAll:         v.GetBool("MCP_ALLOW_ALL"),
			RevalidateSecret:    v.GetString("REVALIDATE_SECRET_TOKEN"),
			WebhookSecret:       v.GetString("WEBHOOK_SECRET"),
			BotAPIKey:           v.GetString("BOT_API_KEY"),
		},
		ReCAPTCHA: ReCAPTCHAConfig{
			SecretKey: v.GetString("RECAPTCHA_V2_SECRET_KEY"),
			SiteKey:   v.GetString("NEXT_PUBLIC_RECAPTCHA_V2_SITE_KEY"),
		},
		NextJS: NextJSConfig{
			BaseURL:          v.GetString("NEXTJS_BASE_URL"),
			RevalidateSecret: v.GetString("NEXTJS_REVALIDATE_SECRET"),
		},
		Logging: LoggingConfig{
			Level: v.GetString("LOG_LEVEL"),
			Dir:   v.GetString("LOG_DIR"),
		},
		Observability: ObservabilityConfig{
			AlloyEndpoint:    v.GetString("O11Y_EXPORTER_ENDPOINT"),
			ServiceName:      v.GetString("O11Y_BE_SERVICE_NAME"),
			ServiceNamespace: v.GetString("O11Y_SERVICE_NAMESPACE"),
			ServiceVersion:   v.GetString("O11Y_BE_SERVICE_VERSION"),
		},
		Cache: CacheConfig{
			MentorTTLSeconds: v.GetInt("MENTOR_CACHE_TTL"),
		},
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if required configuration values are set
func (c *Config) Validate() error {
	// Airtable configuration
	if !c.Airtable.WorkOffline {
		if c.Airtable.APIKey == "" {
			return fmt.Errorf("AIRTABLE_API_KEY is required")
		}
		if c.Airtable.BaseID == "" {
			return fmt.Errorf("AIRTABLE_BASE_ID is required")
		}
	}

	// Authentication tokens
	if c.Auth.InternalMentorsAPI == "" {
		return fmt.Errorf("INTERNAL_MENTORS_API is required")
	}
	if c.Auth.MentorsAPIToken == "" {
		return fmt.Errorf("MENTORS_API_LIST_AUTH_TOKEN is required")
	}
	if c.Auth.WebhookSecret == "" {
		return fmt.Errorf("WEBHOOK_SECRET is required")
	}

	if c.Auth.MCPAuthToken == "" && !c.Auth.MCPAllowAll {
		return fmt.Errorf("MCP_AUTH_TOKEN is required")
	}

	// ReCAPTCHA configuration
	if c.ReCAPTCHA.SecretKey == "" {
		return fmt.Errorf("RECAPTCHA_V2_SECRET_KEY is required")
	}

	// Server configuration
	if c.Server.Port == "" {
		return fmt.Errorf("PORT is required")
	}
	if c.Server.BaseURL == "" {
		return fmt.Errorf("BASE_URL is required")
	}
	if len(c.Server.AllowedOrigins) == 0 {
		return fmt.Errorf("ALLOWED_CORS_ORIGINS is required")
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.AppEnv == "development" || c.Server.GinMode == "debug"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.AppEnv == "production"
}
