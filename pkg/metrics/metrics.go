package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Registry is the custom Prometheus registry that wraps metrics with service_name label
	Registry *prometheus.Registry

	// HTTP Metrics
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPRequestTotal    *prometheus.CounterVec
	ActiveRequests      *prometheus.GaugeVec

	// Database Client Metrics (PostgreSQL)
	DBRequestDuration *prometheus.HistogramVec
	DBRequestTotal    *prometheus.CounterVec

	// Cache Metrics
	CacheHits   *prometheus.CounterVec
	CacheMisses *prometheus.CounterVec
	CacheSize   *prometheus.GaugeVec

	// Storage Client Metrics (Yandex Object Storage)
	YandexStorageRequestDuration *prometheus.HistogramVec
	YandexStorageRequestTotal    *prometheus.CounterVec

	// Business Metrics
	MentorProfileViews     *prometheus.CounterVec
	ContactFormSubmissions *prometheus.CounterVec
	ProfileUpdates         *prometheus.CounterVec
	ProfilePictureUploads  *prometheus.CounterVec
	MentorRegistrations    *prometheus.CounterVec

	// Mentor Auth Metrics
	MentorAuthLoginRequests     *prometheus.CounterVec
	MentorAuthLoginDuration     prometheus.Histogram
	MentorAuthVerifyRequests    *prometheus.CounterVec
	MentorAuthVerifyDuration    prometheus.Histogram
	MentorRequestsListTotal     *prometheus.CounterVec
	MentorRequestsListDuration  prometheus.Histogram
	MentorRequestsStatusUpdates *prometheus.CounterVec
	MentorRequestsDeclines      *prometheus.CounterVec

	// MCP Metrics
	MCPRequestTotal    *prometheus.CounterVec
	MCPRequestDuration *prometheus.HistogramVec
	MCPToolInvocations *prometheus.CounterVec
	MCPSearchKeywords  *prometheus.CounterVec
	MCPResultsReturned *prometheus.HistogramVec

	// Infrastructure Metrics
	GoRoutines prometheus.Gauge
	HeapAlloc  prometheus.Gauge
)

// Init initializes the metrics registry with service_name label from config
// Uses WrapRegistererWith to automatically inject service_name into ALL metrics
// Must be called from main.go after config is loaded
func Init(serviceName string) {
	// Create custom registry
	Registry = prometheus.NewRegistry()

	// Wrap registry to automatically add service_name label to all metrics
	// This eliminates need for ConstLabels on individual metrics
	wrapped := prometheus.WrapRegistererWith(
		prometheus.Labels{"service_name": serviceName},
		Registry,
	)

	// Create promauto factory that uses the wrapped registry
	factory := promauto.With(wrapped)

	// HTTP Metrics
	HTTPRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"http_request_method", "http_route", "http_response_status_code"},
	)

	HTTPRequestTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_request_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"http_request_method", "http_route", "http_response_status_code"},
	)

	ActiveRequests = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_server_active_requests",
			Help: "Number of active HTTP requests",
		},
		[]string{"http_request_method"},
	)

	// Database Client Metrics (PostgreSQL)
	DBRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_client_operation_duration_seconds",
			Help:    "Database client operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	DBRequestTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_client_operation_total",
			Help: "Total number of database client operations",
		},
		[]string{"operation", "status"},
	)

	// Cache Metrics
	CacheHits = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_name"},
	)

	CacheMisses = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_name"},
	)

	CacheSize = factory.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_entries",
			Help: "Number of entries in cache",
		},
		[]string{"cache_name"},
	)

	// Storage Client Metrics (Yandex Object Storage)
	YandexStorageRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "yandex_storage_operation_duration_seconds",
			Help:    "Yandex Object Storage operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	YandexStorageRequestTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "yandex_storage_operation_total",
			Help: "Total number of Yandex Object Storage operations",
		},
		[]string{"operation", "status"},
	)

	// Business Metrics
	MentorProfileViews = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_profile_views_total",
			Help: "Total number of mentor profile views",
		},
		[]string{},
	)

	ContactFormSubmissions = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_contact_form_submissions_total",
			Help: "Total number of contact form submissions",
		},
		[]string{"status"},
	)

	ProfileUpdates = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_profile_updates_total",
			Help: "Total number of profile updates",
		},
		[]string{"status"},
	)

	ProfilePictureUploads = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_profile_picture_uploads_total",
			Help: "Total number of profile picture uploads",
		},
		[]string{"status"},
	)

	MentorRegistrations = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_registrations_total",
			Help: "Total mentor registration attempts",
		},
		[]string{"status"},
	)

	// Mentor Auth Metrics
	MentorAuthLoginRequests = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_auth_login_requests_total",
			Help: "Total mentor login requests",
		},
		[]string{"status"},
	)

	MentorAuthLoginDuration = factory.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "getmentor_mentor_auth_login_duration_seconds",
			Help:    "Mentor login request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	MentorAuthVerifyRequests = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_auth_verify_requests_total",
			Help: "Total mentor token verification requests",
		},
		[]string{"status"},
	)

	MentorAuthVerifyDuration = factory.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "getmentor_mentor_auth_verify_duration_seconds",
			Help:    "Mentor token verification duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	MentorRequestsListTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_requests_list_total",
			Help: "Total mentor requests list fetches",
		},
		[]string{"group"},
	)

	MentorRequestsListDuration = factory.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "getmentor_mentor_requests_list_duration_seconds",
			Help:    "Mentor requests list duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	MentorRequestsStatusUpdates = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_requests_status_updates_total",
			Help: "Total mentor request status updates",
		},
		[]string{"from_status", "to_status"},
	)

	MentorRequestsDeclines = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_requests_declines_total",
			Help: "Total mentor request declines",
		},
		[]string{"reason"},
	)

	// MCP Metrics
	MCPRequestTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mcp_request_total",
			Help: "Total number of MCP requests",
		},
		[]string{"http_request_method", "http_response_status_code"},
	)

	MCPRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "getmentor_mcp_request_duration_seconds",
			Help:    "MCP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"http_request_method"},
	)

	MCPToolInvocations = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mcp_tool_invocations_total",
			Help: "Total number of MCP tool invocations",
		},
		[]string{"tool", "status"},
	)

	MCPSearchKeywords = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mcp_search_keywords_total",
			Help: "Total number of MCP search queries (tracks keyword usage)",
		},
		[]string{"keyword_count_range"}, // "1-2", "3-5", "6-10", "10+"
	)

	MCPResultsReturned = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "getmentor_mcp_results_returned",
			Help:    "Number of results returned by MCP tools",
			Buckets: []float64{0, 1, 5, 10, 20, 50, 100, 200},
		},
		[]string{"tool"},
	)

	// Infrastructure Metrics
	GoRoutines = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_runtime_go_goroutines",
			Help: "Number of goroutines",
		},
	)

	HeapAlloc = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_runtime_go_mem_heap_alloc_bytes",
			Help: "Heap allocated bytes",
		},
	)
}

// RecordInfrastructureMetrics collects infrastructure metrics periodically
func RecordInfrastructureMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	// TODO: Add stop channel/context to metrics goroutine to prevent leak on shutdown
	go func() {
		for range ticker.C {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			GoRoutines.Set(float64(runtime.NumGoroutine()))
			HeapAlloc.Set(float64(m.HeapAlloc))
		}
	}()
}

// MeasureDuration measures the duration of an operation
func MeasureDuration(start time.Time) float64 {
	return time.Since(start).Seconds()
}
