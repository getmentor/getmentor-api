package circuitbreaker

import (
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// Config holds circuit breaker configuration
type Config struct {
	Name          string
	MaxRequests   uint32        // Max requests allowed in half-open state
	Interval      time.Duration // Interval for resetting failure counts
	Timeout       time.Duration // Duration of open state before trying again
	ReadyToTrip   func(counts gobreaker.Counts) bool
	OnStateChange func(name string, from gobreaker.State, to gobreaker.State)
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig(name string) Config {
	return Config{
		Name:        name,
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("Circuit breaker state changed",
				zap.String("breaker", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	}
}

// NewCircuitBreaker creates a new circuit breaker with the given config
func NewCircuitBreaker(cfg Config) *gobreaker.CircuitBreaker {
	settings := gobreaker.Settings{
		Name:          cfg.Name,
		MaxRequests:   cfg.MaxRequests,
		Interval:      cfg.Interval,
		Timeout:       cfg.Timeout,
		ReadyToTrip:   cfg.ReadyToTrip,
		OnStateChange: cfg.OnStateChange,
	}

	return gobreaker.NewCircuitBreaker(settings)
}

// Execute wraps a function call with circuit breaker logic
func Execute[T any](cb *gobreaker.CircuitBreaker, fn func() (T, error)) (T, error) {
	result, err := cb.Execute(func() (interface{}, error) {
		return fn()
	})

	if err != nil {
		var zero T
		return zero, err
	}

	typedResult, ok := result.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("type assertion failed in circuit breaker")
	}

	return typedResult, nil
}

// ExecuteWithFallback executes a function with circuit breaker and fallback
func ExecuteWithFallback[T any](cb *gobreaker.CircuitBreaker, fn func() (T, error), fallback func() (T, error)) (T, error) {
	result, err := Execute(cb, fn)

	if err != nil {
		if err == gobreaker.ErrOpenState {
			logger.Warn("Circuit breaker open, using fallback",
				zap.String("breaker", cb.Name()))
			return fallback()
		}
		return result, err
	}

	return result, nil
}

// IsCircuitOpen checks if the circuit breaker is in open state
func IsCircuitOpen(cb *gobreaker.CircuitBreaker) bool {
	return cb.State() == gobreaker.StateOpen
}

// GetState returns the current state of the circuit breaker
func GetState(cb *gobreaker.CircuitBreaker) string {
	return cb.State().String()
}

// FormatError wraps the error with circuit breaker information
func FormatError(breakerName string, err error) error {
	if err == gobreaker.ErrOpenState {
		return fmt.Errorf("circuit breaker '%s' is open: %w", breakerName, err)
	}
	if err == gobreaker.ErrTooManyRequests {
		return fmt.Errorf("circuit breaker '%s' has too many requests: %w", breakerName, err)
	}
	return err
}
