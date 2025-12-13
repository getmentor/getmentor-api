package repository

import (
	"context"

	"github.com/getmentor/getmentor-api/internal/models"
)

// MentorDataSource defines the interface for mentor data fetching
// This allows switching between Airtable and PostgreSQL implementations
type MentorDataSource interface {
	// GetAllMentors fetches all mentors
	GetAllMentors(ctx context.Context) ([]*models.Mentor, error)

	// GetMentorBySlug fetches a single mentor by slug
	GetMentorBySlug(ctx context.Context, slug string) (*models.Mentor, error)

	// UpdateMentor updates mentor fields
	UpdateMentor(ctx context.Context, recordID string, updates map[string]interface{}) error

	// UpdateMentorImage updates a mentor's profile image
	UpdateMentorImage(ctx context.Context, recordID string, imageURL string) error
}

// TagsDataSource defines the interface for tags data fetching
type TagsDataSource interface {
	// GetAllTags fetches all tags (name -> ID mapping)
	GetAllTags(ctx context.Context) (map[string]string, error)

	// GetTagIDByName fetches a single tag ID by name
	GetTagIDByName(ctx context.Context, name string) (string, error)
}

// ClientRequestDataSource defines the interface for client request operations
type ClientRequestDataSource interface {
	// Create creates a new client request
	Create(ctx context.Context, req *models.ClientRequest) error
}
