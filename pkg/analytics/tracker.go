package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

const (
	DefaultEndpoint     = "https://api.mixpanel.com/track?verbose=1"
	DefaultEventVersion = "v1"
	defaultTimeout      = 3 * time.Second
	defaultSource       = "api"
	defaultEnvironment  = "unknown"
)

type Tracker interface {
	Track(ctx context.Context, event string, distinctID string, properties map[string]interface{})
}

type Config struct {
	Enabled      bool
	Token        string
	Endpoint     string
	SourceSystem string
	Environment  string
	EventVersion string
	Timeout      time.Duration
	HTTPClient   *http.Client
}

type NoopTracker struct{}

func (NoopTracker) Track(context.Context, string, string, map[string]interface{}) {}

type MixpanelTracker struct {
	token        string
	endpoint     string
	sourceSystem string
	environment  string
	eventVersion string
	httpClient   *http.Client
}

type eventPayload struct {
	Event      string                 `json:"event"`
	Properties map[string]interface{} `json:"properties"`
}

func NewTracker(cfg *Config) Tracker {
	if cfg == nil {
		return NoopTracker{}
	}

	if !cfg.Enabled || strings.TrimSpace(cfg.Token) == "" {
		return NoopTracker{}
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	sourceSystem := strings.TrimSpace(cfg.SourceSystem)
	if sourceSystem == "" {
		sourceSystem = defaultSource
	}

	environment := strings.TrimSpace(cfg.Environment)
	if environment == "" {
		environment = defaultEnvironment
	}

	eventVersion := strings.TrimSpace(cfg.EventVersion)
	if eventVersion == "" {
		eventVersion = DefaultEventVersion
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	return &MixpanelTracker{
		token:        strings.TrimSpace(cfg.Token),
		endpoint:     endpoint,
		sourceSystem: sourceSystem,
		environment:  environment,
		eventVersion: eventVersion,
		httpClient:   httpClient,
	}
}

func (t *MixpanelTracker) Track(ctx context.Context, event string, distinctID string, properties map[string]interface{}) {
	event = strings.TrimSpace(event)
	if event == "" {
		return
	}

	cleanDistinctID := strings.TrimSpace(distinctID)
	if cleanDistinctID == "" {
		cleanDistinctID = SystemDistinctID(t.sourceSystem)
	}

	cleanProperties := sanitizeProperties(properties)
	cleanProperties["token"] = t.token
	cleanProperties["distinct_id"] = cleanDistinctID
	cleanProperties["time"] = time.Now().Unix()
	cleanProperties["source_system"] = t.sourceSystem
	cleanProperties["environment"] = t.environment
	cleanProperties["event_version"] = t.eventVersion

	payload := []eventPayload{
		{
			Event:      event,
			Properties: cleanProperties,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Warn("Failed to marshal Mixpanel event payload",
			zap.String("event", event),
			zap.Error(err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytes.NewReader(body))
	if err != nil {
		logger.Warn("Failed to create Mixpanel request",
			zap.String("event", event),
			zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		logger.Warn("Failed to send Mixpanel event",
			zap.String("event", event),
			zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		bodyPreview, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		if readErr != nil {
			logger.Warn("Mixpanel returned non-success status and response body could not be read",
				zap.String("event", event),
				zap.Int("status_code", resp.StatusCode),
				zap.Error(readErr))
			return
		}
		logger.Warn("Mixpanel returned non-success status",
			zap.String("event", event),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(bodyPreview)))
	}
}

func MentorDistinctID(mentorID string) string {
	return prefixedDistinctID("mentor", mentorID)
}

func ModeratorDistinctID(moderatorID string) string {
	return prefixedDistinctID("moderator", moderatorID)
}

func RequestDistinctID(requestID string) string {
	return prefixedDistinctID("request", requestID)
}

func SystemDistinctID(system string) string {
	cleanSystem := strings.TrimSpace(system)
	if cleanSystem == "" {
		cleanSystem = defaultSource
	}
	return fmt.Sprintf("system:%s", cleanSystem)
}

func prefixedDistinctID(prefix, id string) string {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", prefix, cleanID)
}

func sanitizeProperties(properties map[string]interface{}) map[string]interface{} {
	if len(properties) == 0 {
		return map[string]interface{}{}
	}

	safe := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" || isBlockedPropertyKey(normalizedKey) || value == nil {
			continue
		}

		switch typedValue := value.(type) {
		case string:
			safe[normalizedKey] = trimStringValue(typedValue)
		case bool, int, int8, int16, int32, int64, float32, float64, uint, uint8, uint16, uint32, uint64:
			safe[normalizedKey] = typedValue
		case time.Time:
			safe[normalizedKey] = typedValue.Unix()
		case []string:
			safe[normalizedKey] = typedValue
		default:
			safe[normalizedKey] = trimStringValue(fmt.Sprint(typedValue))
		}
	}

	return safe
}

func trimStringValue(input string) string {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) <= 512 {
		return trimmed
	}
	return trimmed[:512]
}

func isBlockedPropertyKey(key string) bool {
	blockedKeys := map[string]struct{}{
		"email":             {},
		"mentor_email":      {},
		"moderator_email":   {},
		"name":              {},
		"mentor_name":       {},
		"moderator_name":    {},
		"telegram":          {},
		"telegram_username": {},
		"intro":             {},
		"description":       {},
		"review":            {},
		"mentor_review":     {},
		"platform_review":   {},
		"improvements":      {},
		"login_url":         {},
	}

	_, found := blockedKeys[strings.ToLower(strings.TrimSpace(key))]
	return found
}
