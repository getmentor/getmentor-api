package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestContactService_SubmitContactForm(t *testing.T) {
	mockClientRequestRepo := new(MockClientRequestRepository)
	mockMentorRepo := new(MockMentorRepository)
	cfg := &config.Config{
		Server: config.ServerConfig{
			AppEnv: "development",
		},
		ReCAPTCHA: config.ReCAPTCHAConfig{
			SecretKey: "test-secret",
		},
	}
	service := services.NewContactService(mockClientRequestRepo, mockMentorRepo, cfg)
	ctx := context.Background()

	contactReq := &models.ContactMentorRequest{
		MentorAirtableID: "recABC",
		RecaptchaToken:   "test-token",
		Email:            "test@example.com",
		Name:             "Test User",
		Experience:       "1-3 years",
		Intro:            "Hello",
		TelegramUsername: "testuser",
	}

	expectedMentor := &models.Mentor{
		AirtableID:  "recABC",
		CalendarURL: "https://calendly.com/test",
	}

	// In development mode, Create is skipped, so we don't set an expectation for it.
	mockMentorRepo.On("GetByRecordID", ctx, "recABC", models.FilterOptions{ShowHidden: true}).Return(expectedMentor, nil).Once()

	resp, err := service.SubmitContactForm(ctx, contactReq)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "https://calendly.com/test", resp.CalendarURL)

	mockClientRequestRepo.AssertExpectations(t)
	mockMentorRepo.AssertExpectations(t)
}

func TestContactService_SubmitContactForm_CaptchaError(t *testing.T) {
	mockClientRequestRepo := new(MockClientRequestRepository)
	mockMentorRepo := new(MockMentorRepository)
	cfg := &config.Config{
		Server: config.ServerConfig{
			AppEnv: "production",
		},
		ReCAPTCHA: config.ReCAPTCHAConfig{
			SecretKey: "test-secret",
		},
	}
	service := services.NewContactService(mockClientRequestRepo, mockMentorRepo, cfg)
	ctx := context.Background()

	contactReq := &models.ContactMentorRequest{
		MentorAirtableID: "recABC",
		RecaptchaToken:   "any-token", // In production, this will make a real HTTP call, which will fail
	}

	resp, err := service.SubmitContactForm(ctx, contactReq)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Captcha verification failed", resp.Error)

	mockClientRequestRepo.AssertNotCalled(t, "Create")
	mockMentorRepo.AssertNotCalled(t, "GetByRecordID")
}

func TestContactService_SubmitContactForm_CreateError(t *testing.T) {
	mockClientRequestRepo := new(MockClientRequestRepository)
	mockMentorRepo := new(MockMentorRepository)
	cfg := &config.Config{
		Server: config.ServerConfig{
			AppEnv: "production", // Set to production to test the Create path
		},
	}
	service := services.NewContactService(mockClientRequestRepo, mockMentorRepo, cfg)
	ctx := context.Background()

	contactReq := &models.ContactMentorRequest{
		MentorAirtableID: "recABC",
		RecaptchaToken:   "test-token", // Use the bypass token
	}

	clientReq := &models.ClientRequest{
		MentorID: contactReq.MentorAirtableID,
	}

	mockError := errors.New("airtable error")

	mockClientRequestRepo.On("Create", ctx, clientReq).Return(mockError).Once()

	resp, err := service.SubmitContactForm(ctx, contactReq)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.Success)
	assert.Equal(t, "Failed to save contact request", resp.Error)

	mockClientRequestRepo.AssertExpectations(t)
	mockMentorRepo.AssertNotCalled(t, "GetByRecordID")
}
