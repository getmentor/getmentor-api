package middleware

import (
	"strconv"
	"time"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ObservabilityMiddleware instruments HTTP requests with metrics and logging
func ObservabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		// Track active requests
		metrics.ActiveRequests.WithLabelValues(method, path).Inc()
		defer metrics.ActiveRequests.WithLabelValues(method, path).Dec()

		// Process request
		c.Next()

		// Measure duration
		duration := metrics.MeasureDuration(start)
		status := c.Writer.Status()
		statusStr := strconv.Itoa(status)

		// Record metrics
		metrics.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration)
		metrics.HTTPRequestTotal.WithLabelValues(method, path, statusStr).Inc()

		// Log request
		fields := []zap.Field{
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("response_size", c.Writer.Size()),
		}

		// Add error if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
		}

		logger.LogHTTPRequest(method, path, status, duration, fields...)
	}
}
