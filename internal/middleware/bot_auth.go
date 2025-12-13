package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BotAPIAuthMiddleware validates the X-Bot-API-Key header for bot API requests
func BotAPIAuthMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			// If no API key is configured, reject all requests
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "bot API not configured"})
			return
		}

		providedKey := c.GetHeader("X-Bot-API-Key")
		if providedKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing X-Bot-API-Key header"})
			return
		}

		if providedKey != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Next()
	}
}
