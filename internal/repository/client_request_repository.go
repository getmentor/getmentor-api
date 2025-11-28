package repository

import (
	"context"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/airtable"
)

// ClientRequestRepository handles client request data access
type ClientRequestRepository struct {
	airtableClient *airtable.Client
}

// NewClientRequestRepository creates a new client request repository
func NewClientRequestRepository(airtableClient *airtable.Client) *ClientRequestRepository {
	return &ClientRequestRepository{
		airtableClient: airtableClient,
	}
}

// Create creates a new client request in Airtable
func (r *ClientRequestRepository) Create(ctx context.Context, req *models.ClientRequest) error {
	return r.airtableClient.CreateClientRequest(req)
}
