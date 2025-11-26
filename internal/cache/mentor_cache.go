package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/airtable"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

const (
	mentorsCacheKey  = "mentors"
	mentorsCacheName = "mentors"
	cacheCheckPeriod = 10 * time.Second
	maxRetries       = 3
	initialRetryWait = 2 * time.Second
)

// MentorCache manages the in-memory cache for mentors
type MentorCache struct {
	cache          *gocache.Cache
	airtableClient *airtable.Client
	mu             sync.RWMutex
	refreshing     bool
	ready          bool // Indicates if cache has been successfully initialized
	ttl            time.Duration
}

// NewMentorCache creates a new mentor cache
func NewMentorCache(airtableClient *airtable.Client, ttlSeconds int) *MentorCache {
	ttl := time.Duration(ttlSeconds) * time.Second
	cache := gocache.New(ttl, cacheCheckPeriod)

	mc := &MentorCache{
		cache:          cache,
		airtableClient: airtableClient,
		refreshing:     false,
		ready:          false,
		ttl:            ttl,
	}

	// Set up expiration callback to auto-refresh in background
	cache.OnEvicted(func(key string, _ interface{}) {
		if key == mentorsCacheKey {
			logger.Info("Mentor cache expired, triggering background refresh")
			go func() {
				if err := mc.refreshInBackground(); err != nil {
					logger.Error("Failed to refresh mentor cache in background", zap.Error(err))
				}
			}()
		}
	})

	return mc
}

// Initialize performs initial cache population (synchronous, blocks until ready)
// Should be called during application startup before accepting requests
func (mc *MentorCache) Initialize() error {
	logger.Info("Initializing mentor cache...")
	err := mc.refreshWithRetry()
	if err != nil {
		logger.Error("Failed to initialize mentor cache", zap.Error(err))
		return err
	}

	mc.mu.Lock()
	mc.ready = true
	mc.mu.Unlock()

	logger.Info("Mentor cache initialized successfully")
	return nil
}

// IsReady returns true if the cache has been successfully initialized
func (mc *MentorCache) IsReady() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.ready
}

// Get retrieves mentors from cache (stale-while-revalidate pattern)
// Returns cached data even if refresh is in progress
func (mc *MentorCache) Get() ([]*models.Mentor, error) {
	// Check cache
	if data, found := mc.cache.Get(mentorsCacheKey); found {
		metrics.CacheHits.WithLabelValues(mentorsCacheName).Inc()
		logger.Debug("Mentor cache hit")

		mentors, ok := data.([]*models.Mentor)
		if !ok {
			logger.Error("Invalid cache data type")
			mc.cache.Delete(mentorsCacheKey)
			return nil, fmt.Errorf("invalid cache data type")
		}
		metrics.CacheSize.WithLabelValues(mentorsCacheName).Set(float64(len(mentors)))

		return mentors, nil
	}

	metrics.CacheMisses.WithLabelValues(mentorsCacheName).Inc()
	logger.Info("Mentor cache miss, fetching from Airtable")

	// Cache miss, need to fetch
	// If not ready yet, this is an error
	if !mc.IsReady() {
		return nil, fmt.Errorf("cache not initialized")
	}

	// Try to refresh synchronously on cache miss
	return mc.refreshSync()
}

// ForceRefresh forces a cache refresh
func (mc *MentorCache) ForceRefresh() ([]*models.Mentor, error) {
	logger.Info("Force refreshing mentor cache")
	return mc.refreshSync()
}

// refreshSync performs a synchronous refresh
// Returns old cache data if refresh is already in progress
func (mc *MentorCache) refreshSync() ([]*models.Mentor, error) {
	mc.mu.Lock()

	// Check if already refreshing - if so, return old cache data (stale-while-revalidate)
	if mc.refreshing {
		mc.mu.Unlock()
		logger.Debug("Refresh already in progress, returning stale data if available")

		// Return stale data if available
		if data, found := mc.cache.Get(mentorsCacheKey); found {
			logger.Info("Returning stale cache data while refresh is in progress")
			return data.([]*models.Mentor), nil
		}

		// No stale data available, wait for refresh to complete
		logger.Warn("No stale data available, waiting for refresh to complete")
		time.Sleep(2 * time.Second)

		if data, found := mc.cache.Get(mentorsCacheKey); found {
			return data.([]*models.Mentor), nil
		}

		return nil, fmt.Errorf("cache refresh in progress and no stale data available")
	}

	mc.refreshing = true
	mc.mu.Unlock()

	defer func() {
		mc.mu.Lock()
		mc.refreshing = false
		mc.mu.Unlock()
	}()

	return mc.doRefresh()
}

// refreshInBackground performs a background refresh
// Does not block, does not return data
func (mc *MentorCache) refreshInBackground() error {
	mc.mu.Lock()

	// Check if already refreshing
	if mc.refreshing {
		mc.mu.Unlock()
		logger.Debug("Refresh already in progress, skipping background refresh")
		return nil
	}

	mc.refreshing = true
	mc.mu.Unlock()

	defer func() {
		mc.mu.Lock()
		mc.refreshing = false
		mc.mu.Unlock()
	}()

	_, err := mc.doRefresh()
	return err
}

// refreshWithRetry performs a refresh with exponential backoff retry logic
func (mc *MentorCache) refreshWithRetry() error {
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			waitTime := initialRetryWait * time.Duration(1<<uint(attempt-1)) // Exponential backoff
			logger.Info("Retrying cache refresh",
				zap.Int("attempt", attempt+1),
				zap.Int("max_attempts", maxRetries),
				zap.Duration("wait_time", waitTime))
			time.Sleep(waitTime)
		}

		_, err = mc.doRefresh()
		if err == nil {
			return nil
		}

		logger.Error("Cache refresh attempt failed",
			zap.Int("attempt", attempt+1),
			zap.Error(err))
	}

	return fmt.Errorf("failed to refresh cache after %d attempts: %w", maxRetries, err)
}

// doRefresh performs the actual refresh operation
func (mc *MentorCache) doRefresh() ([]*models.Mentor, error) {
	// Fetch from Airtable
	mentors, err := mc.airtableClient.GetAllMentors()
	if err != nil {
		logger.Error("Failed to fetch mentors from Airtable", zap.Error(err))
		return nil, err
	}

	// Update cache
	mc.cache.Set(mentorsCacheKey, mentors, mc.ttl)
	metrics.CacheSize.WithLabelValues(mentorsCacheName).Set(float64(len(mentors)))

	logger.Info("Mentor cache refreshed successfully", zap.Int("count", len(mentors)))

	return mentors, nil
}

// Clear clears the cache
func (mc *MentorCache) Clear() {
	mc.cache.Delete(mentorsCacheKey)
	logger.Info("Mentor cache cleared")
}
