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
	mentorsCacheTTL  = 60 * time.Second
	cacheCheckPeriod = 10 * time.Second
)

// MentorCache manages the in-memory cache for mentors
type MentorCache struct {
	cache          *gocache.Cache
	airtableClient *airtable.Client
	mu             sync.RWMutex
	refreshing     bool
}

// NewMentorCache creates a new mentor cache
func NewMentorCache(airtableClient *airtable.Client) *MentorCache {
	cache := gocache.New(mentorsCacheTTL, cacheCheckPeriod)

	mc := &MentorCache{
		cache:          cache,
		airtableClient: airtableClient,
		refreshing:     false,
	}

	// Set up expiration callback to auto-refresh
	cache.OnEvicted(func(key string, value interface{}) {
		if key == mentorsCacheKey {
			logger.Info("Mentor cache expired, triggering refresh")
			go func() {
				if _, err := mc.refresh(); err != nil {
					logger.Error("Failed to refresh mentor cache", zap.Error(err))
				}
			}()
		}
	})

	// Initial population
	go mc.warmUp()

	return mc
}

// Get retrieves mentors from cache or fetches them if cache miss
func (mc *MentorCache) Get() ([]*models.Mentor, error) {
	// Check cache
	if data, found := mc.cache.Get(mentorsCacheKey); found {
		metrics.CacheHits.WithLabelValues(mentorsCacheName).Inc()
		logger.Debug("Mentor cache hit")

		mentors := data.([]*models.Mentor)
		metrics.CacheSize.WithLabelValues(mentorsCacheName).Set(float64(len(mentors)))

		return mentors, nil
	}

	metrics.CacheMisses.WithLabelValues(mentorsCacheName).Inc()
	logger.Info("Mentor cache miss, fetching from Airtable")

	// Cache miss, fetch and populate
	return mc.refresh()
}

// ForceRefresh forces a cache refresh
func (mc *MentorCache) ForceRefresh() ([]*models.Mentor, error) {
	logger.Info("Force refreshing mentor cache")
	return mc.refresh()
}

// refresh fetches mentors from Airtable and updates the cache
func (mc *MentorCache) refresh() ([]*models.Mentor, error) {
	mc.mu.Lock()

	// Check if already refreshing
	if mc.refreshing {
		mc.mu.Unlock()
		logger.Debug("Refresh already in progress, waiting for completion")

		// Wait a bit and try to get from cache
		time.Sleep(500 * time.Millisecond)
		if data, found := mc.cache.Get(mentorsCacheKey); found {
			return data.([]*models.Mentor), nil
		}
		return nil, fmt.Errorf("cache refresh in progress")
	}

	mc.refreshing = true
	mc.mu.Unlock()

	defer func() {
		mc.mu.Lock()
		mc.refreshing = false
		mc.mu.Unlock()
	}()

	// Fetch from Airtable
	mentors, err := mc.airtableClient.GetAllMentors()
	if err != nil {
		logger.Error("Failed to refresh mentor cache", zap.Error(err))
		return nil, err
	}

	// Update cache
	mc.cache.Set(mentorsCacheKey, mentors, mentorsCacheTTL)
	metrics.CacheSize.WithLabelValues(mentorsCacheName).Set(float64(len(mentors)))

	logger.Info("Mentor cache refreshed", zap.Int("count", len(mentors)))

	return mentors, nil
}

// warmUp performs initial cache population
func (mc *MentorCache) warmUp() {
	logger.Info("Warming up mentor cache")
	_, err := mc.refresh()
	if err != nil {
		logger.Error("Failed to warm up cache", zap.Error(err))
	}
}

// Clear clears the cache
func (mc *MentorCache) Clear() {
	mc.cache.Delete(mentorsCacheKey)
	logger.Info("Mentor cache cleared")
}
