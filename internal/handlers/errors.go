package handlers

import (
	"github.com/gin-gonic/gin"
)

// attachError attaches err to the gin context so the observability middleware
// can include the reason in the request log. c.Error() returns *gin.Error (not
// the error interface), so we suppress errcheck here intentionally.
func attachError(c *gin.Context, err error) {
	if err != nil {
		_ = c.Error(err) //nolint:errcheck
	}
}

// respondError sends an error JSON response and attaches the error to the gin context
// so the observability middleware can include the reason in the request log.
func respondError(c *gin.Context, status int, message string, err error) {
	attachError(c, err)
	c.JSON(status, gin.H{"error": message})
}

// respondErrorWithDetails sends an error response with an additional details field.
func respondErrorWithDetails(c *gin.Context, status int, message string, details any, err error) { //nolint:unparam
	attachError(c, err)
	c.JSON(status, gin.H{"error": message, "details": details})
}
