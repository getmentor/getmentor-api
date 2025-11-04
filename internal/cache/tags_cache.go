package cache

import (
	"time"

	"github.com/getmentor/getmentor-api/pkg/airtable"
	"github.com/getmentor/getmentor-api/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

const (
	tagsCacheKey = "tags"
	tagsCacheTTL = 24 * time.Hour
)

// TagsCache manages the in-memory cache for tags
type TagsCache struct {
	cache          *gocache.Cache
	airtableClient *airtable.Client
}

// NewTagsCache creates a new tags cache
func NewTagsCache(airtableClient *airtable.Client) *TagsCache {
	cache := gocache.New(tagsCacheTTL, time.Hour)

	tc := &TagsCache{
		cache:          cache,
		airtableClient: airtableClient,
	}

	// Initial population
	go tc.warmUp()

	return tc
}

// Get retrieves tags from cache or fetches them if cache miss
func (tc *TagsCache) Get() (map[string]string, error) {
	// Check cache
	if data, found := tc.cache.Get(tagsCacheKey); found {
		logger.Debug("Tags cache hit")
		return data.(map[string]string), nil
	}

	logger.Info("Tags cache miss, fetching from Airtable")

	// Cache miss, fetch and populate
	return tc.refresh()
}

// refresh fetches tags from Airtable and updates the cache
func (tc *TagsCache) refresh() (map[string]string, error) {
	tags, err := tc.airtableClient.GetAllTags()
	if err != nil {
		logger.Error("Failed to refresh tags cache", zap.Error(err))
		return nil, err
	}

	// Update cache
	tc.cache.Set(tagsCacheKey, tags, tagsCacheTTL)

	logger.Info("Tags cache refreshed", zap.Int("count", len(tags)))

	return tags, nil
}

// warmUp performs initial cache population
func (tc *TagsCache) warmUp() {
	logger.Info("Warming up tags cache")
	_, err := tc.refresh()
	if err != nil {
		logger.Error("Failed to warm up tags cache", zap.Error(err))
	}
}

// GetTagIDByName gets a single tag ID by name
func (tc *TagsCache) GetTagIDByName(name string) (string, error) {
	tags, err := tc.Get()
	if err != nil {
		return "", err
	}

	if id, found := tags[name]; found {
		return id, nil
	}

	return "", nil
}
