package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// RequestStatus represents the status of a client request
type RequestStatus string

const (
	RequestStatusPending     RequestStatus = "pending"
	RequestStatusContacted   RequestStatus = "contacted"
	RequestStatusWorking     RequestStatus = "working"
	RequestStatusDone        RequestStatus = "done"
	RequestStatusDeclined    RequestStatus = "declined"
	RequestStatusUnavailable RequestStatus = "unavailable"
	RequestStatusReschedule  RequestStatus = "reschedule"
)

// ClientRequestRow represents a client request row from the database
type ClientRequestRow struct {
	ID              int
	AirtableID      *string
	Email           string
	Name            string
	Description     *string
	Telegram        *string
	Level           *string
	MentorID        *int
	Status          RequestStatus
	StatusChangedAt *time.Time
	ScheduledAt     *time.Time
	Review          *string
	ReviewToken     uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// BotClientRequest represents a client request for the bot API
type BotClientRequest struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Telegram        string     `json:"telegram"`
	Description     string     `json:"description"`
	Level           string     `json:"level"`
	Status          string     `json:"status"`
	Review          *string    `json:"review"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	ScheduledAt     *time.Time `json:"scheduledAt"`
	StatusChangedAt *time.Time `json:"statusChangedAt"`
	MentorID        string     `json:"mentorId"`
}

// CreateClientRequest creates a new client request
func (c *Client) CreateClientRequest(ctx context.Context, req *models.ClientRequest) error {
	start := time.Now()
	operation := "createClientRequest"

	// Get mentor's internal ID from airtable_id or slug
	var mentorPK *int
	if req.MentorID != "" {
		var pk int
		err := c.pool.QueryRow(ctx,
			"SELECT id FROM mentors WHERE airtable_id = $1 OR slug = $1",
			req.MentorID).Scan(&pk)
		if err != nil && err != pgx.ErrNoRows {
			duration := metrics.MeasureDuration(start)
			recordMetrics(operation, "error", duration)
			return fmt.Errorf("failed to find mentor: %w", err)
		}
		if err == nil {
			mentorPK = &pk
		}
	}

	query := `
		INSERT INTO client_requests (email, name, description, telegram, level, mentor_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := c.pool.Exec(ctx, query,
		req.Email,
		req.Name,
		nilIfEmpty(req.Description),
		nilIfEmpty(req.Telegram),
		nilIfEmpty(req.Level),
		mentorPK,
		RequestStatusPending,
	)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to create client request: %w", err)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration)

	return nil
}

// getRequestsForMentor is a helper that fetches requests for a mentor with a status filter
func (c *Client) getRequestsForMentor(ctx context.Context, mentorID int, operation, statusFilter, orderBy string) ([]*BotClientRequest, error) {
	start := time.Now()

	// Get mentor's internal PK
	var mentorPK int
	err := c.pool.QueryRow(ctx, "SELECT id FROM mentors WHERE mentor_id = $1", mentorID).Scan(&mentorPK)
	if err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		return nil, fmt.Errorf("mentor not found: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT cr.id, cr.name, cr.email, cr.telegram, cr.description, cr.level,
		       cr.status, cr.review, cr.created_at, cr.updated_at, cr.scheduled_at,
		       cr.status_changed_at, m.mentor_id
		FROM client_requests cr
		JOIN mentors m ON m.id = cr.mentor_id
		WHERE cr.mentor_id = $1
		  AND cr.status NOT IN (%s)
		ORDER BY %s
	`, statusFilter, orderBy)

	rows, err := c.pool.Query(ctx, query, mentorPK)
	if err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		return nil, fmt.Errorf("failed to query requests: %w", err)
	}
	defer rows.Close()

	requests := make([]*BotClientRequest, 0)
	for rows.Next() {
		req, err := scanBotRequest(rows)
		if err != nil {
			duration := metrics.MeasureDuration(start)
			recordMetrics(operation, "error", duration)
			return nil, err
		}
		requests = append(requests, req)
	}

	duration := metrics.MeasureDuration(start)
	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.Int("count", len(requests)))

	return requests, nil
}

// GetActiveRequestsForMentor returns active requests for a mentor (for bot)
func (c *Client) GetActiveRequestsForMentor(ctx context.Context, mentorID int) ([]*BotClientRequest, error) {
	return c.getRequestsForMentor(ctx, mentorID,
		"getActiveRequestsForMentor",
		"'done', 'declined', 'unavailable'",
		"cr.created_at ASC")
}

// GetArchivedRequestsForMentor returns archived requests for a mentor (for bot)
func (c *Client) GetArchivedRequestsForMentor(ctx context.Context, mentorID int) ([]*BotClientRequest, error) {
	return c.getRequestsForMentor(ctx, mentorID,
		"getArchivedRequestsForMentor",
		"'pending', 'working', 'contacted'",
		"cr.updated_at DESC")
}

// GetRequestByID returns a single request by ID (for bot)
func (c *Client) GetRequestByID(ctx context.Context, requestID int) (*BotClientRequest, error) {
	start := time.Now()
	operation := "getRequestByID"

	query := `
		SELECT cr.id, cr.name, cr.email, cr.telegram, cr.description, cr.level,
		       cr.status, cr.review, cr.created_at, cr.updated_at, cr.scheduled_at,
		       cr.status_changed_at, m.mentor_id
		FROM client_requests cr
		JOIN mentors m ON m.id = cr.mentor_id
		WHERE cr.id = $1
	`

	row := c.pool.QueryRow(ctx, query, requestID)
	req, err := scanBotRequestRow(row)

	duration := metrics.MeasureDuration(start)

	if err == pgx.ErrNoRows {
		recordMetrics(operation, "not_found", duration)
		return nil, fmt.Errorf("request with ID %d not found", requestID)
	}
	if err != nil {
		recordMetrics(operation, "error", duration)
		return nil, err
	}

	recordMetrics(operation, "success", duration)
	return req, nil
}

// UpdateRequestStatus updates the status of a client request (for bot)
func (c *Client) UpdateRequestStatus(ctx context.Context, requestID int, newStatus RequestStatus) error {
	start := time.Now()
	operation := "updateRequestStatus"

	// Validate status transition
	var currentStatus RequestStatus
	err := c.pool.QueryRow(ctx, "SELECT status FROM client_requests WHERE id = $1", requestID).Scan(&currentStatus)
	if err == pgx.ErrNoRows {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "not_found", duration)
		return fmt.Errorf("request with ID %d not found", requestID)
	}
	if err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		return fmt.Errorf("failed to get current status: %w", err)
	}

	// Check if transition is allowed
	if !isValidStatusTransition(currentStatus, newStatus) {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "invalid_transition", duration)
		return fmt.Errorf("invalid status transition from %s to %s", currentStatus, newStatus)
	}

	query := `
		UPDATE client_requests
		SET status = $1, status_changed_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`

	result, err := c.pool.Exec(ctx, query, newStatus, requestID)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return fmt.Errorf("failed to update request status: %w", err)
	}

	if result.RowsAffected() == 0 {
		recordMetrics(operation, "not_found", duration)
		return fmt.Errorf("request with ID %d not found", requestID)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration,
		zap.Int("request_id", requestID),
		zap.String("new_status", string(newStatus)))

	return nil
}

// isValidStatusTransition checks if a status transition is allowed
func isValidStatusTransition(from, to RequestStatus) bool {
	// Terminal states cannot be changed
	if from == RequestStatusDone || from == RequestStatusDeclined {
		return false
	}

	// Define valid transitions
	validTransitions := map[RequestStatus][]RequestStatus{
		RequestStatusPending:     {RequestStatusContacted, RequestStatusDeclined},
		RequestStatusContacted:   {RequestStatusWorking, RequestStatusDeclined, RequestStatusUnavailable},
		RequestStatusWorking:     {RequestStatusDone, RequestStatusDeclined, RequestStatusUnavailable, RequestStatusReschedule},
		RequestStatusUnavailable: {RequestStatusContacted}, // Can revert
		RequestStatusReschedule:  {RequestStatusWorking, RequestStatusDone, RequestStatusDeclined, RequestStatusUnavailable},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}

	return false
}

// scanBotRequest scans a row into a BotClientRequest
func scanBotRequest(rows pgx.Rows) (*BotClientRequest, error) {
	var req BotClientRequest
	var id int
	var telegram, description, level *string
	var mentorID int

	err := rows.Scan(
		&id, &req.Name, &req.Email, &telegram, &description, &level,
		&req.Status, &req.Review, &req.CreatedAt, &req.UpdatedAt, &req.ScheduledAt,
		&req.StatusChangedAt, &mentorID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan request row: %w", err)
	}

	req.ID = fmt.Sprintf("%d", id)
	req.MentorID = fmt.Sprintf("%d", mentorID)
	req.Telegram = derefString(telegram)
	req.Description = derefString(description)
	req.Level = derefString(level)

	return &req, nil
}

// scanBotRequestRow scans a single row into a BotClientRequest
func scanBotRequestRow(row pgx.Row) (*BotClientRequest, error) {
	var req BotClientRequest
	var id int
	var telegram, description, level *string
	var mentorID int

	err := row.Scan(
		&id, &req.Name, &req.Email, &telegram, &description, &level,
		&req.Status, &req.Review, &req.CreatedAt, &req.UpdatedAt, &req.ScheduledAt,
		&req.StatusChangedAt, &mentorID,
	)
	if err != nil {
		return nil, err
	}

	req.ID = fmt.Sprintf("%d", id)
	req.MentorID = fmt.Sprintf("%d", mentorID)
	req.Telegram = derefString(telegram)
	req.Description = derefString(description)
	req.Level = derefString(level)

	return &req, nil
}

// nilIfEmpty returns nil if string is empty, otherwise returns pointer to string
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
