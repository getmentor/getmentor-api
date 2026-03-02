package analytics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

type slowTransport struct {
	mu    sync.Mutex
	delay time.Duration
	calls int
}

func (t *slowTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	time.Sleep(t.delay)

	t.mu.Lock()
	t.calls++
	t.mu.Unlock()

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":1}`)),
		Header:     make(http.Header),
	}, nil
}

func (t *slowTransport) Calls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func waitForBody(t *testing.T, transport *captureTransport) string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		body := transport.Body()
		if body != "" {
			return body
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for Mixpanel payload")
	return ""
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

	body := waitForBody(t, transport)
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

func TestMixpanelTracker_Track_DoesNotBlockOnSlowNetwork(t *testing.T) {
	transport := &slowTransport{delay: 300 * time.Millisecond}
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

	startedAt := time.Now()
	tracker.Track(context.Background(), EventMenteeContactSubmitted, MentorDistinctID("mentor-123"), map[string]interface{}{
		"mentor_id": "mentor-123",
		"outcome":   "success",
	})
	elapsed := time.Since(startedAt)

	assert.Less(t, elapsed, 100*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if transport.Calls() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for async Mixpanel worker request")
}

func TestNewTracker_DisabledReturnsNoop(t *testing.T) {
	t.Parallel()

	tracker := NewTracker(&Config{
		Enabled: false,
		Token:   "",
	})

	assert.IsType(t, NoopTracker{}, tracker)
}
