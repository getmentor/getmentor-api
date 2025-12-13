package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// MentorRow represents a mentor row from the database
type MentorRow struct {
	ID               int
	AirtableID       *string
	MentorID         int
	Slug             string
	Name             string
	JobTitle         *string
	Workplace        *string
	Details          *string
	About            *string
	Competencies     *string
	Experience       *string
	Price            *string
	SessionsCount    int
	SortOrder        int
	IsVisible        bool
	Status           string
	AuthToken        *string
	CalendarURL      *string
	IsNew            bool
	ImageURL         *string
	TelegramUsername *string
	TelegramChatID   *string
	TgSecret         *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GetAllMentors fetches all mentors from the database
func (c *Client) GetAllMentors(ctx context.Context) ([]*models.Mentor, error) {
	start := time.Now()
	operation := "getAllMentors"

	query := `
		SELECT
			m.id, m.airtable_id, m.mentor_id, m.slug, m.name, m.job_title, m.workplace,
			m.details, m.about, m.competencies, m.experience, m.price, m.sessions_count,
			m.sort_order, m.is_visible, m.status, m.auth_token, m.calendar_url, m.is_new,
			m.image_url, m.telegram_username, m.telegram_chat_id, m.tg_secret,
			m.created_at, m.updated_at,
			COALESCE(
				(SELECT string_agg(t.name, ',' ORDER BY t.name)
				 FROM mentor_tags mt
				 JOIN tags t ON t.id = mt.tag_id
				 WHERE mt.mentor_id = m.id), ''
			) as tags
		FROM mentors m
		ORDER BY m.sort_order ASC
	`

	rows, err := c.pool.Query(ctx, query)
	if err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to query mentors: %w", err)
	}
	defer rows.Close()

	mentors := make([]*models.Mentor, 0)
	for rows.Next() {
		var row MentorRow
		var tagsStr string

		err := rows.Scan(
			&row.ID, &row.AirtableID, &row.MentorID, &row.Slug, &row.Name, &row.JobTitle,
			&row.Workplace, &row.Details, &row.About, &row.Competencies, &row.Experience,
			&row.Price, &row.SessionsCount, &row.SortOrder, &row.IsVisible, &row.Status,
			&row.AuthToken, &row.CalendarURL, &row.IsNew, &row.ImageURL,
			&row.TelegramUsername, &row.TelegramChatID, &row.TgSecret,
			&row.CreatedAt, &row.UpdatedAt, &tagsStr,
		)
		if err != nil {
			duration := metrics.MeasureDuration(start)
			recordMetrics(operation, "error", duration)
			logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
			return nil, fmt.Errorf("failed to scan mentor row: %w", err)
		}

		mentor := rowToMentor(&row, tagsStr)
		mentors = append(mentors, mentor)
	}

	if err := rows.Err(); err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("error iterating mentor rows: %w", err)
	}

	duration := metrics.MeasureDuration(start)
	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.Int("count", len(mentors)))

	return mentors, nil
}

// getMentorByField is a helper that fetches a mentor by a specific field condition
func (c *Client) getMentorByField(ctx context.Context, operation, whereClause, notFoundMsg string, arg interface{}) (*models.Mentor, error) {
	start := time.Now()

	query := fmt.Sprintf(`
		SELECT
			m.id, m.airtable_id, m.mentor_id, m.slug, m.name, m.job_title, m.workplace,
			m.details, m.about, m.competencies, m.experience, m.price, m.sessions_count,
			m.sort_order, m.is_visible, m.status, m.auth_token, m.calendar_url, m.is_new,
			m.image_url, m.telegram_username, m.telegram_chat_id, m.tg_secret,
			m.created_at, m.updated_at,
			COALESCE(
				(SELECT string_agg(t.name, ',' ORDER BY t.name)
				 FROM mentor_tags mt
				 JOIN tags t ON t.id = mt.tag_id
				 WHERE mt.mentor_id = m.id), ''
			) as tags
		FROM mentors m
		WHERE %s
	`, whereClause)

	var row MentorRow
	var tagsStr string

	err := c.pool.QueryRow(ctx, query, arg).Scan(
		&row.ID, &row.AirtableID, &row.MentorID, &row.Slug, &row.Name, &row.JobTitle,
		&row.Workplace, &row.Details, &row.About, &row.Competencies, &row.Experience,
		&row.Price, &row.SessionsCount, &row.SortOrder, &row.IsVisible, &row.Status,
		&row.AuthToken, &row.CalendarURL, &row.IsNew, &row.ImageURL,
		&row.TelegramUsername, &row.TelegramChatID, &row.TgSecret,
		&row.CreatedAt, &row.UpdatedAt, &tagsStr,
	)

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
	return rowToMentor(&row, tagsStr), nil
}

// GetMentorBySlug fetches a single mentor by slug
func (c *Client) GetMentorBySlug(ctx context.Context, slug string) (*models.Mentor, error) {
	return c.getMentorByField(ctx, "getMentorBySlug",
		"m.slug = $1",
		fmt.Sprintf("mentor with slug %s not found", slug),
		slug)
}

// GetMentorByID fetches a single mentor by numeric ID
func (c *Client) GetMentorByID(ctx context.Context, mentorID int) (*models.Mentor, error) {
	return c.getMentorByField(ctx, "getMentorByID",
		"m.mentor_id = $1",
		fmt.Sprintf("mentor with ID %d not found", mentorID),
		mentorID)
}

// GetMentorByAirtableID fetches a single mentor by Airtable record ID
func (c *Client) GetMentorByAirtableID(ctx context.Context, airtableID string) (*models.Mentor, error) {
	return c.getMentorByField(ctx, "getMentorByAirtableID",
		"m.airtable_id = $1",
		fmt.Sprintf("mentor with airtable ID %s not found", airtableID),
		airtableID)
}

// UpdateMentor updates mentor fields
func (c *Client) UpdateMentor(ctx context.Context, slug string, updates map[string]interface{}) error {
	start := time.Now()
	operation := "updateMentor"

	// Build dynamic update query
	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+1)
	argIndex := 1

	// Map of allowed fields and their database column names
	fieldMapping := map[string]string{
		"Name":         "name",
		"JobTitle":     "job_title",
		"Workplace":    "workplace",
		"Details":      "details",
		"About":        "about",
		"Competencies": "competencies",
		"Experience":   "experience",
		"Price":        "price",
		"Calendly Url": "calendar_url",
		"AuthToken":    "auth_token",
		"Status":       "status",
		"OnSite":       "is_visible",
		"Is New":       "is_new",
		"SortOrder":    "sort_order",
	}

	for field, value := range updates {
		if field == "Tags Links" {
			// Tags are handled separately
			continue
		}

		colName, ok := fieldMapping[field]
		if !ok {
			continue // Skip unknown fields
		}

		// Handle special type conversions
		switch field {
		case "OnSite":
			// Convert int to bool
			if v, ok := value.(float64); ok {
				value = v == 1
			}
		case "Is New":
			// Convert int to bool
			if v, ok := value.(float64); ok {
				value = v == 1
			}
		}

		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", colName, argIndex))
		args = append(args, value)
		argIndex++
	}

	if len(setClauses) == 0 {
		return nil // Nothing to update
	}

	// Add slug as the last argument for WHERE clause
	args = append(args, slug)

	query := fmt.Sprintf(
		"UPDATE mentors SET %s, updated_at = NOW() WHERE slug = $%d",
		strings.Join(setClauses, ", "),
		argIndex,
	)

	result, err := c.pool.Exec(ctx, query, args...)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to update mentor: %w", err)
	}

	if result.RowsAffected() == 0 {
		recordMetrics(operation, "not_found", duration)
		return fmt.Errorf("mentor with slug %s not found", slug)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.String("slug", slug))

	return nil
}

// UpdateMentorTags updates the tags for a mentor
func (c *Client) UpdateMentorTags(ctx context.Context, slug string, tagIDs []int) error {
	start := time.Now()
	operation := "updateMentorTags"

	// Get mentor's internal ID
	var mentorPK int
	err := c.pool.QueryRow(ctx, "SELECT id FROM mentors WHERE slug = $1", slug).Scan(&mentorPK)
	if err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		return fmt.Errorf("failed to find mentor: %w", err)
	}

	// Use a transaction
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Delete existing tags
	_, err = tx.Exec(ctx, "DELETE FROM mentor_tags WHERE mentor_id = $1", mentorPK)
	if err != nil {
		return fmt.Errorf("failed to delete existing tags: %w", err)
	}

	// Insert new tags
	for _, tagID := range tagIDs {
		_, err = tx.Exec(ctx,
			"INSERT INTO mentor_tags (mentor_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			mentorPK, tagID)
		if err != nil {
			return fmt.Errorf("failed to insert tag: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	duration := metrics.MeasureDuration(start)
	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.String("slug", slug))

	return nil
}

// UpdateMentorImage updates a mentor's profile image URL
func (c *Client) UpdateMentorImage(ctx context.Context, slug, imageURL string) error {
	start := time.Now()
	operation := "updateMentorImage"

	query := "UPDATE mentors SET image_url = $1, updated_at = NOW() WHERE slug = $2"
	result, err := c.pool.Exec(ctx, query, imageURL, slug)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to update mentor image: %w", err)
	}

	if result.RowsAffected() == 0 {
		recordMetrics(operation, "not_found", duration)
		return fmt.Errorf("mentor with slug %s not found", slug)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.String("slug", slug))

	return nil
}

// rowToMentor converts a database row to a Mentor model
func rowToMentor(row *MentorRow, tagsStr string) *models.Mentor {
	// Parse tags
	tags := []string{}
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Determine calendar type
	calendarURL := ""
	if row.CalendarURL != nil {
		calendarURL = *row.CalendarURL
	}
	calendarType := models.GetCalendarType(calendarURL)

	// Get sponsor
	sponsor := models.GetMentorSponsor(tags)

	// Build AirtableID
	airtableID := ""
	if row.AirtableID != nil {
		airtableID = *row.AirtableID
	}

	return &models.Mentor{
		ID:           row.MentorID,
		AirtableID:   airtableID,
		Slug:         row.Slug,
		Name:         row.Name,
		Job:          derefString(row.JobTitle),
		Workplace:    derefString(row.Workplace),
		Description:  derefString(row.Details),
		About:        derefString(row.About),
		Competencies: derefString(row.Competencies),
		Experience:   derefString(row.Experience),
		Price:        derefString(row.Price),
		MenteeCount:  row.SessionsCount,
		Tags:         tags,
		SortOrder:    row.SortOrder,
		IsVisible:    row.IsVisible,
		Sponsors:     sponsor,
		CalendarType: calendarType,
		IsNew:        row.IsNew,
		AuthToken:    derefString(row.AuthToken),
		CalendarURL:  calendarURL,
	}
}

// derefString safely dereferences a string pointer
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
