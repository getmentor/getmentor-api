package services_test

import (
	"context"
	"errors"
	"net/http" // Import http for http.Response
	"testing"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWebhookService_HandleAirtableWebhook(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockHTTPClient := new(MockHTTPClient)
	cfg := &config.Config{
		NextJS: config.NextJSConfig{
			BaseURL:          "http://localhost:3000",
			RevalidateSecret: "test-secret",
		},
	}
	// Inject the mock HTTP client
	service := services.NewWebhookService(mockMentorRepo, cfg)
	service.(*services.WebhookService).HTTPClient.Transport = mockHTTPClient

	ctx := context.Background()

	payload := &models.WebhookPayload{
		RecordID: "recABC",
	}

	mentor := &models.Mentor{
		AirtableID: "recABC",
		Slug:       "test-mentor",
	}

	mockMentorRepo.On("InvalidateCache").Return().Once()
	mockMentorRepo.On("GetByRecordID", ctx, "recABC", models.FilterOptions{}).Return(mentor, nil).Once()

	// Mock the HTTP call for revalidateNextJS
	mockHTTPClient.On("RoundTrip", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil).Once()

	err := service.HandleAirtableWebhook(ctx, payload)
	assert.NoError(t, err)

	mockMentorRepo.AssertExpectations(t)
	mockHTTPClient.AssertExpectations(t)
}

func TestWebhookService_HandleAirtableWebhook_MentorNotFound(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockHTTPClient := new(MockHTTPClient) // Also need a mock HTTP client here
	cfg := &config.Config{
		NextJS: config.NextJSConfig{
			BaseURL:          "http://localhost:3000",
			RevalidateSecret: "test-secret",
		},
	}
	service := services.NewWebhookService(mockMentorRepo, cfg)
	service.(*services.WebhookService).HTTPClient.Transport = mockHTTPClient // Inject mock
	ctx := context.Background()

	payload := &models.WebhookPayload{
		RecordID: "recABC",
	}

	mockMentorRepo.On("InvalidateCache").Return().Once()
	mockMentorRepo.On("GetByRecordID", ctx, "recABC", models.FilterOptions{}).Return(nil, errors.New("not found")).Once()

	err := service.HandleAirtableWebhook(ctx, payload)
	assert.NoError(t, err) // Should not return an error, just log a warning

	mockMentorRepo.AssertExpectations(t)
	mockHTTPClient.AssertNotCalled(t, "RoundTrip") // Ensure no HTTP call is made
}

func TestWebhookService_RevalidateNextJSManual(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockHTTPClient := new(MockHTTPClient) // Also need a mock HTTP client here
	cfg := &config.Config{
		Auth: config.AuthConfig{
			RevalidateSecret: "super-secret",
		},
		NextJS: config.NextJSConfig{
			BaseURL:          "http://localhost:3000",
			RevalidateSecret: "test-secret", // This is actually used by the revalidateNextJS internal call
		},
	}
	service := services.NewWebhookService(mockMentorRepo, cfg)
	service.(*services.WebhookService).HTTPClient.Transport = mockHTTPClient // Inject mock
	ctx := context.Background()

	// Mock the HTTP call for revalidateNextJS
	mockHTTPClient.On("RoundTrip", mock.AnythingOfType("*http.Request")).Return(&http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil).Once()

	err := service.RevalidateNextJSManual(ctx, "test-slug", "super-secret")
	assert.NoError(t, err)

	mockMentorRepo.AssertExpectations(t)
	mockHTTPClient.AssertExpectations(t)
}

func TestWebhookService_RevalidateNextJSManual_InvalidSecret(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockHTTPClient := new(MockHTTPClient) // Also need a mock HTTP client here
	cfg := &config.Config{
		Auth: config.AuthConfig{
			RevalidateSecret: "super-secret",
		},
	}
	service := services.NewWebhookService(mockMentorRepo, cfg)
	service.(*services.WebhookService).HTTPClient.Transport = mockHTTPClient // Inject mock
	ctx := context.Background()

	err := service.RevalidateNextJSManual(ctx, "test-slug", "wrong-secret")
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid secret")

	mockMentorRepo.AssertExpectations(t)
	mockHTTPClient.AssertNotCalled(t, "RoundTrip") // Ensure no HTTP call is made
}