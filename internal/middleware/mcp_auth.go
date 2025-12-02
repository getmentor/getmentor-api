package middleware

import (
	"net/http"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MCPAuthMiddleware validates MCP API authentication token
// Uses X-MCP-API-Token header for authentication
func MCPAuthMiddleware(validToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-MCP-API-Token")

		if token == "" {
			logger.Warn("MCP request missing authentication token",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			metrics.MCPErrors.WithLabelValues("auth_missing").Inc()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing MCP authentication token"})
			c.Abort()
			return
		}

		if token != validToken {
			logger.Warn("MCP request with invalid authentication token",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			metrics.MCPErrors.WithLabelValues("auth_invalid").Inc()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid MCP authentication token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
