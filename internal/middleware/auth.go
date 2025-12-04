package middleware

import (
	"net/http"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TokenAuthMiddleware validates authentication tokens
func TokenAuthMiddleware(validTokens ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("mentors_api_auth_token")

		if token == "" {
			logger.Warn("Missing authentication token",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication token"})
			c.Abort()
			return
		}

		valid := false
		for _, validToken := range validTokens {
			if token == validToken {
				valid = true
				break
			}
		}

		if !valid {
			logger.Warn("Invalid authentication token",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// InternalAPIAuthMiddleware validates internal API token
func MCPServerAuthMiddleware(validToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("x-mcp-auth-token")

		if token == "" || token != validToken {
			logger.Warn("Invalid MCP server token",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing MCP server token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// InternalAPIAuthMiddleware validates internal API token
func InternalAPIAuthMiddleware(validToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("x-internal-mentors-api-auth-token")

		if token == "" || token != validToken {
			logger.Warn("Invalid internal API token",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing internal API token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// WebhookAuthMiddleware validates webhook secret
func WebhookAuthMiddleware(validSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Webhook-Secret")

		if secret == "" || secret != validSecret {
			logger.Warn("Invalid webhook secret",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook secret"})
			c.Abort()
			return
		}

		c.Next()
	}
}
