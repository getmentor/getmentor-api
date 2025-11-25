package services_test

import (
	"context"
	"sync"
	"testing"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProfileService_SaveProfile(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockAzureClient := new(MockStorageClient)
	cfg := &config.Config{}
	service := services.NewProfileService(mockMentorRepo, mockAzureClient, cfg)
	ctx := context.Background()

	saveReq := &models.SaveProfileRequest{
		Name: "Test Mentor",
		Job:  "Developer",
		Tags: []string{"Go"},
	}
	mentor := &models.Mentor{
		ID:         1,
		AirtableID: "recABC",
		AuthToken:  "test-token",
	}

	mockMentorRepo.On("GetByID", ctx, 1, models.FilterOptions{ShowHidden: true}).Return(mentor, nil).Once()
	mockMentorRepo.On("Update", ctx, "recABC", mock.Anything).Return(nil).Once()
	mockMentorRepo.On("GetTagIDByName", ctx, "Go").Return("tagID", nil)

	err := service.SaveProfile(ctx, 1, "test-token", saveReq)
	assert.NoError(t, err)

	mockMentorRepo.AssertExpectations(t)
}

func TestProfileService_SaveProfile_AuthError(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockAzureClient := new(MockStorageClient)
	cfg := &config.Config{}
	service := services.NewProfileService(mockMentorRepo, mockAzureClient, cfg)
	ctx := context.Background()

	saveReq := &models.SaveProfileRequest{}
	mentor := &models.Mentor{
		ID:         1,
		AirtableID: "recABC",
		AuthToken:  "real-token",
	}

	mockMentorRepo.On("GetByID", ctx, 1, models.FilterOptions{ShowHidden: true}).Return(mentor, nil).Once()

	err := service.SaveProfile(ctx, 1, "wrong-token", saveReq)
	assert.Error(t, err)
	assert.EqualError(t, err, "access denied")

	mockMentorRepo.AssertExpectations(t)
	mockMentorRepo.AssertNotCalled(t, "Update")
}

func TestProfileService_UploadProfilePicture(t *testing.T) {
	mockMentorRepo := new(MockMentorRepository)
	mockAzureClient := new(MockStorageClient)
	cfg := &config.Config{}
	service := services.NewProfileService(mockMentorRepo, mockAzureClient, cfg)
	ctx := context.Background()

	uploadReq := &models.UploadProfilePictureRequest{
		Image:       "base64-encoded-image",
		FileName:    "test.jpg",
		ContentType: "image/jpeg",
	}
	mentor := &models.Mentor{
		ID:         1,
		AirtableID: "recABC",
		AuthToken:  "test-token",
	}
	expectedURL := "https://example.com/test.jpg"

	var wg sync.WaitGroup
	wg.Add(1)

	mockMentorRepo.On("GetByID", ctx, 1, models.FilterOptions{ShowHidden: true}).Return(mentor, nil).Once()
	mockAzureClient.On("ValidateImageType", "image/jpeg").Return(nil).Once()
	mockAzureClient.On("ValidateImageSize", "base64-encoded-image").Return(nil).Once()
	mockAzureClient.On("GenerateFileName", 1, "test.jpg").Return("generated-filename.jpg").Once()
	mockAzureClient.On("UploadImage", ctx, "base64-encoded-image", "generated-filename.jpg", "image/jpeg").Return(expectedURL, nil).Once()
	mockMentorRepo.On("UpdateImage", mock.Anything, "recABC", expectedURL).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(nil).Once()

	imageURL, err := service.UploadProfilePicture(ctx, 1, "test-token", uploadReq)
	assert.NoError(t, err)
	assert.Equal(t, expectedURL, imageURL)

	wg.Wait()

	mockMentorRepo.AssertExpectations(t)
	mockAzureClient.AssertExpectations(t)
}