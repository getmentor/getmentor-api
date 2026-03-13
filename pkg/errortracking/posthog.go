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

// CaptureException reports an error to PostHog error tracking with a structured stack trace
// captured at the call site. Extra properties are merged into the event (must not contain PII).
// No-ops if the client is not initialized or err is nil.
func CaptureException(err error, properties map[string]interface{}) {
	if client == nil || err == nil {
		return
	}
	errType := fmt.Sprintf("%T", err)

	// Capture structured stack trace here, before any internal call indirection.
	// skip=4: runtime.Callers, GetStackTrace, captureWithStack, CaptureException
	extractor := posthog.DefaultStackTraceExtractor{InAppDecider: posthog.SimpleInAppDecider}
	stacktrace := extractor.GetStackTrace(4)

	client.captureWithStack(errType, err.Error(), debug.Stack(), stacktrace, properties)
}

// CapturePanic reports a recovered panic to PostHog error tracking.
// stack must be the output of debug.Stack() captured immediately inside the recover() block,
// before any other calls, so it reflects the original panic origin.
// No-ops if the client is not initialized.
func CapturePanic(recovered interface{}, stack []byte) {
	if client == nil {
		return
	}
	panicType := fmt.Sprintf("%T", recovered)
	panicMsg := fmt.Sprintf("%v", recovered)

	// For panics, we do NOT use GetStackTrace — it would capture the recovery
	// middleware frames, not the panic origin. The raw debug.Stack() output
	// (captured at recovery time) already contains the full panic origin stack,
	// and PostHog can parse it via $exception_stack_trace_raw.
	client.captureWithStack(panicType, panicMsg, stack, nil, map[string]interface{}{"panic": true})
}

// captureWithStack is the internal implementation.
// structuredStack may be nil (e.g. for panics where the call-site stack is meaningless).
func (c *errorTrackingClient) captureWithStack(
	errType, errMsg string,
	rawStack []byte,
	structuredStack *posthog.ExceptionStacktrace,
	extraProps map[string]interface{},
) {
	exceptionItem := posthog.ExceptionItem{
		Type:       errType,
		Value:      errMsg,
		Stacktrace: structuredStack, // nil for panics; PostHog falls back to $exception_stack_trace_raw
	}

	props := posthog.NewProperties().
		Set("source_system", "backend").
		Set("environment", c.environment).
		Set("service_version", c.version).
		Set("$exception_type", errType).
		Set("$exception_message", errMsg).
		Set("$exception_stack_trace_raw", string(rawStack)).
		Set("$exception_list", []posthog.ExceptionItem{exceptionItem})

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
