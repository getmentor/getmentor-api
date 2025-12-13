package services

import (
	"context"
	"fmt"

	"github.com/getmentor/getmentor-api/internal/database/postgres"
)

// BotService handles operations for the Telegram bot
type BotService struct {
	db *postgres.Client
}

// NewBotService creates a new bot service
func NewBotService(db *postgres.Client) *BotService {
	return &BotService{
		db: db,
	}
}

// GetMentorByTgSecret finds a mentor by their TgSecret authentication code
func (s *BotService) GetMentorByTgSecret(ctx context.Context, code string) (*postgres.BotMentor, error) {
	if code == "" {
		return nil, fmt.Errorf("tg_secret code is required")
	}
	return s.db.GetMentorByTgSecret(ctx, code)
}

// GetMentorByTelegramChatID finds a mentor by their Telegram chat ID
func (s *BotService) GetMentorByTelegramChatID(ctx context.Context, chatID string) (*postgres.BotMentor, error) {
	if chatID == "" {
		return nil, fmt.Errorf("telegram chat ID is required")
	}
	return s.db.GetMentorByTelegramChatID(ctx, chatID)
}

// GetMentorByID finds a mentor by their numeric ID
func (s *BotService) GetMentorByID(ctx context.Context, mentorID int) (*postgres.BotMentor, error) {
	return s.db.GetBotMentorByID(ctx, mentorID)
}

// SetMentorTelegramChatID sets the Telegram chat ID for a mentor
func (s *BotService) SetMentorTelegramChatID(ctx context.Context, mentorID int, chatID string) error {
	if chatID == "" {
		return fmt.Errorf("telegram chat ID is required")
	}
	return s.db.SetMentorTelegramChatID(ctx, mentorID, chatID)
}

// SetMentorStatus updates the status of a mentor
func (s *BotService) SetMentorStatus(ctx context.Context, mentorID int, status string) error {
	if status == "" {
		return fmt.Errorf("status is required")
	}
	return s.db.SetMentorStatus(ctx, mentorID, status)
}

// GetActiveRequestsForMentor returns active requests for a mentor
func (s *BotService) GetActiveRequestsForMentor(ctx context.Context, mentorID int) ([]*postgres.BotClientRequest, error) {
	return s.db.GetActiveRequestsForMentor(ctx, mentorID)
}

// GetArchivedRequestsForMentor returns archived requests for a mentor
func (s *BotService) GetArchivedRequestsForMentor(ctx context.Context, mentorID int) ([]*postgres.BotClientRequest, error) {
	return s.db.GetArchivedRequestsForMentor(ctx, mentorID)
}

// GetRequestByID returns a single request by ID
func (s *BotService) GetRequestByID(ctx context.Context, requestID int) (*postgres.BotClientRequest, error) {
	return s.db.GetRequestByID(ctx, requestID)
}

// UpdateRequestStatus updates the status of a client request
func (s *BotService) UpdateRequestStatus(ctx context.Context, requestID int, status string) error {
	if status == "" {
		return fmt.Errorf("status is required")
	}
	return s.db.UpdateRequestStatus(ctx, requestID, postgres.RequestStatus(status))
}
