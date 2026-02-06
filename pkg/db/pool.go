package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// configureTLS sets up TLS configuration for Yandex Cloud Managed PostgreSQL
// Returns nil if TLS is not required (local development)
func configureTLS() (*tls.Config, error) {
	// Check if DATABASE_URL contains sslmode parameter to determine if TLS is needed
	// For local dev (localhost), typically no sslmode or sslmode=disable
	// For production, DATABASE_URL should include sslmode=verify-full or sslmode=require
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" || !containsSSLMode(databaseURL) {
		// No SSL configured - assume local development
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

	// Configure TLS with CA cert
	tlsConfig := &tls.Config{
		RootCAs: rootCertPool,
	}

	// Optional: Set ServerName if certificate name differs from connection hostname
	// Only needed if you get "certificate is valid for X, not Y" errors
	if serverName := os.Getenv("DATABASE_TLS_SERVER_NAME"); serverName != "" {
		tlsConfig.ServerName = serverName
	}

	return tlsConfig, nil
}

// containsSSLMode checks if DATABASE_URL has sslmode parameter
func containsSSLMode(url string) bool {
	return strings.Contains(url, "sslmode=require") ||
		strings.Contains(url, "sslmode=verify-full") ||
		strings.Contains(url, "sslmode=verify-ca")
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
//   - Automatically enabled if DATABASE_URL contains sslmode=verify-full or sslmode=require
//   - Reads CA certificate from certs/yandex-ca.pem
//   - DATABASE_TLS_SERVER_NAME is optional (only needed if cert name differs from hostname)
//   - Local development (localhost without sslmode) connects without TLS
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
