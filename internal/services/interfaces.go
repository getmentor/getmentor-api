package services

import (
	"github.com/getmentor/getmentor-api/internal/models"
)

// ContactServiceInterface defines the interface for contact service operations
type ContactServiceInterface interface {
	SubmitContactForm(req *models.ContactMentorRequest) (*models.ContactMentorResponse, error)
}

// Ensure ContactService implements the interface
var _ ContactServiceInterface = (*ContactService)(nil)
