package trigger

import (
	"fmt"

	"github.com/getmentor/getmentor-api/pkg/httpclient"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

// CallAsync calls a trigger URL asynchronously with a record_id query parameter.
// This is used to trigger Azure Functions after Airtable operations.
// Failures are logged but don't block the operation.
func CallAsync(triggerURL, recordID string, httpClient httpclient.Client) {
	if triggerURL == "" {
		// No trigger URL configured, skip silently
		return
	}

	// Run in goroutine to avoid blocking
	go func() {
		targetURL := fmt.Sprintf("%s%s", triggerURL, recordID)

		logger.Info("Calling trigger URL",
			zap.String("url", targetURL),
			zap.String("record_id", recordID))

		resp, err := httpClient.Get(targetURL)
		if err != nil {
			logger.Error("Failed to call trigger URL",
				zap.Error(err),
				zap.String("url", targetURL),
				zap.String("record_id", recordID))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.Info("Trigger URL called successfully",
				zap.String("url", targetURL),
				zap.String("record_id", recordID),
				zap.Int("status_code", resp.StatusCode))
		} else {
			logger.Warn("Trigger URL returned non-success status",
				zap.String("url", targetURL),
				zap.String("record_id", recordID),
				zap.Int("status_code", resp.StatusCode))
		}
	}()
}
