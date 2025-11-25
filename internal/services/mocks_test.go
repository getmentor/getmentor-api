package services_test

import (
	"context"
	"net/http"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/stretchr/testify/mock"
)

// MockMentorRepository is a mock implementation of MentorRepositoryInterface
type MockMentorRepository struct {
	mock.Mock
}

func (m *MockMentorRepository) GetAll(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Mentor), args.Error(1)
}

func (m *MockMentorRepository) GetByID(ctx context.Context, id int, opts models.FilterOptions) (*models.Mentor, error) {
	args := m.Called(ctx, id, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Mentor), args.Error(1)
}

func (m *MockMentorRepository) GetBySlug(ctx context.Context, slug string, opts models.FilterOptions) (*models.Mentor, error) {
	args := m.Called(ctx, slug, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Mentor), args.Error(1)
}

func (m *MockMentorRepository) GetByRecordID(ctx context.Context, recordID string, opts models.FilterOptions) (*models.Mentor, error) {
	args := m.Called(ctx, recordID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Mentor), args.Error(1)
}

func (m *MockMentorRepository) Update(ctx context.Context, recordID string, updates map[string]interface{}) error {
	args := m.Called(ctx, recordID, updates)
	return args.Error(0)
}

func (m *MockMentorRepository) UpdateImage(ctx context.Context, recordID, imageURL string) error {
	args := m.Called(ctx, recordID, imageURL)
	return args.Error(0)
}

func (m *MockMentorRepository) GetTagIDByName(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockMentorRepository) GetAllTags(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockMentorRepository) InvalidateCache() {
	m.Called()
}

// MockClientRequestRepository is a mock implementation of ClientRequestRepositoryInterface
type MockClientRequestRepository struct {
	mock.Mock
}

func (m *MockClientRequestRepository) Create(ctx context.Context, req *models.ClientRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

// MockStorageClient is a mock implementation of azure.StorageClientInterface
type MockStorageClient struct {
	mock.Mock
}

func (m *MockStorageClient) UploadImage(ctx context.Context, imageData, fileName, contentType string) (string, error) {
	args := m.Called(ctx, imageData, fileName, contentType)
	return args.String(0), args.Error(1)
}

func (m *MockStorageClient) GenerateFileName(mentorID int, originalFileName string) string {
	args := m.Called(mentorID, originalFileName)
	return args.String(0)
}

func (m *MockStorageClient) ValidateImageType(contentType string) error {
	args := m.Called(contentType)
	return args.Error(0)
}

func (m *MockStorageClient) ValidateImageSize(imageData string) error {
	args := m.Called(imageData)
	return args.Error(0)
}

// MockHTTPClient is a mock implementation of http.RoundTripper to mock HTTP calls
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}