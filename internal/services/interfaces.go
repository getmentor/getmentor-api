package services

import (
	"context"

	"github.com/getmentor/getmentor-api/internal/models"
)

// ContactServiceInterface defines the interface for contact service operations
type ContactServiceInterface interface {
	SubmitContactForm(ctx context.Context, req *models.ContactMentorRequest) (*models.ContactMentorResponse, error)
}

// MentorServiceInterface defines the interface for mentor service operations
type MentorServiceInterface interface {
	GetAllMentors(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error)
	GetMentorByID(ctx context.Context, id int, opts models.FilterOptions) (*models.Mentor, error)
	GetMentorBySlug(ctx context.Context, slug string, opts models.FilterOptions) (*models.Mentor, error)
	GetMentorByRecordID(ctx context.Context, recordID string, opts models.FilterOptions) (*models.Mentor, error)
}

// ProfileServiceInterface defines the interface for profile service operations
type ProfileServiceInterface interface {
	SaveProfile(ctx context.Context, id int, token string, req *models.SaveProfileRequest) error
	UploadProfilePicture(ctx context.Context, id int, token string, req *models.UploadProfilePictureRequest) (string, error)
}

// WebhookServiceInterface defines the interface for webhook service operations
type WebhookServiceInterface interface {
	HandleAirtableWebhook(ctx context.Context, payload *models.WebhookPayload) error
}

// Ensure services implement their interfaces
var _ ContactServiceInterface = (*ContactService)(nil)
var _ MentorServiceInterface = (*MentorService)(nil)
var _ ProfileServiceInterface = (*ProfileService)(nil)
var _ WebhookServiceInterface = (*WebhookService)(nil)
