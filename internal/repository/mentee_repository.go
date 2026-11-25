package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrMenteeNotFound means no mentee row matched (unknown email, bad token, unknown id).
	ErrMenteeNotFound = errors.New("mentee not found")
	// ErrMenteeRequestNotFound covers both a missing request and one sent from another email.
	ErrMenteeRequestNotFound = errors.New("request not found")
	// ErrMenteeReviewNotAllowed means the request is not done or its mentor is deleted.
	ErrMenteeReviewNotAllowed = errors.New("review not allowed for this request")
	// ErrMenteeReviewExists means the request already has a review.
	ErrMenteeReviewExists = errors.New("review already exists for this request")
)

// MenteeRepository is the data access for the mentee dashboard.
type MenteeRepository struct {
	pool *pgxpool.Pool
}

// NewMenteeRepository creates a new mentee repository.
func NewMenteeRepository(pool *pgxpool.Pool) *MenteeRepository {
	return &MenteeRepository{pool: pool}
}

// SetLoginToken stores the hash of a new login token for the email, creating the mentee
// row on first login. The existence check and the write are one statement, so a row is
// never created for an email that has not sent a request. Returns ErrMenteeNotFound
// when the email has no requests.
func (r *MenteeRepository) SetLoginToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) (string, error) {
	const query = `
		INSERT INTO mentees (email, login_token, login_token_expires_at)
		SELECT $1::citext, $2, $3
		WHERE EXISTS (SELECT 1 FROM client_requests WHERE email = $1::citext)
		ON CONFLICT (email) DO UPDATE
			SET login_token = EXCLUDED.login_token,
				login_token_expires_at = EXCLUDED.login_token_expires_at
		RETURNING id
	`
	var id string
	err := r.pool.QueryRow(ctx, query, email, tokenHash, expiresAt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrMenteeNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to store mentee login token: %w", err)
	}
	return id, nil
}

// ConsumeLoginToken atomically clears an unexpired token and returns its mentee.
// Of any number of concurrent callers with the same token, exactly one gets the row.
func (r *MenteeRepository) ConsumeLoginToken(ctx context.Context, tokenHash string) (*models.Mentee, error) {
	const query = `
		UPDATE mentees
		SET login_token = NULL, login_token_expires_at = NULL
		WHERE login_token = $1 AND login_token_expires_at >= now()
		RETURNING id, email::text, session_version
	`
	var m models.Mentee
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(&m.ID, &m.Email, &m.SessionVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMenteeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to consume mentee login token: %w", err)
	}
	return &m, nil
}

// GetSessionVersion returns the current session version of a mentee.
func (r *MenteeRepository) GetSessionVersion(ctx context.Context, menteeID string) (int, error) {
	var version int
	err := r.pool.QueryRow(ctx, `SELECT session_version FROM mentees WHERE id = $1`, menteeID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrMenteeNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("failed to read mentee session version: %w", err)
	}
	return version, nil
}

// RevokeSessions increments the session version, invalidating every issued session token.
func (r *MenteeRepository) RevokeSessions(ctx context.Context, menteeID string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE mentees SET session_version = session_version + 1 WHERE id = $1`, menteeID)
	if err != nil {
		return fmt.Errorf("failed to revoke mentee sessions: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMenteeNotFound
	}
	return nil
}

// ListRequests returns every request sent from the mentee's stored email, newest first.
// The email comes from the mentees row (looked up by id), never from the caller.
func (r *MenteeRepository) ListRequests(ctx context.Context, menteeID string) ([]models.MenteeRequestRow, error) {
	const query = `
		SELECT cr.id, cr.status, cr.description, cr.created_at, cr.status_changed_at,
			cr.decline_reason, cr.decline_comment,
			m.id, m.name, m.slug, m.calendar_url, m.status,
			COALESCE(m.telegram_chat_id IS NOT NULL, false), m.deleted_at,
			rv.id IS NOT NULL, rv.mentor_review, rv.platform_review, rv.improvements, rv.created_at
		FROM mentees me
		JOIN client_requests cr ON cr.email = me.email
		LEFT JOIN mentors m ON m.id = cr.mentor_id
		LEFT JOIN reviews rv ON rv.client_request_id = cr.id
		WHERE me.id = $1
		ORDER BY cr.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, menteeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list mentee requests: %w", err)
	}
	defer rows.Close()

	result := []models.MenteeRequestRow{}
	for rows.Next() {
		var row models.MenteeRequestRow
		if err := rows.Scan(
			&row.ID, &row.Status, &row.Description, &row.CreatedAt, &row.StatusChangedAt,
			&row.DeclineReason, &row.DeclineComment,
			&row.MentorID, &row.MentorName, &row.MentorSlug, &row.MentorCalendarURL, &row.MentorStatus,
			&row.MentorHasTelegramBot, &row.MentorDeletedAt,
			&row.HasReview, &row.ReviewMentorReview, &row.ReviewPlatformReview, &row.ReviewImprovements, &row.ReviewCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan mentee request: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read mentee requests: %w", err)
	}
	return result, nil
}

// CreateReview stores a dashboard review in one transaction: ownership check, eligibility
// re-check under a row lock, then the insert, which also relies on the reviews unique
// constraint. A request that is not the mentee's and one that doesn't exist both give
// ErrMenteeRequestNotFound.
//
// There is no review-invitation table: the emailed link (/reviews/new?request_id=...) only
// works while the request has no review, so this insert is what makes that link stop working.
func (r *MenteeRepository) CreateReview(ctx context.Context, menteeID, requestID, mentorReview, platformReview, improvements string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin review transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	}()

	var status string
	var mentorAvailable bool
	err = tx.QueryRow(ctx, `
		SELECT cr.status, (m.id IS NOT NULL AND m.deleted_at IS NULL)
		FROM client_requests cr
		JOIN mentees me ON me.id = $1 AND me.email = cr.email
		LEFT JOIN mentors m ON m.id = cr.mentor_id
		WHERE cr.id = $2
		FOR UPDATE OF cr
	`, menteeID, requestID).Scan(&status, &mentorAvailable)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrMenteeRequestNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" { // not a uuid
			return "", ErrMenteeRequestNotFound
		}
		return "", fmt.Errorf("failed to check request ownership: %w", err)
	}
	if status != string(models.StatusDone) || !mentorAvailable {
		return "", ErrMenteeReviewNotAllowed
	}

	var reviewID string
	err = tx.QueryRow(ctx, `
		INSERT INTO reviews (client_request_id, mentor_review, platform_review, improvements)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, requestID, mentorReview, platformReview, improvements).Scan(&reviewID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", ErrMenteeReviewExists
		}
		return "", fmt.Errorf("failed to insert review: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit review: %w", err)
	}
	return reviewID, nil
}
