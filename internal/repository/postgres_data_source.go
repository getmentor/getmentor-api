package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/getmentor/getmentor-api/internal/database/postgres"
	"github.com/getmentor/getmentor-api/internal/models"
)

// PostgresMentorDataSource implements MentorDataSource using PostgreSQL
type PostgresMentorDataSource struct {
	client *postgres.Client
}

// NewPostgresMentorDataSource creates a new PostgreSQL mentor data source
func NewPostgresMentorDataSource(client *postgres.Client) *PostgresMentorDataSource {
	return &PostgresMentorDataSource{
		client: client,
	}
}

// GetAllMentors fetches all mentors from PostgreSQL
func (ds *PostgresMentorDataSource) GetAllMentors(ctx context.Context) ([]*models.Mentor, error) {
	return ds.client.GetAllMentors(ctx)
}

// GetMentorBySlug fetches a single mentor by slug from PostgreSQL
func (ds *PostgresMentorDataSource) GetMentorBySlug(ctx context.Context, slug string) (*models.Mentor, error) {
	return ds.client.GetMentorBySlug(ctx, slug)
}

// UpdateMentor updates mentor fields in PostgreSQL
// recordID can be either slug or airtable_id for backward compatibility
func (ds *PostgresMentorDataSource) UpdateMentor(ctx context.Context, recordID string, updates map[string]interface{}) error {
	// Try to find by slug first, then by airtable_id
	mentor, err := ds.client.GetMentorBySlug(ctx, recordID)
	if err != nil {
		mentor, err = ds.client.GetMentorByAirtableID(ctx, recordID)
		if err != nil {
			return fmt.Errorf("mentor not found: %w", err)
		}
	}

	return ds.client.UpdateMentor(ctx, mentor.Slug, updates)
}

// UpdateMentorImage updates a mentor's profile image in PostgreSQL
func (ds *PostgresMentorDataSource) UpdateMentorImage(ctx context.Context, recordID string, imageURL string) error {
	// Try to find by slug first, then by airtable_id
	mentor, err := ds.client.GetMentorBySlug(ctx, recordID)
	if err != nil {
		mentor, err = ds.client.GetMentorByAirtableID(ctx, recordID)
		if err != nil {
			return fmt.Errorf("mentor not found: %w", err)
		}
	}

	return ds.client.UpdateMentorImage(ctx, mentor.Slug, imageURL)
}

// Ensure PostgresMentorDataSource implements MentorDataSource
var _ MentorDataSource = (*PostgresMentorDataSource)(nil)

// PostgresTagsDataSource implements TagsDataSource using PostgreSQL
type PostgresTagsDataSource struct {
	client *postgres.Client
}

// NewPostgresTagsDataSource creates a new PostgreSQL tags data source
func NewPostgresTagsDataSource(client *postgres.Client) *PostgresTagsDataSource {
	return &PostgresTagsDataSource{
		client: client,
	}
}

// GetAllTags fetches all tags from PostgreSQL (name -> ID string mapping)
func (ds *PostgresTagsDataSource) GetAllTags(ctx context.Context) (map[string]string, error) {
	return ds.client.GetAllTags(ctx)
}

// GetTagIDByName fetches a single tag ID by name from PostgreSQL
func (ds *PostgresTagsDataSource) GetTagIDByName(ctx context.Context, name string) (string, error) {
	tagID, err := ds.client.GetTagIDByName(ctx, name)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(tagID), nil
}

// Ensure PostgresTagsDataSource implements TagsDataSource
var _ TagsDataSource = (*PostgresTagsDataSource)(nil)

// PostgresClientRequestDataSource implements ClientRequestDataSource using PostgreSQL
type PostgresClientRequestDataSource struct {
	client *postgres.Client
}

// NewPostgresClientRequestDataSource creates a new PostgreSQL client request data source
func NewPostgresClientRequestDataSource(client *postgres.Client) *PostgresClientRequestDataSource {
	return &PostgresClientRequestDataSource{
		client: client,
	}
}

// Create creates a new client request in PostgreSQL
func (ds *PostgresClientRequestDataSource) Create(ctx context.Context, req *models.ClientRequest) error {
	return ds.client.CreateClientRequest(ctx, req)
}

// Ensure PostgresClientRequestDataSource implements ClientRequestDataSource
var _ ClientRequestDataSource = (*PostgresClientRequestDataSource)(nil)
