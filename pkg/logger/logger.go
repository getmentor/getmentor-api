package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log *zap.Logger
)

// Config holds logger configuration
type Config struct {
	Level       string
	LogDir      string
	Environment string
}

// Initialize sets up the global logger
func Initialize(cfg Config) error {
	var config zap.Config

	// Determine log level
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return fmt.Errorf("invalid log level %s: %w", cfg.Level, err)
	}

	if cfg.Environment == "development" {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	config.Level = zap.NewAtomicLevelAt(level)

	// Set output paths
	if cfg.Environment == "production" && cfg.LogDir != "" {
		// Ensure log directory exists
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		config.OutputPaths = []string{
			"stdout",
			filepath.Join(cfg.LogDir, "app.log"),
		}
		config.ErrorOutputPaths = []string{
			"stderr",
			filepath.Join(cfg.LogDir, "error.log"),
		}
	} else {
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	}

	// Build logger
	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	Log = logger
	return nil
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// With creates a child logger with additional fields
func With(fields ...zap.Field) *zap.Logger {
	return Log.With(fields...)
}

// Sync flushes any buffered log entries
func Sync() {
	_ = Log.Sync()
}

// LogHTTPRequest logs an HTTP request with standard fields
func LogHTTPRequest(method, path string, statusCode int, duration float64, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", statusCode),
		zap.Float64("duration", duration),
	}
	baseFields = append(baseFields, fields...)

	if statusCode >= 500 {
		Error("HTTP request failed", baseFields...)
	} else if statusCode >= 400 {
		Warn("HTTP request client error", baseFields...)
	} else {
		Info("HTTP request", baseFields...)
	}
}

// LogAPICall logs an external API call
func LogAPICall(service, operation, status string, duration float64, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.String("service", service),
		zap.String("operation", operation),
		zap.String("status", status),
		zap.Float64("duration", duration),
	}
	baseFields = append(baseFields, fields...)

	if status == "error" {
		Error("API call failed", baseFields...)
	} else {
		Info("API call", baseFields...)
	}
}

// LogError logs an error with context
func LogError(err error, msg string, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.Error(err),
	}
	baseFields = append(baseFields, fields...)
	Error(msg, baseFields...)
}
