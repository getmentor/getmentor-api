package repository

import (
	"context"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/airtable"
)

// ClientRequestRepositoryInterface defines the interface for client request data access operations.
type ClientRequestRepositoryInterface interface {
	Create(ctx context.Context, req *models.ClientRequest) error
}

// ClientRequestRepository handles client request data access
type ClientRequestRepository struct {
	airtableClient airtable.ClientInterface
}

// NewClientRequestRepository creates a new client request repository
func NewClientRequestRepository(airtableClient airtable.ClientInterface) ClientRequestRepositoryInterface {
	return &ClientRequestRepository{
		airtableClient: airtableClient,
	}
}

// Create creates a new client request in Airtable
func (r *ClientRequestRepository) Create(ctx context.Context, req *models.ClientRequest) error {
	return r.airtableClient.CreateClientRequest(ctx, req)
}
