package errortracking

import (
	"fmt"
	"runtime/debug"
	"time"

	posthog "github.com/posthog/posthog-go"
	"go.uber.org/zap"

	"github.com/getmentor/getmentor-api/pkg/logger"
)

const (
	// distinctID is used for all backend error events since there's no user session.
	distinctID = "backend-service"
)

// client is the package-level singleton.
var client *errorTrackingClient

type errorTrackingClient struct {
	ph          posthog.Client
	environment string
	version     string
}

// Init initializes the singleton PostHog error tracking client.
// If apiKey is empty, initialization is skipped and a warning is logged.
// Safe to call multiple times; only the first call has effect.
func Init(apiKey, host, environment, serviceVersion string) {
	if apiKey == "" {
		logger.Warn("PostHog error tracking disabled: POSTHOG_API_KEY not set")
		return
	}

	if host == "" {
		host = "https://eu.i.posthog.com"
	}

	ph, err := posthog.NewWithConfig(apiKey, posthog.Config{
		Endpoint: host,
	})
	if err != nil {
		logger.Warn("Failed to initialize PostHog error tracking client", zap.Error(err))
		return
	}

	client = &errorTrackingClient{
		ph:          ph,
		environment: environment,
		version:     serviceVersion,
	}

	logger.Info("PostHog error tracking initialized",
		zap.String("host", host),
		zap.String("environment", environment),
	)
}

// Close flushes pending events and shuts down the client. Call this on graceful shutdown.
func Close() {
	if client == nil {
		return
	}
	if err := client.ph.Close(); err != nil {
		logger.Warn("Failed to close PostHog error tracking client", zap.Error(err))
	}
}

// CaptureException reports an error to PostHog error tracking.
// Extra properties are merged into the event (must not contain PII).
// No-ops if the client is not initialized or err is nil.
func CaptureException(err error, properties map[string]interface{}) {
	if client == nil || err == nil {
		return
	}
	errType := fmt.Sprintf("%T", err)
	client.capture(errType, err.Error(), nil, properties)
}

// CapturePanic reports a recovered panic to PostHog error tracking.
// stack should be the output of debug.Stack() captured at recovery time.
// No-ops if the client is not initialized.
func CapturePanic(recovered interface{}, stack []byte) {
	if client == nil {
		return
	}
	panicType := fmt.Sprintf("%T", recovered)
	panicMsg := fmt.Sprintf("%v", recovered)
	client.capture(panicType, panicMsg, stack, map[string]interface{}{"panic": true})
}

// capture is the internal implementation. If rawStack is nil, debug.Stack() is called.
func (c *errorTrackingClient) capture(errType, errMsg string, rawStack []byte, extraProps map[string]interface{}) {
	if rawStack == nil {
		rawStack = debug.Stack()
	}

	extractor := posthog.DefaultStackTraceExtractor{InAppDecider: posthog.SimpleInAppDecider}
	// skip=4: runtime.Callers, GetStackTrace, capture, CaptureException/CapturePanic
	stacktrace := extractor.GetStackTrace(4)

	props := posthog.NewProperties().
		Set("source_system", "backend").
		Set("environment", c.environment).
		Set("service_version", c.version).
		Set("$exception_type", errType).
		Set("$exception_message", errMsg).
		Set("$exception_stack_trace_raw", string(rawStack)).
		Set("$exception_list", []posthog.ExceptionItem{
			{
				Type:       errType,
				Value:      errMsg,
				Stacktrace: stacktrace,
			},
		})

	for k, v := range extraProps {
		props.Set(k, v)
	}

	if err := c.ph.Enqueue(posthog.Capture{
		DistinctId: distinctID,
		Event:      "$exception",
		Timestamp:  time.Now().UTC(),
		Properties: props,
	}); err != nil {
		logger.Warn("Failed to enqueue PostHog exception event",
			zap.String("error_type", errType),
			zap.Error(err),
		)
	}
}
