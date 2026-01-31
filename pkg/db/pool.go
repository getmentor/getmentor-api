package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a new PostgreSQL connection pool with sensible defaults
// Parameters:
//   - ctx: Context for the connection
//   - databaseURL: PostgreSQL connection string
//
// Returns:
//   - *pgxpool.Pool: Configured connection pool
//   - error: Error if pool creation fails
//
// Connection pool configuration:
//   - MaxConns: 10 (maximum number of connections)
//   - MinConns: 2 (minimum number of idle connections)
//   - HealthCheckPeriod: 30s (how often to check connection health)
//   - MaxConnLifetime: 1h (maximum lifetime of a connection)
//   - MaxConnIdleTime: 30m (maximum idle time before closing)
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// Parse connection string and configure pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.HealthCheckPeriod = 30 * time.Second
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Create pool with config
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection by pinging database
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// Close gracefully closes the connection pool
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}
