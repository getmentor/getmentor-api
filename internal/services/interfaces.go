package services

import (
	"context"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/jwt"
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
	GetMentorByMentorId(ctx context.Context, mentorId string, opts models.FilterOptions) (*models.Mentor, error)
}

// ProfileServiceInterface defines the interface for profile service operations
type ProfileServiceInterface interface {
	SaveProfileByMentorId(ctx context.Context, mentorId string, req *models.SaveProfileRequest) error
	UploadPictureByMentorId(ctx context.Context, mentorId string, mentorSlug string, req *models.UploadProfilePictureRequest) (string, error)
}

// RegistrationServiceInterface defines the interface for registration service operations
type RegistrationServiceInterface interface {
	RegisterMentor(ctx context.Context, req *models.RegisterMentorRequest) (*models.RegisterMentorResponse, error)
}

// MentorAuthServiceInterface defines the interface for mentor authentication
type MentorAuthServiceInterface interface {
	RequestLogin(ctx context.Context, email string) (*models.RequestLoginResponse, error)
	VerifyLogin(ctx context.Context, token string) (*models.MentorSession, string, error)
	GetSessionTTL() int
	GetCookieDomain() string
	GetCookieSecure() bool
	GetTokenManager() *jwt.TokenManager
}

// MentorRequestsServiceInterface defines the interface for mentor request management
type MentorRequestsServiceInterface interface {
	GetRequests(ctx context.Context, mentorId string, group string) (*models.ClientRequestsResponse, error)
	GetRequestByID(ctx context.Context, mentorId string, requestID string) (*models.MentorClientRequest, error)
	UpdateStatus(ctx context.Context, mentorId string, requestID string, newStatus models.RequestStatus) (*models.MentorClientRequest, error)
	DeclineRequest(ctx context.Context, mentorId string, requestID string, payload *models.DeclineRequestPayload) (*models.MentorClientRequest, error)
}

// Ensure services implement their interfaces
var _ ContactServiceInterface = (*ContactService)(nil)
var _ MentorServiceInterface = (*MentorService)(nil)
var _ ProfileServiceInterface = (*ProfileService)(nil)
var _ RegistrationServiceInterface = (*RegistrationService)(nil)
var _ MentorAuthServiceInterface = (*MentorAuthService)(nil)
var _ MentorRequestsServiceInterface = (*MentorRequestsService)(nil)
