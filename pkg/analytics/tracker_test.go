package analytics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureTransport struct {
	mu          sync.Mutex
	requestBody string
}

func (t *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.requestBody = string(body)
	t.mu.Unlock()

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":1}`)),
		Header:     make(http.Header),
	}, nil
}

func (t *captureTransport) Body() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.requestBody
}

func TestMixpanelTracker_Track_SanitizesAndAddsCommonProps(t *testing.T) {
	t.Parallel()

	transport := &captureTransport{}
	client := &http.Client{Transport: transport}
	tracker := NewTracker(&Config{
		Enabled:      true,
		Token:        "test-token",
		Endpoint:     "https://mixpanel.invalid/track",
		SourceSystem: "api",
		Environment:  "staging",
		EventVersion: "v9",
		HTTPClient:   client,
	})

	tracker.Track(context.Background(), EventMenteeContactSubmitted, MentorDistinctID("mentor-123"), map[string]interface{}{
		"email":     "private@getmentor.dev",
		"name":      "Private Name",
		"mentor_id": "mentor-123",
		"outcome":   "success",
	})

	body := transport.Body()
	require.NotEmpty(t, body)
	assert.Contains(t, body, EventMenteeContactSubmitted)
	assert.Contains(t, body, `"token":"test-token"`)
	assert.Contains(t, body, `"distinct_id":"mentor:mentor-123"`)
	assert.Contains(t, body, `"source_system":"api"`)
	assert.Contains(t, body, `"environment":"staging"`)
	assert.Contains(t, body, `"event_version":"v9"`)
	assert.Contains(t, body, `"mentor_id":"mentor-123"`)
	assert.Contains(t, body, `"outcome":"success"`)
	assert.NotContains(t, body, "private@getmentor.dev")
	assert.NotContains(t, body, "Private Name")
}

func TestNewTracker_DisabledReturnsNoop(t *testing.T) {
	t.Parallel()

	tracker := NewTracker(&Config{
		Enabled: false,
		Token:   "",
	})

	assert.IsType(t, NoopTracker{}, tracker)
}
