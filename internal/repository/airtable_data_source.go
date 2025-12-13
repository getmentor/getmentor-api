package repository

import (
	"context"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/airtable"
)

// AirtableMentorDataSource implements MentorDataSource using Airtable
type AirtableMentorDataSource struct {
	client *airtable.Client
}

// NewAirtableMentorDataSource creates a new Airtable mentor data source
func NewAirtableMentorDataSource(client *airtable.Client) *AirtableMentorDataSource {
	return &AirtableMentorDataSource{
		client: client,
	}
}

// GetAllMentors fetches all mentors from Airtable
func (ds *AirtableMentorDataSource) GetAllMentors(_ context.Context) ([]*models.Mentor, error) {
	return ds.client.GetAllMentors()
}

// GetMentorBySlug fetches a single mentor by slug from Airtable
func (ds *AirtableMentorDataSource) GetMentorBySlug(_ context.Context, slug string) (*models.Mentor, error) {
	return ds.client.GetMentorBySlug(slug)
}

// UpdateMentor updates mentor fields in Airtable
func (ds *AirtableMentorDataSource) UpdateMentor(_ context.Context, recordID string, updates map[string]interface{}) error {
	return ds.client.UpdateMentor(recordID, updates)
}

// UpdateMentorImage updates a mentor's profile image in Airtable
func (ds *AirtableMentorDataSource) UpdateMentorImage(_ context.Context, recordID string, imageURL string) error {
	return ds.client.UpdateMentorImage(recordID, imageURL)
}

// Ensure AirtableMentorDataSource implements MentorDataSource
var _ MentorDataSource = (*AirtableMentorDataSource)(nil)

// AirtableTagsDataSource implements TagsDataSource using Airtable
type AirtableTagsDataSource struct {
	client *airtable.Client
}

// NewAirtableTagsDataSource creates a new Airtable tags data source
func NewAirtableTagsDataSource(client *airtable.Client) *AirtableTagsDataSource {
	return &AirtableTagsDataSource{
		client: client,
	}
}

// GetAllTags fetches all tags from Airtable
func (ds *AirtableTagsDataSource) GetAllTags(_ context.Context) (map[string]string, error) {
	return ds.client.GetAllTags()
}

// GetTagIDByName fetches a single tag ID by name from Airtable
func (ds *AirtableTagsDataSource) GetTagIDByName(_ context.Context, name string) (string, error) {
	tags, err := ds.client.GetAllTags()
	if err != nil {
		return "", err
	}
	return tags[name], nil
}

// Ensure AirtableTagsDataSource implements TagsDataSource
var _ TagsDataSource = (*AirtableTagsDataSource)(nil)

// AirtableClientRequestDataSource implements ClientRequestDataSource using Airtable
type AirtableClientRequestDataSource struct {
	client *airtable.Client
}

// NewAirtableClientRequestDataSource creates a new Airtable client request data source
func NewAirtableClientRequestDataSource(client *airtable.Client) *AirtableClientRequestDataSource {
	return &AirtableClientRequestDataSource{
		client: client,
	}
}

// Create creates a new client request in Airtable
func (ds *AirtableClientRequestDataSource) Create(_ context.Context, req *models.ClientRequest) error {
	return ds.client.CreateClientRequest(req)
}

// Ensure AirtableClientRequestDataSource implements ClientRequestDataSource
var _ ClientRequestDataSource = (*AirtableClientRequestDataSource)(nil)
