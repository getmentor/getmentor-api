package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/jackc/pgx/v5"
)

// ErrClientRequestNotFound is returned when a client request does not exist.
var ErrClientRequestNotFound = errors.New("client request not found")

// ErrInvalidMentorIDFilter is returned when the mentor ID filter is neither a UUID nor a legacy ID.
var ErrInvalidMentorIDFilter = errors.New("mentor ID filter must be a UUID or a numeric ID")

// escapeLikePattern escapes LIKE wildcards so user input is matched literally.
func escapeLikePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// isUUID performs a cheap syntactic UUID check (8-4-4-4-12 hex).
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, ch := range s {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			isHex := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

// BuildAdminRequestsWhere builds the WHERE clause and args for the admin list filter.
func BuildAdminRequestsWhere(filter *models.AdminRequestsFilter) (string, []interface{}, error) {
	conditions := []string{}
	args := []interface{}{}
	add := func(cond string, arg interface{}) {
		args = append(args, arg)
		conditions = append(conditions, fmt.Sprintf(cond, len(args)))
	}

	if mentorID := strings.TrimSpace(filter.MentorID); mentorID != "" {
		if isUUID(mentorID) {
			add("cr.mentor_id = $%d", strings.ToLower(mentorID))
		} else if legacyID, err := strconv.ParseInt(mentorID, 10, 64); err == nil {
			add("m.legacy_id = $%d", legacyID)
		} else {
			return "", nil, ErrInvalidMentorIDFilter
		}
	}
	if v := strings.TrimSpace(filter.MentorEmail); v != "" {
		add("m.email::text ILIKE '%%' || $%d || '%%'", escapeLikePattern(v))
	}
	if v := strings.TrimSpace(filter.MentorName); v != "" {
		add("m.name ILIKE '%%' || $%d || '%%'", escapeLikePattern(v))
	}
	if v := strings.TrimSpace(filter.MenteeEmail); v != "" {
		add("cr.email::text ILIKE '%%' || $%d || '%%'", escapeLikePattern(v))
	}
	if filter.Status != "" {
		add("cr.status = $%d", string(filter.Status))
	}

	if len(conditions) == 0 {
		return "", args, nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), args, nil
}

// ListForAdmin returns one page of client requests (newest first) and the total match count.
func (r *ClientRequestRepository) ListForAdmin(
	ctx context.Context,
	filter *models.AdminRequestsFilter,
) ([]models.AdminRequestListItem, int, error) {

	where, args, err := BuildAdminRequestsWhere(filter)
	if err != nil {
		return nil, 0, err
	}

	countQuery := `
		SELECT COUNT(*)
		FROM client_requests cr
		LEFT JOIN mentors m ON m.id = cr.mentor_id
		` + where

	var total int
	if err = r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count client requests: %w", err)
	}
	if total == 0 {
		return []models.AdminRequestListItem{}, 0, nil
	}

	listArgs := append(append([]interface{}{}, args...), filter.PageSize, filter.Offset())
	listQuery := fmt.Sprintf(`
		SELECT
			cr.id,
			COALESCE(cr.name, ''),
			COALESCE(cr.email::text, ''),
			COALESCE(cr.telegram, ''),
			cr.status,
			cr.created_at,
			EXISTS (SELECT 1 FROM reviews rv WHERE rv.client_request_id = cr.id) AS has_review,
			m.id::text,
			m.legacy_id,
			m.slug,
			m.name,
			m.email::text
		FROM client_requests cr
		LEFT JOIN mentors m ON m.id = cr.mentor_id
		%s
		ORDER BY cr.created_at DESC, cr.id DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list client requests: %w", err)
	}
	defer rows.Close()

	items := make([]models.AdminRequestListItem, 0, filter.PageSize)
	for rows.Next() {
		var item models.AdminRequestListItem
		var mentor nullableMentor
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Email,
			&item.Telegram,
			&item.Status,
			&item.CreatedAt,
			&item.HasReview,
			&mentor.id,
			&mentor.legacyID,
			&mentor.slug,
			&mentor.name,
			&mentor.email,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan client request: %w", err)
		}
		item.Mentor = mentor.toModel()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate client requests: %w", err)
	}

	return items, total, nil
}

// GetForAdminByID returns full request details with mentor summary and review (if any).
func (r *ClientRequestRepository) GetForAdminByID(ctx context.Context, id string) (*models.AdminRequestDetails, error) {
	if !isUUID(id) {
		return nil, ErrClientRequestNotFound
	}

	query := `
		SELECT
			cr.id,
			COALESCE(cr.name, ''),
			COALESCE(cr.email::text, ''),
			COALESCE(cr.telegram, ''),
			COALESCE(cr.description, ''),
			COALESCE(cr.level, ''),
			cr.status,
			cr.created_at,
			cr.updated_at,
			cr.status_changed_at,
			cr.scheduled_at,
			COALESCE(cr.decline_reason, ''),
			COALESCE(cr.decline_comment, ''),
			m.id::text,
			m.legacy_id,
			m.slug,
			m.name,
			m.email::text,
			rv.id::text,
			rv.mentor_review,
			rv.platform_review,
			rv.improvements,
			rv.created_at
		FROM client_requests cr
		LEFT JOIN mentors m ON m.id = cr.mentor_id
		LEFT JOIN reviews rv ON rv.client_request_id = cr.id
		WHERE cr.id = $1
	`

	var d models.AdminRequestDetails
	var mentor nullableMentor
	var reviewID, mentorReview, platformReview, improvements *string
	var reviewCreatedAt *time.Time

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&d.ID,
		&d.Name,
		&d.Email,
		&d.Telegram,
		&d.Details,
		&d.Level,
		&d.Status,
		&d.CreatedAt,
		&d.ModifiedAt,
		&d.StatusChangedAt,
		&d.ScheduledAt,
		&d.DeclineReason,
		&d.DeclineComment,
		&mentor.id,
		&mentor.legacyID,
		&mentor.slug,
		&mentor.name,
		&mentor.email,
		&reviewID,
		&mentorReview,
		&platformReview,
		&improvements,
		&reviewCreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrClientRequestNotFound
		}
		return nil, fmt.Errorf("failed to get client request: %w", err)
	}

	d.Mentor = mentor.toModel()
	if reviewID != nil {
		d.Review = &models.AdminRequestReview{
			ID:             *reviewID,
			MentorReview:   derefString(mentorReview),
			PlatformReview: derefString(platformReview),
			Improvements:   derefString(improvements),
		}
		if reviewCreatedAt != nil {
			d.Review.CreatedAt = *reviewCreatedAt
		}
	}

	return &d, nil
}

// nullableMentor holds LEFT JOIN mentor columns which are NULL when the mentor was deleted.
type nullableMentor struct {
	id       *string
	legacyID *int64
	slug     *string
	name     *string
	email    *string
}

func (m nullableMentor) toModel() *models.AdminRequestMentor {
	if m.id == nil {
		return nil
	}
	mentor := &models.AdminRequestMentor{
		MentorID: *m.id,
		Slug:     derefString(m.slug),
		Name:     derefString(m.name),
		Email:    derefString(m.email),
	}
	if m.legacyID != nil {
		mentor.LegacyID = int(*m.legacyID)
	}
	return mentor
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
