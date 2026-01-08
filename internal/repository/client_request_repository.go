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
// Returns: recordID (Airtable rec*), error
func (r *ClientRequestRepository) Create(ctx context.Context, req *models.ClientRequest) (string, error) {
	return r.airtableClient.CreateClientRequest(ctx, req)
}

// GetByMentor retrieves all client requests for a mentor filtered by statuses
func (r *ClientRequestRepository) GetByMentor(ctx context.Context, mentorAirtableID string, statuses []models.RequestStatus) ([]*models.MentorClientRequest, error) {
	return r.airtableClient.GetClientRequestsByMentor(ctx, mentorAirtableID, statuses)
}

// GetByID retrieves a single client request by ID
func (r *ClientRequestRepository) GetByID(ctx context.Context, id string) (*models.MentorClientRequest, error) {
	return r.airtableClient.GetClientRequestByID(ctx, id)
}

// UpdateStatus updates the status of a client request
func (r *ClientRequestRepository) UpdateStatus(ctx context.Context, id string, status models.RequestStatus) error {
	return r.airtableClient.UpdateClientRequestStatus(ctx, id, status)
}

// UpdateDecline updates a client request with decline info
func (r *ClientRequestRepository) UpdateDecline(ctx context.Context, id string, reason models.DeclineReason, comment string) error {
	return r.airtableClient.UpdateClientRequestDecline(ctx, id, reason, comment)
}
