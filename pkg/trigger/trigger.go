package trigger

import (
	"bytes"
	"encoding/json"
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

// CallAsyncWithPayload calls a trigger URL asynchronously with a JSON payload.
// This is used for triggers that need more than just a record ID.
// Failures are logged but don't block the operation.
func CallAsyncWithPayload(triggerURL string, payload interface{}, httpClient httpclient.Client) {
	if triggerURL == "" {
		// No trigger URL configured, skip silently
		return
	}

	// Run in goroutine to avoid blocking
	go func() {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			logger.Error("Failed to marshal trigger payload",
				zap.Error(err),
				zap.String("url", triggerURL))
			return
		}

		logger.Info("Calling trigger URL with payload",
			zap.String("url", triggerURL))

		resp, err := httpClient.Post(triggerURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			logger.Error("Failed to call trigger URL",
				zap.Error(err),
				zap.String("url", triggerURL))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.Info("Trigger URL called successfully",
				zap.String("url", triggerURL),
				zap.Int("status_code", resp.StatusCode))
		} else {
			logger.Warn("Trigger URL returned non-success status",
				zap.String("url", triggerURL),
				zap.Int("status_code", resp.StatusCode))
		}
	}()
}
