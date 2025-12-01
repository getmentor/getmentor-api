package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

// Config holds retry configuration
type Config struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the factor by which delay increases
	Multiplier float64
	// Jitter adds randomness to delays to prevent thundering herd
	Jitter bool
	// RetryableErrors is a function to determine if an error should be retried
	RetryableErrors func(error) bool
}

// DefaultConfig returns sensible retry defaults
func DefaultConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		RetryableErrors: func(err error) bool {
			// By default, retry all errors
			return true
		},
	}
}

// AirtableConfig returns retry config optimized for Airtable API
func AirtableConfig() Config {
	config := DefaultConfig()
	config.MaxRetries = 3
	config.InitialDelay = 500 * time.Millisecond
	config.MaxDelay = 10 * time.Second
	return config
}

// AzureConfig returns retry config optimized for Azure Storage
func AzureConfig() Config {
	config := DefaultConfig()
	config.MaxRetries = 3
	config.InitialDelay = 200 * time.Millisecond
	config.MaxDelay = 3 * time.Second
	return config
}

// Do executes the function with retry logic
func Do(ctx context.Context, config Config, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := fn()
		if err == nil {
			// Success
			if attempt > 0 {
				logger.Info("Operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt))
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			logger.Warn("Non-retryable error encountered",
				zap.String("operation", operation),
				zap.Error(err))
			return err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, config)

		logger.Warn("Operation failed, retrying",
			zap.String("operation", operation),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", config.MaxRetries),
			zap.Duration("delay", delay),
			zap.Error(err))

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	logger.Error("Operation failed after all retries",
		zap.String("operation", operation),
		zap.Int("max_retries", config.MaxRetries),
		zap.Error(lastErr))

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// DoWithResult executes the function with retry logic and returns a result
func DoWithResult[T any](ctx context.Context, config Config, operation string, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Execute the operation
		res, err := fn()
		if err == nil {
			// Success
			if attempt > 0 {
				logger.Info("Operation succeeded after retry",
					zap.String("operation", operation),
					zap.Int("attempt", attempt))
			}
			return res, nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			logger.Warn("Non-retryable error encountered",
				zap.String("operation", operation),
				zap.Error(err))
			return result, err
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, config)

		logger.Warn("Operation failed, retrying",
			zap.String("operation", operation),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", config.MaxRetries),
			zap.Duration("delay", delay),
			zap.Error(err))

		// Wait before retrying
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}
	}

	logger.Error("Operation failed after all retries",
		zap.String("operation", operation),
		zap.Int("max_retries", config.MaxRetries),
		zap.Error(lastErr))

	return result, fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

// calculateDelay calculates the delay for the next retry using exponential backoff
func calculateDelay(attempt int, config Config) time.Duration {
	// Calculate exponential delay: initialDelay * (multiplier ^ attempt)
	delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter if enabled (±25% randomness)
	if config.Jitter {
		jitterRange := delay * 0.25
		//nolint:gosec // G404: math/rand is sufficient for retry jitter, crypto/rand not needed
		jitter := (rand.Float64() * 2 * jitterRange) - jitterRange
		delay += jitter
	}

	return time.Duration(delay)
}

// IsRetryable is a helper to check common retryable conditions
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Add common retryable error patterns here
	// For now, retry all errors by default
	// In production, you'd check for specific error types:
	// - Network timeouts
	// - 5xx HTTP errors
	// - Temporary DNS failures
	// - Connection refused

	return true
}
