package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gm_api_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status_code"},
	)

	HTTPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_http_request_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route", "status_code"},
	)

	ActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gm_api_active_requests",
			Help: "Number of active HTTP requests",
		},
		[]string{"method", "route"},
	)

	// Airtable Metrics
	AirtableRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gm_api_airtable_request_duration_seconds",
			Help:    "Airtable API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	AirtableRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_airtable_request_total",
			Help: "Total number of Airtable API requests",
		},
		[]string{"operation", "status"},
	)

	// Cache Metrics
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_name"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_name"},
	)

	CacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gm_api_cache_size",
			Help: "Number of items in cache",
		},
		[]string{"cache_name"},
	)

	// Azure Storage Metrics
	AzureStorageRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gm_api_azure_storage_request_duration_seconds",
			Help:    "Azure Storage request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	AzureStorageRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_azure_storage_request_total",
			Help: "Total number of Azure Storage requests",
		},
		[]string{"operation", "status"},
	)

	// Business Metrics
	MentorProfileViews = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_mentor_profile_views_total",
			Help: "Total number of mentor profile views",
		},
		[]string{"mentor_slug"},
	)

	ContactFormSubmissions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_contact_form_submissions_total",
			Help: "Total number of contact form submissions",
		},
		[]string{"status"},
	)

	ProfileUpdates = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_profile_updates_total",
			Help: "Total number of profile updates",
		},
		[]string{"status"},
	)

	ProfilePictureUploads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_profile_picture_uploads_total",
			Help: "Total number of profile picture uploads",
		},
		[]string{"status"},
	)

	// MCP Metrics
	MCPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_mcp_request_total",
			Help: "Total number of MCP requests",
		},
		[]string{"method", "status"},
	)

	MCPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gm_api_mcp_request_duration_seconds",
			Help:    "MCP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	MCPToolInvocations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_mcp_tool_invocations_total",
			Help: "Total number of MCP tool invocations",
		},
		[]string{"tool", "status"},
	)

	MCPSearchKeywords = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gm_api_mcp_search_keywords_total",
			Help: "Total number of MCP search queries (tracks keyword usage)",
		},
		[]string{"keyword_count_range"}, // "1-2", "3-5", "6-10", "10+"
	)

	MCPResultsReturned = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gm_api_mcp_results_returned",
			Help:    "Number of results returned by MCP tools",
			Buckets: []float64{0, 1, 5, 10, 20, 50, 100, 200},
		},
		[]string{"tool"},
	)

	// Infrastructure Metrics
	GoRoutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gm_api_goroutines",
			Help: "Number of goroutines",
		},
	)

	MemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gm_api_memory_usage_bytes",
			Help: "Memory usage in bytes",
		},
	)

	HeapAlloc = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gm_api_heap_alloc_bytes",
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
			MemoryUsage.Set(float64(m.Alloc))
			HeapAlloc.Set(float64(m.HeapAlloc))
		}
	}()
}

// MeasureDuration measures the duration of an operation
func MeasureDuration(start time.Time) float64 {
	return time.Since(start).Seconds()
}
