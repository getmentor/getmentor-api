package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// configureTLS sets up TLS configuration for Yandex Cloud Managed PostgreSQL
// Returns nil if TLS is not required (local development)
func configureTLS() (*tls.Config, error) {
	// Check if TLS is required via environment variable
	tlsServerName := os.Getenv("DATABASE_TLS_SERVER_NAME")
	if tlsServerName == "" {
		// TLS not configured - assume local development
		return nil, nil
	}

	// Load CA certificate from certs directory
	certPath := filepath.Join("certs", "yandex-ca.pem")
	caPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate from %s: %w", certPath, err)
	}

	// Create certificate pool and add CA cert
	rootCertPool := x509.NewCertPool()
	if ok := rootCertPool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	// Configure TLS with CA cert and server name verification
	return &tls.Config{
		RootCAs:    rootCertPool,
		ServerName: tlsServerName,
	}, nil
}

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
//
// TLS configuration:
//   - Set DATABASE_TLS_SERVER_NAME env var to enable TLS (e.g., "c-xxxxx.rw.mdb.yandexcloud.net")
//   - Reads CA certificate from certs/yandex-ca.pem
//   - If DATABASE_TLS_SERVER_NAME is not set, connects without TLS (local development)
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// Parse connection string and configure pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure TLS if required
	tlsConfig, err := configureTLS()
	if err != nil {
		return nil, fmt.Errorf("failed to configure TLS: %w", err)
	}
	if tlsConfig != nil {
		config.ConnConfig.TLSConfig = tlsConfig
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
