package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	Airtable  AirtableConfig
	Azure     AzureConfig
	Auth      AuthConfig
	ReCAPTCHA ReCAPTCHAConfig
	NextJS    NextJSConfig
	Grafana   GrafanaConfig
	Logging   LoggingConfig
}

type ServerConfig struct {
	Port    string
	GinMode string
	AppEnv  string
}

type AirtableConfig struct {
	APIKey      string
	BaseID      string
	WorkOffline bool
}

type AzureConfig struct {
	ConnectionString string
	ContainerName    string
	StorageDomain    string
}

type AuthConfig struct {
	MentorsAPIToken      string
	MentorsAPITokenInno  string
	MentorsAPITokenAIKB  string
	InternalMentorsAPI   string
	RevalidateSecret     string
	WebhookSecret        string
}

type ReCAPTCHAConfig struct {
	SecretKey string
	SiteKey   string
}

type NextJSConfig struct {
	BaseURL           string
	RevalidateSecret  string
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

// Load reads configuration from environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("PORT", "8080")
	v.SetDefault("GIN_MODE", "release")
	v.SetDefault("APP_ENV", "production")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_DIR", "/app/logs")
	v.SetDefault("AIRTABLE_WORK_OFFLINE", false)
	v.SetDefault("NEXTJS_BASE_URL", "http://localhost:3000")

	// Automatically read environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read from .env file if it exists
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	v.AddConfigPath("..")
	_ = v.ReadInConfig() // Ignore error if file doesn't exist

	cfg := &Config{
		Server: ServerConfig{
			Port:    v.GetString("PORT"),
			GinMode: v.GetString("GIN_MODE"),
			AppEnv:  v.GetString("APP_ENV"),
		},
		Airtable: AirtableConfig{
			APIKey:      v.GetString("AIRTABLE_API_KEY"),
			BaseID:      v.GetString("AIRTABLE_BASE_ID"),
			WorkOffline: v.GetBool("AIRTABLE_WORK_OFFLINE"),
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
			RevalidateSecret:    v.GetString("REVALIDATE_SECRET_TOKEN"),
			WebhookSecret:       v.GetString("WEBHOOK_SECRET"),
		},
		ReCAPTCHA: ReCAPTCHAConfig{
			SecretKey: v.GetString("RECAPTCHA_V2_SECRET_KEY"),
			SiteKey:   v.GetString("NEXT_PUBLIC_RECAPTCHA_V2_SITE_KEY"),
		},
		NextJS: NextJSConfig{
			BaseURL:          v.GetString("NEXTJS_BASE_URL"),
			RevalidateSecret: v.GetString("NEXTJS_REVALIDATE_SECRET"),
		},
		Grafana: GrafanaConfig{
			MetricsURL:      v.GetString("GRAFANA_CLOUD_METRICS_URL"),
			MetricsUsername: v.GetString("GRAFANA_CLOUD_METRICS_USERNAME"),
			LogsURL:         v.GetString("GRAFANA_CLOUD_LOGS_URL"),
			LogsUsername:    v.GetString("GRAFANA_CLOUD_LOGS_USERNAME"),
			APIKey:          v.GetString("GRAFANA_CLOUD_API_KEY"),
		},
		Logging: LoggingConfig{
			Level: v.GetString("LOG_LEVEL"),
			Dir:   v.GetString("LOG_DIR"),
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
	if !c.Airtable.WorkOffline {
		if c.Airtable.APIKey == "" {
			return fmt.Errorf("AIRTABLE_API_KEY is required")
		}
		if c.Airtable.BaseID == "" {
			return fmt.Errorf("AIRTABLE_BASE_ID is required")
		}
	}

	if c.Auth.InternalMentorsAPI == "" {
		return fmt.Errorf("INTERNAL_MENTORS_API is required")
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
