package repository

import (
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
func (r *MentorRepository) GetAll(opts models.FilterOptions) ([]*models.Mentor, error) {
	var mentors []*models.Mentor
	var err error

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
func (r *MentorRepository) GetByID(id int, opts models.FilterOptions) (*models.Mentor, error) {
	mentors, err := r.GetAll(opts)
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

// GetBySlug retrieves a mentor by slug
func (r *MentorRepository) GetBySlug(slug string, opts models.FilterOptions) (*models.Mentor, error) {
	mentors, err := r.GetAll(opts)
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.Slug == slug {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with slug %s not found", slug)
}

// GetByRecordID retrieves a mentor by Airtable record ID
func (r *MentorRepository) GetByRecordID(recordID string, opts models.FilterOptions) (*models.Mentor, error) {
	mentors, err := r.GetAll(opts)
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
func (r *MentorRepository) Update(recordID string, updates map[string]interface{}) error {
	err := r.airtableClient.UpdateMentor(recordID, updates)
	if err != nil {
		return err
	}

	// Note: Cache will auto-refresh after TTL expires
	return nil
}

// UpdateImage updates a mentor's profile image
func (r *MentorRepository) UpdateImage(recordID, imageURL string) error {
	return r.airtableClient.UpdateMentorImage(recordID, imageURL)
}

// GetTagIDByName retrieves a tag ID by name
func (r *MentorRepository) GetTagIDByName(name string) (string, error) {
	return r.tagsCache.GetTagIDByName(name)
}

// GetAllTags retrieves all tags
func (r *MentorRepository) GetAllTags() (map[string]string, error) {
	return r.tagsCache.Get()
}

// applyFilters applies filtering options to a mentor list
func (r *MentorRepository) applyFilters(mentors []*models.Mentor, opts models.FilterOptions) []*models.Mentor {
	result := make([]*models.Mentor, 0)

	for _, mentor := range mentors {
		// Filter by visibility
		if opts.OnlyVisible && !mentor.IsVisible {
			continue
		}

		// Make a copy to avoid modifying cached data
		m := *mentor

		// Drop long fields if requested
		if opts.DropLongFields {
			m.About = ""
			m.Description = ""
		}

		// Hide secure fields unless explicitly requested
		if !opts.ShowHidden {
			m.AuthToken = ""
			m.CalendarURL = ""
		}

		result = append(result, &m)
	}

	return result
}

// InvalidateCache forces cache invalidation
func (r *MentorRepository) InvalidateCache() {
	r.mentorCache.Clear()
}
