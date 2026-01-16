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
		method := c.Request.Method

		// Track active requests (method only - route not known until after routing)
		metrics.ActiveRequests.WithLabelValues(method).Inc()
		defer metrics.ActiveRequests.WithLabelValues(method).Dec()

		// Process request - this allows Gin to set the matched route
		c.Next()

		// Get route template AFTER routing (prevents cardinality explosion)
		// c.FullPath() returns the route pattern like "/api/v1/mentor/requests/:id"
		// instead of the actual path like "/api/v1/mentor/requests/recXYZ123"
		path := c.FullPath()
		if path == "" {
			// Fallback for unmatched routes (404s) - use a generic label
			path = "unmatched"
		}

		// Measure duration
		duration := metrics.MeasureDuration(start)
		status := c.Writer.Status()
		statusStr := strconv.Itoa(status)

		// Record metrics with route template (not actual path)
		metrics.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration)
		metrics.HTTPRequestTotal.WithLabelValues(method, path, statusStr).Inc()

		// Log request (use actual path for debugging, but route template for metrics)
		actualPath := c.Request.URL.Path
		fields := []zap.Field{
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("response_size", c.Writer.Size()),
		}

		// Add error if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("error", c.Errors.String()))
		}

		// Log with actual path for debugging purposes
		logger.LogHTTPRequest(c.Request.Context(), method, actualPath, status, duration, fields...)
	}
}
