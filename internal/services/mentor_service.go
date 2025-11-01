package services

import (
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

func (s *MentorService) GetAllMentors(opts models.FilterOptions) ([]*models.Mentor, error) {
	return s.repo.GetAll(opts)
}

func (s *MentorService) GetMentorByID(id int, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetByID(id, opts)
}

func (s *MentorService) GetMentorBySlug(slug string, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetBySlug(slug, opts)
}

func (s *MentorService) GetMentorByRecordID(recordID string, opts models.FilterOptions) (*models.Mentor, error) {
	return s.repo.GetByRecordID(recordID, opts)
}
