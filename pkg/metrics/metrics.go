package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Custom histogram buckets optimized for API response times ranging from milliseconds to 30+ seconds
	// This provides better granularity for monitoring Airtable API calls and cache refresh operations
	// Note: Removed 60s bucket to avoid histogram_quantile interpolation issues with low sample counts
	CustomAPIBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 3, 5, 8, 13, 21, 34, 55}

	// HTTP Metrics
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: CustomAPIBuckets,
		},
		[]string{"http_request_method", "http_route", "http_response_status_code"},
	)

	HTTPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_request_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"http_request_method", "http_route", "http_response_status_code"},
	)

	ActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_server_active_requests",
			Help: "Number of active HTTP requests",
		},
		[]string{"http_request_method", "http_route"},
	)

	// Database Client Metrics (Airtable)
	AirtableRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_client_operation_duration_seconds",
			Help:    "Database client operation duration in seconds",
			Buckets: CustomAPIBuckets,
		},
		[]string{"operation", "status"},
	)

	AirtableRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_client_operation_total",
			Help: "Total number of database client operations",
		},
		[]string{"operation", "status"},
	)

	// Cache Metrics
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_name"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_name"},
	)

	CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_entries",
			Help: "Number of entries in cache",
		},
		[]string{"cache_name"},
	)

	// Storage Client Metrics (Azure)
	AzureStorageRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "storage_client_operation_duration_seconds",
			Help:    "Storage client operation duration in seconds",
			Buckets: CustomAPIBuckets,
		},
		[]string{"operation", "status"},
	)

	AzureStorageRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "storage_client_operation_total",
			Help: "Total number of storage client operations",
		},
		[]string{"operation", "status"},
	)

	// Business Metrics
	MentorProfileViews = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_profile_views_total",
			Help: "Total number of mentor profile views",
		},
		[]string{"mentor_slug"},
	)

	ContactFormSubmissions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_contact_form_submissions_total",
			Help: "Total number of contact form submissions",
		},
		[]string{"status"},
	)

	ProfileUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_profile_updates_total",
			Help: "Total number of profile updates",
		},
		[]string{"status"},
	)

	ProfilePictureUploads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_profile_picture_uploads_total",
			Help: "Total number of profile picture uploads",
		},
		[]string{"status"},
	)

	MentorRegistrations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mentor_registrations_total",
			Help: "Total mentor registration attempts",
		},
		[]string{"status"},
	)

	// MCP Metrics
	MCPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mcp_request_total",
			Help: "Total number of MCP requests",
		},
		[]string{"http_request_method", "http_response_status_code"},
	)

	MCPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "getmentor_mcp_request_duration_seconds",
			Help:    "MCP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"http_request_method"},
	)

	MCPToolInvocations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mcp_tool_invocations_total",
			Help: "Total number of MCP tool invocations",
		},
		[]string{"tool", "status"},
	)

	MCPSearchKeywords = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "getmentor_mcp_search_keywords_total",
			Help: "Total number of MCP search queries (tracks keyword usage)",
		},
		[]string{"keyword_count_range"}, // "1-2", "3-5", "6-10", "10+"
	)

	MCPResultsReturned = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "getmentor_mcp_results_returned",
			Help:    "Number of results returned by MCP tools",
			Buckets: []float64{0, 1, 5, 10, 20, 50, 100, 200},
		},
		[]string{"tool"},
	)

	// Infrastructure Metrics
	GoRoutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_runtime_go_goroutines",
			Help: "Number of goroutines",
		},
	)

	HeapAlloc = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "process_runtime_go_mem_heap_alloc_bytes",
			Help: "Heap allocated bytes",
		},
	)
)

// RecordInfrastructureMetrics collects infrastructure metrics periodically
func RecordInfrastructureMetrics() {
	ticker := time.NewTicker(15 * time.Second)
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
