package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Client wraps a pgx connection pool with observability
type Client struct {
	pool *pgxpool.Pool
}

// Config holds PostgreSQL connection configuration
type Config struct {
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	SSLMode      string
	MaxConns     int32
	MinConns     int32
	MaxConnLife  time.Duration
	MaxConnIdle  time.Duration
	HealthPeriod time.Duration
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() Config {
	return Config{
		Host:         "localhost",
		Port:         5432,
		Database:     "getmentor",
		User:         "getmentor",
		SSLMode:      "prefer",
		MaxConns:     10,
		MinConns:     2,
		MaxConnLife:  time.Hour,
		MaxConnIdle:  30 * time.Minute,
		HealthPeriod: time.Minute,
	}
}

// ConnectionString builds a PostgreSQL connection string from config
func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.Database, c.User, c.Password, c.SSLMode,
	)
}

// NewClient creates a new PostgreSQL client with connection pooling
func NewClient(ctx context.Context, cfg *Config) (*Client, error) {
	connString := cfg.ConnectionString()

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLife
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdle
	poolConfig.HealthCheckPeriod = cfg.HealthPeriod

	// Create the pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("PostgreSQL client initialized",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
		zap.Int32("max_conns", cfg.MaxConns),
	)

	return &Client{pool: pool}, nil
}

// Close closes the connection pool
func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
		logger.Info("PostgreSQL connection pool closed")
	}
}

// Pool returns the underlying connection pool for advanced usage
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

// Ping checks if the database connection is alive
func (c *Client) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (c *Client) Stats() *pgxpool.Stat {
	return c.pool.Stat()
}

// recordMetrics records database operation metrics
func recordMetrics(operation, status string, duration float64) {
	metrics.AirtableRequestDuration.WithLabelValues("postgres_"+operation, status).Observe(duration)
	metrics.AirtableRequestTotal.WithLabelValues("postgres_"+operation, status).Inc()
}
