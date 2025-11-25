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

func TestMentorService_GetAllMentors(t *testing.T) {
	mockRepo := new(MockMentorRepository)
	cfg := &config.Config{} // Mock config if needed, or use a default one
	service := services.NewMentorService(mockRepo, cfg)
	ctx := context.Background()

	expectedMentors := []*models.Mentor{
		{ID: 1, Name: "Mentor 1"},
		{ID: 2, Name: "Mentor 2"},
	}
	filterOpts := models.FilterOptions{OnlyVisible: true}

	mockRepo.On("GetAll", ctx, filterOpts).Return(expectedMentors, nil).Once()

	mentors, err := service.GetAllMentors(ctx, filterOpts)
	assert.NoError(t, err)
	assert.Equal(t, expectedMentors, mentors)
	mockRepo.AssertExpectations(t)
}

func TestMentorService_GetAllMentors_Error(t *testing.T) {
	mockRepo := new(MockMentorRepository)
	cfg := &config.Config{}
	service := services.NewMentorService(mockRepo, cfg)
	ctx := context.Background()

	mockError := errors.New("repository error")
	filterOpts := models.FilterOptions{OnlyVisible: true}

	mockRepo.On("GetAll", ctx, filterOpts).Return(nil, mockError).Once()

	mentors, err := service.GetAllMentors(ctx, filterOpts)
	assert.Error(t, err)
	assert.Nil(t, mentors)
	assert.EqualError(t, err, mockError.Error())
	mockRepo.AssertExpectations(t)
}

func TestMentorService_GetMentorByID(t *testing.T) {
	mockRepo := new(MockMentorRepository)
	cfg := &config.Config{}
	service := services.NewMentorService(mockRepo, cfg)
	ctx := context.Background()

	expectedMentor := &models.Mentor{ID: 1, Name: "Mentor 1"}
	filterOpts := models.FilterOptions{OnlyVisible: true}

	mockRepo.On("GetByID", ctx, 1, filterOpts).Return(expectedMentor, nil).Once()

	mentor, err := service.GetMentorByID(ctx, 1, filterOpts)
	assert.NoError(t, err)
	assert.Equal(t, expectedMentor, mentor)
	mockRepo.AssertExpectations(t)
}

func TestMentorService_GetMentorByID_NotFound(t *testing.T) {
	mockRepo := new(MockMentorRepository)
	cfg := &config.Config{}
	service := services.NewMentorService(mockRepo, cfg)
	ctx := context.Background()

	mockError := errors.New("mentor not found")
	filterOpts := models.FilterOptions{OnlyVisible: true}

	mockRepo.On("GetByID", ctx, 999, filterOpts).Return(nil, mockError).Once()

	mentor, err := service.GetMentorByID(ctx, 999, filterOpts)
	assert.Error(t, err)
	assert.Nil(t, mentor)
	assert.EqualError(t, err, mockError.Error())
	mockRepo.AssertExpectations(t)
}

func TestMentorService_GetMentorBySlug(t *testing.T) {
	mockRepo := new(MockMentorRepository)
	cfg := &config.Config{}
	service := services.NewMentorService(mockRepo, cfg)
	ctx := context.Background()

	expectedMentor := &models.Mentor{ID: 1, Slug: "mentor-1"}
	filterOpts := models.FilterOptions{OnlyVisible: true}

	mockRepo.On("GetBySlug", ctx, "mentor-1", filterOpts).Return(expectedMentor, nil).Once()

	mentor, err := service.GetMentorBySlug(ctx, "mentor-1", filterOpts)
	assert.NoError(t, err)
	assert.Equal(t, expectedMentor, mentor)
	mockRepo.AssertExpectations(t)
}

func TestMentorService_GetMentorByRecordID(t *testing.T) {
	mockRepo := new(MockMentorRepository)
	cfg := &config.Config{}
	service := services.NewMentorService(mockRepo, cfg)
	ctx := context.Background()

	expectedMentor := &models.Mentor{ID: 1, AirtableID: "recABC"}
	filterOpts := models.FilterOptions{OnlyVisible: true}

	mockRepo.On("GetByRecordID", ctx, "recABC", filterOpts).Return(expectedMentor, nil).Once()

	mentor, err := service.GetMentorByRecordID(ctx, "recABC", filterOpts)
	assert.NoError(t, err)
	assert.Equal(t, expectedMentor, mentor)
	mockRepo.AssertExpectations(t)
}
