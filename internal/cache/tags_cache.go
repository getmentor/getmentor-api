package cache

import (
	"context"
	"time"

	"github.com/getmentor/getmentor-api/pkg/airtable"
	"github.com/getmentor/getmentor-api/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

// TagsCacheInterface defines the interface for tags cache operations.
type TagsCacheInterface interface {
	Get(ctx context.Context) (map[string]string, error)
	GetTagIDByName(ctx context.Context, name string) (string, error)
}

const (
	tagsCacheKey = "tags"
	tagsCacheTTL = 24 * time.Hour
)

// TagsCache manages the in-memory cache for tags
type TagsCache struct {
	cache          *gocache.Cache
	airtableClient airtable.ClientInterface
}

// NewTagsCache creates a new tags cache
func NewTagsCache(airtableClient airtable.ClientInterface) TagsCacheInterface {
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
func (tc *TagsCache) Get(ctx context.Context) (map[string]string, error) {
	// Check cache
	if data, found := tc.cache.Get(tagsCacheKey); found {
		logger.Debug("Tags cache hit")
		return data.(map[string]string), nil
	}

	logger.Info("Tags cache miss, fetching from Airtable")

	// Cache miss, fetch and populate
	return tc.refresh(ctx)
}

// refresh fetches tags from Airtable and updates the cache
func (tc *TagsCache) refresh(ctx context.Context) (map[string]string, error) {
	tags, err := tc.airtableClient.GetAllTags(ctx)
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
	_, err := tc.refresh(context.Background())
	if err != nil {
		logger.Error("Failed to warm up tags cache", zap.Error(err))
	}
}

// GetTagIDByName gets a single tag ID by name
func (tc *TagsCache) GetTagIDByName(ctx context.Context, name string) (string, error) {
	tags, err := tc.Get(ctx)
	if err != nil {
		return "", err
	}

	if id, found := tags[name]; found {
		return id, nil
	}

	return "", nil
}
