package services

import (
	"context"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/internal/repository"
)

type MentorService struct {
	repo   *repository.MentorRepository
	config *config.Config
}

func NewMentorService(repo *repository.MentorRepository, cfg *config.Config) *MentorService {
	return &MentorService{
		repo:   repo,
		config: cfg,
	}
}

func (s *MentorService) GetAllMentors(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error) {
	return s.repo.GetAll(ctx, opts)
}

func (s *MentorService) GetMentorByID(ctx context.Context, id int, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetByID(ctx, id, opts)
}

func (s *MentorService) GetMentorBySlug(ctx context.Context, slug string, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetBySlug(ctx, slug, opts)
}

func (s *MentorService) GetMentorByRecordID(ctx context.Context, recordID string, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetByRecordID(ctx, recordID, opts)
}
