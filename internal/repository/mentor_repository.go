package repository

import (
	"context"
	"fmt"

	"github.com/getmentor/getmentor-api/internal/cache"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/airtable"
)

// MentorRepository handles mentor data access
type MentorRepository struct {
	airtableClient *airtable.Client
	mentorCache    *cache.MentorCache
	tagsCache      *cache.TagsCache
}

// NewMentorRepository creates a new mentor repository
func NewMentorRepository(airtableClient *airtable.Client, mentorCache *cache.MentorCache, tagsCache *cache.TagsCache) *MentorRepository {
	return &MentorRepository{
		airtableClient: airtableClient,
		mentorCache:    mentorCache,
		tagsCache:      tagsCache,
	}
}

// GetAll retrieves all mentors with optional filtering
func (r *MentorRepository) GetAll(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error) {
	var mentors []*models.Mentor
	var err error

	// ForceRefresh triggers background refresh but returns current data
	if opts.ForceRefresh {
		mentors, err = r.mentorCache.ForceRefresh()
	} else {
		mentors, err = r.mentorCache.Get()
	}

	if err != nil {
		return nil, err
	}

	// Apply filters
	filtered := r.applyFilters(mentors, opts)

	return filtered, nil
}

// GetByID retrieves a mentor by numeric ID
// Note: O(n) complexity is acceptable as per requirements
func (r *MentorRepository) GetByID(ctx context.Context, id int, opts models.FilterOptions) (*models.Mentor, error) {
	mentors, err := r.GetAll(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.ID == id {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with ID %d not found", id)
}

// GetBySlug retrieves a mentor by slug with O(1) complexity
func (r *MentorRepository) GetBySlug(ctx context.Context, slug string, opts models.FilterOptions) (*models.Mentor, error) {
	// Note: ForceRefresh is ignored for single lookups
	// Only webhook/profile updates trigger single-mentor refresh

	mentor, err := r.mentorCache.GetBySlug(slug)
	if err != nil {
		return nil, err
	}

	// Apply filters to single mentor
	filtered := r.applySingleMentorFilters(mentor, opts)
	if filtered == nil {
		return nil, fmt.Errorf("mentor with slug %s not found or not visible", slug)
	}

	return filtered, nil
}

// GetByRecordID retrieves a mentor by Airtable record ID
func (r *MentorRepository) GetByRecordID(ctx context.Context, recordID string, opts models.FilterOptions) (*models.Mentor, error) {
	mentors, err := r.GetAll(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.AirtableID == recordID {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with record ID %s not found", recordID)
}

// Update updates a mentor in Airtable
func (r *MentorRepository) Update(ctx context.Context, recordID string, updates map[string]interface{}) error {
	err := r.airtableClient.UpdateMentor(recordID, updates)
	if err != nil {
		return err
	}

	// Note: Cache will auto-refresh after TTL expires
	return nil
}

// UpdateImage updates a mentor's profile image
func (r *MentorRepository) UpdateImage(ctx context.Context, recordID, imageURL string) error {
	return r.airtableClient.UpdateMentorImage(recordID, imageURL)
}

// CreateMentor creates a new mentor record in Airtable
// Returns: recordID (Airtable rec*), mentorID (numeric ID), error
func (r *MentorRepository) CreateMentor(ctx context.Context, fields map[string]interface{}) (string, int, error) {
	return r.airtableClient.CreateMentor(fields)
}

// GetTagIDByName retrieves a tag ID by name
func (r *MentorRepository) GetTagIDByName(ctx context.Context, name string) (string, error) {
	return r.tagsCache.GetTagIDByName(name)
}

// GetAllTags retrieves all tags
func (r *MentorRepository) GetAllTags(ctx context.Context) (map[string]string, error) {
	return r.tagsCache.Get()
}

// applyFilters applies filtering options to a mentor list
func (r *MentorRepository) applyFilters(mentors []*models.Mentor, opts models.FilterOptions) []*models.Mentor {
	result := make([]*models.Mentor, 0, len(mentors))

	for _, mentor := range mentors {
		filtered := r.applySingleMentorFilters(mentor, opts)
		if filtered != nil {
			result = append(result, filtered)
		}
	}

	return result
}

// applySingleMentorFilters applies filtering options to a single mentor
// Returns nil if mentor should be filtered out
func (r *MentorRepository) applySingleMentorFilters(mentor *models.Mentor, opts models.FilterOptions) *models.Mentor {
	// Filter by visibility
	if opts.OnlyVisible && !mentor.IsVisible {
		return nil
	}

	// Only copy if modifications are needed
	if opts.DropLongFields || !opts.ShowHidden {
		m := *mentor // Copy only when necessary

		if opts.DropLongFields {
			m.About = ""
			m.Description = ""
		}

		if !opts.ShowHidden {
			m.AuthToken = ""
			m.CalendarURL = ""
		}

		return &m
	}

	// Return original pointer if no modifications needed
	return mentor
}

// InvalidateCache forces cache invalidation
func (r *MentorRepository) InvalidateCache() {
	r.mentorCache.Clear()
}

// UpdateSingleMentorCache updates a single mentor in cache
// Called by webhook or profile update flow
func (r *MentorRepository) UpdateSingleMentorCache(slug string) error {
	return r.mentorCache.UpdateSingleMentor(slug)
}

// RemoveMentorFromCache removes a mentor from cache
// Called when a mentor is deleted
func (r *MentorRepository) RemoveMentorFromCache(slug string) error {
	return r.mentorCache.RemoveMentor(slug)
}

// RefreshCache triggers a background cache refresh
func (r *MentorRepository) RefreshCache() error {
	_, err := r.mentorCache.ForceRefresh()
	return err
}
