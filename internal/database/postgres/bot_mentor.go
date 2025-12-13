package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// BotMentor represents a mentor for the bot API
type BotMentor struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email,omitempty"`
	JobTitle       string `json:"jobTitle,omitempty"`
	Workplace      string `json:"workplace,omitempty"`
	Details        string `json:"details,omitempty"`
	ProfileURL     string `json:"profileUrl,omitempty"`
	Slug           string `json:"alias"`
	TgSecret       string `json:"tgSecret,omitempty"`
	Telegram       string `json:"telegram,omitempty"`
	TelegramChatID string `json:"telegramChatId,omitempty"`
	Price          string `json:"price,omitempty"`
	Status         string `json:"status"`
	Tags           string `json:"tags,omitempty"`
	ImageURL       string `json:"image,omitempty"`
	Experience     string `json:"experience,omitempty"`
	CalendlyURL    string `json:"calendlyUrl,omitempty"`
	AuthToken      string `json:"authToken,omitempty"`
}

// getBotMentorByField is a helper that fetches a bot mentor by a specific field condition
func (c *Client) getBotMentorByField(ctx context.Context, operation, whereClause, notFoundMsg string, arg interface{}) (*BotMentor, error) {
	start := time.Now()

	query := fmt.Sprintf(`
		SELECT m.mentor_id, m.name, m.job_title, m.workplace, m.details, m.slug,
		       m.tg_secret, m.telegram_username, m.telegram_chat_id, m.price, m.status,
		       m.image_url, m.experience, m.calendar_url, m.auth_token,
		       COALESCE(
		           (SELECT string_agg(t.name, ',' ORDER BY t.name)
		            FROM mentor_tags mt
		            JOIN tags t ON t.id = mt.tag_id
		            WHERE mt.mentor_id = m.id), ''
		       ) as tags
		FROM mentors m
		WHERE %s
	`, whereClause)

	mentor, err := scanBotMentor(c.pool.QueryRow(ctx, query, arg))

	duration := metrics.MeasureDuration(start)

	if err == pgx.ErrNoRows {
		recordMetrics(operation, "not_found", duration)
		return nil, fmt.Errorf("%s", notFoundMsg)
	}
	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to query mentor: %w", err)
	}

	recordMetrics(operation, "success", duration)
	return mentor, nil
}

// GetMentorByTelegramChatID returns a mentor by their Telegram chat ID
func (c *Client) GetMentorByTelegramChatID(ctx context.Context, chatID string) (*BotMentor, error) {
	return c.getBotMentorByField(ctx, "getMentorByTelegramChatID",
		"m.telegram_chat_id = $1",
		fmt.Sprintf("mentor with telegram chat ID %s not found", chatID),
		chatID)
}

// GetMentorByTgSecret returns a mentor by their TgSecret code
func (c *Client) GetMentorByTgSecret(ctx context.Context, code string) (*BotMentor, error) {
	return c.getBotMentorByField(ctx, "getMentorByTgSecret",
		"m.tg_secret = $1",
		"mentor with tg_secret code not found",
		code)
}

// SetMentorTelegramChatID sets the Telegram chat ID for a mentor
func (c *Client) SetMentorTelegramChatID(ctx context.Context, mentorID int, chatID string) error {
	start := time.Now()
	operation := "setMentorTelegramChatID"

	query := `
		UPDATE mentors
		SET telegram_chat_id = $1, updated_at = NOW()
		WHERE mentor_id = $2
	`

	result, err := c.pool.Exec(ctx, query, chatID, mentorID)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to update telegram chat ID: %w", err)
	}

	if result.RowsAffected() == 0 {
		recordMetrics(operation, "not_found", duration)
		return fmt.Errorf("mentor with ID %d not found", mentorID)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration,
		zap.Int("mentor_id", mentorID),
		zap.String("chat_id", chatID))

	return nil
}

// SetMentorStatus updates the status of a mentor
func (c *Client) SetMentorStatus(ctx context.Context, mentorID int, status string) error {
	start := time.Now()
	operation := "setMentorStatus"

	// Validate status
	validStatuses := map[string]bool{
		"pending":  true,
		"active":   true,
		"inactive": true,
		"declined": true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("invalid mentor status: %s", status)
	}

	// Update is_visible based on status
	isVisible := status == "active"

	query := `
		UPDATE mentors
		SET status = $1, is_visible = $2, updated_at = NOW()
		WHERE mentor_id = $3
	`

	result, err := c.pool.Exec(ctx, query, status, isVisible, mentorID)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to update mentor status: %w", err)
	}

	if result.RowsAffected() == 0 {
		recordMetrics(operation, "not_found", duration)
		return fmt.Errorf("mentor with ID %d not found", mentorID)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration,
		zap.Int("mentor_id", mentorID),
		zap.String("status", status))

	return nil
}

// GetBotMentorByID returns a mentor by their numeric ID for bot API
func (c *Client) GetBotMentorByID(ctx context.Context, mentorID int) (*BotMentor, error) {
	return c.getBotMentorByField(ctx, "getBotMentorByID",
		"m.mentor_id = $1",
		fmt.Sprintf("mentor with ID %d not found", mentorID),
		mentorID)
}

// scanBotMentor scans a row into a BotMentor
func scanBotMentor(row pgx.Row) (*BotMentor, error) {
	var mentor BotMentor
	var jobTitle, workplace, details, tgSecret, telegram, chatID *string
	var price, imageURL, experience, calendlyURL, authToken, tags *string

	err := row.Scan(
		&mentor.ID, &mentor.Name, &jobTitle, &workplace, &details, &mentor.Slug,
		&tgSecret, &telegram, &chatID, &price, &mentor.Status,
		&imageURL, &experience, &calendlyURL, &authToken, &tags,
	)
	if err != nil {
		return nil, err
	}

	mentor.JobTitle = derefString(jobTitle)
	mentor.Workplace = derefString(workplace)
	mentor.Details = derefString(details)
	mentor.TgSecret = derefString(tgSecret)
	mentor.Telegram = derefString(telegram)
	mentor.TelegramChatID = derefString(chatID)
	mentor.Price = derefString(price)
	mentor.ImageURL = derefString(imageURL)
	mentor.Experience = derefString(experience)
	mentor.CalendlyURL = derefString(calendlyURL)
	mentor.AuthToken = derefString(authToken)
	mentor.Tags = derefString(tags)

	return &mentor, nil
}
