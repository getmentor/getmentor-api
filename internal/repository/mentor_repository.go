package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/getmentor/getmentor-api/internal/cache"
	"github.com/getmentor/getmentor-api/internal/models"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"github.com/getmentor/getmentor-api/pkg/slug"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MentorRepository handles mentor data access with PostgreSQL
type MentorRepository struct {
	pool               *pgxpool.Pool
	mentorCache        *cache.MentorCache
	tagsCache          *cache.TagsCache
	disableMentorCache bool
}

// NewMentorRepository creates a new PostgreSQL-based mentor repository
func NewMentorRepository(pool *pgxpool.Pool, mentorCache *cache.MentorCache, tagsCache *cache.TagsCache, disableMentorCache bool) *MentorRepository {
	return &MentorRepository{
		pool:               pool,
		mentorCache:        mentorCache,
		tagsCache:          tagsCache,
		disableMentorCache: disableMentorCache,
	}
}

// GetAll retrieves all mentors with optional filtering
func (r *MentorRepository) GetAll(ctx context.Context, opts models.FilterOptions) ([]*models.Mentor, error) {
	var mentors []*models.Mentor
	var err error

	// Experimental: bypass cache if disabled
	if r.disableMentorCache {
		logger.Debug("Cache disabled, fetching mentors from database")
		mentors, err = r.FetchAllMentorsFromDB(ctx)
		if err != nil {
			logger.Error("Failed to fetch mentors from database",
				zap.Error(err))
			return nil, err
		}
		logger.Debug("Successfully fetched mentors from database",
			zap.Int("count", len(mentors)))
	} else {
		// ForceRefresh triggers background refresh but returns current data
		if opts.ForceRefresh {
			mentors, err = r.mentorCache.ForceRefresh()
		} else {
			mentors, err = r.mentorCache.Get()
		}

		if err != nil {
			return nil, err
		}
	}

	// Apply filters
	filtered := r.applyFilters(mentors, opts)

	return filtered, nil
}

// GetByID retrieves a mentor by legacy numeric ID
// Note: O(n) complexity is acceptable as per requirements
func (r *MentorRepository) GetByID(ctx context.Context, id int, opts models.FilterOptions) (*models.Mentor, error) {
	mentors, err := r.GetAll(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.LegacyID == id {
			return mentor, nil
		}
	}

	return nil, fmt.Errorf("mentor with ID %d not found", id)
}

// GetBySlug retrieves a mentor by slug with O(1) complexity
func (r *MentorRepository) GetBySlug(ctx context.Context, mentorSlug string, opts models.FilterOptions) (*models.Mentor, error) {
	var mentor *models.Mentor
	var err error

	// Experimental: bypass cache if disabled
	if r.disableMentorCache {
		mentor, err = r.FetchSingleMentorFromDB(ctx, mentorSlug)
		if err != nil {
			return nil, err
		}
	} else {
		// Note: ForceRefresh is ignored for single lookups
		// Only webhook/profile updates trigger single-mentor refresh
		mentor, err = r.mentorCache.GetBySlug(mentorSlug)
		if err != nil {
			return nil, err
		}
	}

	// Apply filters to single mentor
	filtered := r.applySingleMentorFilters(mentor, opts)
	if filtered == nil {
		return nil, fmt.Errorf("mentor with slug %s not found or not visible", mentorSlug)
	}

	return filtered, nil
}

// GetByMentorId retrieves a mentor by UUID
// First tries cache (active mentors only), then falls back to database query
func (r *MentorRepository) GetByMentorId(ctx context.Context, mentorId string, opts models.FilterOptions) (*models.Mentor, error) {
	// Try cache first (contains only active mentors)
	mentors, err := r.GetAll(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, mentor := range mentors {
		if mentor.MentorID == mentorId {
			return mentor, nil
		}
	}

	// Fallback to DB query for inactive mentors or mentors not in cache
	mentor, err := r.fetchMentorByUUIDFromDB(ctx, mentorId)
	if err != nil {
		return nil, fmt.Errorf("mentor with ID %s not found", mentorId)
	}

	// Apply filters to the fetched mentor
	filtered := r.applySingleMentorFilters(mentor, opts)
	if filtered == nil {
		return nil, fmt.Errorf("mentor with ID %s not found or filtered out", mentorId)
	}

	return filtered, nil
}

// fetchMentorByUUIDFromDB retrieves a single mentor by UUID from PostgreSQL
func (r *MentorRepository) fetchMentorByUUIDFromDB(ctx context.Context, mentorId string) (*models.Mentor, error) {
	query := `
		SELECT m.id, m.airtable_id, m.legacy_id, m.slug, m.name, m.job_title, m.workplace,
			m.about, m.details, m.competencies, m.experience, m.price, m.status,
			COALESCE(array_to_string(array_agg(t.name), ','), '') as tags,
			m.telegram_chat_id, m.calendar_url, m.sort_order, m.created_at,
			COALESCE(
				(SELECT COUNT(*)
				 FROM client_requests cr
				 WHERE cr.mentor_id = m.id
				 AND cr.status = 'done'),
				0
			) AS mentee_count
		FROM mentors m
		LEFT JOIN mentor_tags mt ON mt.mentor_id = m.id
		LEFT JOIN tags t ON t.id = mt.tag_id
		WHERE m.id = $1
		GROUP BY m.id
	`

	row := r.pool.QueryRow(ctx, query, mentorId)
	return models.ScanMentor(row)
}

// allowedUpdateColumns defines the columns that can be updated via the Update method
var allowedUpdateColumns = map[string]bool{
	"name":         true,
	"job_title":    true,
	"workplace":    true,
	"about":        true,
	"details":      true,
	"competencies": true,
	"experience":   true,
	"price":        true,
	"telegram":     true,
	"calendar_url": true,
	"status":       true,
	"updated_at":   true,
}

// Update updates a mentor in PostgreSQL
func (r *MentorRepository) Update(ctx context.Context, mentorId string, updates map[string]interface{}) error {
	// Validate all keys against allowlist to prevent SQL injection
	for key := range updates {
		if !allowedUpdateColumns[key] {
			return fmt.Errorf("invalid column name: %s", key)
		}
	}

	// Build dynamic UPDATE query
	// This is simplified - in production you'd want proper query building
	query := `UPDATE mentors SET `
	args := []interface{}{}
	argPos := 1

	for key, value := range updates {
		if argPos > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", key, argPos)
		args = append(args, value)
		argPos++
	}

	query += fmt.Sprintf(", updated_at = NOW() WHERE id = $%d", argPos)
	args = append(args, mentorId)

	_, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update mentor: %w", err)
	}

	// Note: Cache will auto-refresh after TTL expires
	return nil
}

// UpdateImage updates a mentor's profile image URL
func (r *MentorRepository) UpdateImage(ctx context.Context, mentorId, imageURL string) error {
	query := `UPDATE mentors SET updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, mentorId)
	return err
}

// CreateMentor creates a new mentor record in PostgreSQL
// Returns: mentorId (UUID), legacyId (int), error
// Note: slug is generated automatically using pre-fetched legacy_id
func (r *MentorRepository) CreateMentor(ctx context.Context, fields map[string]interface{}) (string, int, error) {
	// Begin transaction to ensure atomicity
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// Rollback is safe to call even after Commit
		_ = tx.Rollback(ctx) //nolint:errcheck
	}()

	// Pre-fetch the next legacy_id from the sequence
	var nextLegacyID int
	err = tx.QueryRow(ctx, "SELECT nextval('mentors_legacy_id_seq')").Scan(&nextLegacyID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get next legacy_id: %w", err)
	}

	// Generate slug from name and legacy_id
	name, ok := fields["name"].(string)
	if !ok || name == "" {
		return "", 0, fmt.Errorf("name is required")
	}
	mentorSlug := slug.GenerateMentorSlug(name, nextLegacyID)

	query := `
		INSERT INTO mentors (legacy_id, slug, name, email, job_title, workplace, about, details,
			competencies, experience, price, status, telegram, tg_secret, calendar_url, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id
	`

	var mentorId string

	err = tx.QueryRow(ctx, query,
		nextLegacyID, // Explicit legacy_id
		mentorSlug,   // Generated slug
		fields["name"],
		fields["email"],
		fields["job_title"],
		fields["workplace"],
		fields["about"],
		fields["details"],
		fields["competencies"],
		fields["experience"],
		fields["price"],
		fields["status"],
		fields["telegram"],
		fields["tg_secret"],
		fields["calendar_url"],
		fields["sort_order"],
	).Scan(&mentorId)

	if err != nil {
		return "", 0, fmt.Errorf("failed to create mentor: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return "", 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return mentorId, nextLegacyID, nil
}

// GetTagIDByName retrieves a tag ID by name
func (r *MentorRepository) GetTagIDByName(ctx context.Context, name string) (string, error) {
	return r.tagsCache.GetTagIDByName(name)
}

// UpdateMentorTags updates the tags for a mentor
func (r *MentorRepository) UpdateMentorTags(ctx context.Context, mentorID string, tagIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		// Rollback is safe to call even after Commit
		// Error is ignored as we prioritize the Commit error
		_ = tx.Rollback(ctx) //nolint:errcheck
	}()

	// Delete existing tags for this mentor
	_, err = tx.Exec(ctx, "DELETE FROM mentor_tags WHERE mentor_id = $1", mentorID)
	if err != nil {
		return fmt.Errorf("failed to delete existing tags: %w", err)
	}

	// Insert new tags
	for _, tagID := range tagIDs {
		_, err = tx.Exec(ctx,
			"INSERT INTO mentor_tags (mentor_id, tag_id) VALUES ($1, $2)",
			mentorID, tagID)
		if err != nil {
			return fmt.Errorf("failed to insert tag: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetAllTags retrieves all tags
func (r *MentorRepository) GetAllTags(ctx context.Context) (map[string]string, error) {
	return r.tagsCache.Get()
}

// GetByEmail retrieves a mentor by email address
func (r *MentorRepository) GetByEmail(ctx context.Context, email string) (*models.Mentor, error) {
	query := `
		SELECT id, airtable_id, legacy_id, slug, name, job_title, workplace, about, details,
			competencies, experience, price, status, '' as tags, telegram_chat_id, calendar_url,
			sort_order, created_at, 0 as mentee_count
		FROM mentors
		WHERE email = $1 AND status IN ('active', 'inactive')
		LIMIT 1
	`

	row := r.pool.QueryRow(ctx, query, email)
	return models.ScanMentor(row)
}

// GetByLoginToken retrieves a mentor by login token
func (r *MentorRepository) GetByLoginToken(ctx context.Context, token string) (*models.Mentor, string, time.Time, error) {
	query := `
		SELECT id, airtable_id, legacy_id, slug, name, job_title, workplace, about, details,
			competencies, experience, price, status, '' as tags, telegram_chat_id, calendar_url,
			sort_order, created_at, 0 as mentee_count, login_token_expires_at
		FROM mentors
		WHERE login_token = $1
		LIMIT 1
	`

	row := r.pool.QueryRow(ctx, query, token)

	var mentor models.Mentor
	var tagsStr *string
	var airtableID *string
	var telegramChatID *int64
	var expiresAt time.Time

	err := row.Scan(
		&mentor.MentorID,
		&airtableID,
		&mentor.LegacyID,
		&mentor.Slug,
		&mentor.Name,
		&mentor.Job,
		&mentor.Workplace,
		&mentor.About,
		&mentor.Description,
		&mentor.Competencies,
		&mentor.Experience,
		&mentor.Price,
		&mentor.Status,
		&tagsStr,
		&telegramChatID,
		&mentor.CalendarURL,
		&mentor.SortOrder,
		&mentor.CreatedAt,
		&mentor.MenteeCount,
		&expiresAt,
	)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	mentor.AirtableID = airtableID
	mentor.TelegramChatID = telegramChatID

	// Return the token that was used to find this mentor (already validated by SQL query)
	return &mentor, token, expiresAt, nil
}

// SetLoginToken sets the login token for a mentor
func (r *MentorRepository) SetLoginToken(ctx context.Context, mentorId string, token string, exp time.Time) error {
	query := `
		UPDATE mentors
		SET login_token = $1, login_token_expires_at = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.pool.Exec(ctx, query, token, exp, mentorId)
	return err
}

// ClearLoginToken clears the login token for a mentor
func (r *MentorRepository) ClearLoginToken(ctx context.Context, mentorId string) error {
	query := `
		UPDATE mentors
		SET login_token = NULL, login_token_expires_at = NULL, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, mentorId)
	return err
}

// FetchAllMentorsFromDB retrieves all mentors from PostgreSQL for cache population
func (r *MentorRepository) FetchAllMentorsFromDB(ctx context.Context) ([]*models.Mentor, error) {
	query := `
		SELECT m.id, m.airtable_id, m.legacy_id, m.slug, m.name, m.job_title, m.workplace,
			m.about, m.details, m.competencies, m.experience, m.price, m.status,
			COALESCE(array_to_string(array_agg(t.name), ','), '') as tags,
			m.telegram_chat_id, m.calendar_url, m.sort_order, m.created_at,
			COALESCE(
				(SELECT COUNT(*)
				 FROM client_requests cr
				 WHERE cr.mentor_id = m.id
				 AND cr.status = 'done'),
				0
			) AS mentee_count
		FROM mentors m
		LEFT JOIN mentor_tags mt ON mt.mentor_id = m.id
		LEFT JOIN tags t ON t.id = mt.tag_id
		WHERE m.status = 'active'
		GROUP BY m.id
		ORDER BY m.sort_order
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch mentors: %w", err)
	}

	return models.ScanMentors(rows)
}

// FetchSingleMentorFromDB retrieves a single mentor by slug from PostgreSQL
func (r *MentorRepository) FetchSingleMentorFromDB(ctx context.Context, mentorSlug string) (*models.Mentor, error) {
	query := `
		SELECT m.id, m.airtable_id, m.legacy_id, m.slug, m.name, m.job_title, m.workplace,
			m.about, m.details, m.competencies, m.experience, m.price, m.status,
			COALESCE(array_to_string(array_agg(t.name), ','), '') as tags,
			m.telegram_chat_id, m.calendar_url, m.sort_order, m.created_at,
			COALESCE(
				(SELECT COUNT(*)
				 FROM client_requests cr
				 WHERE cr.mentor_id = m.id
				 AND cr.status = 'done'),
				0
			) AS mentee_count
		FROM mentors m
		LEFT JOIN mentor_tags mt ON mt.mentor_id = m.id
		LEFT JOIN tags t ON t.id = mt.tag_id
		WHERE m.slug = $1
		GROUP BY m.id
	`

	row := r.pool.QueryRow(ctx, query, mentorSlug)
	return models.ScanMentor(row)
}

// FetchAllTagsFromDB retrieves all tags from PostgreSQL for cache population
func (r *MentorRepository) FetchAllTagsFromDB(ctx context.Context) (map[string]string, error) {
	query := `SELECT id, name FROM tags ORDER BY name`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer rows.Close()

	tags := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags[name] = id
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}

// applyFilters applies filtering options to a mentor list
func (r *MentorRepository) applyFilters(mentors []*models.Mentor, opts models.FilterOptions) []*models.Mentor {
	result := make([]*models.Mentor, 0, len(mentors))

	for _, mentor := range mentors {
		filtered := r.applySingleMentorFilters(mentor, opts)
		if filtered != nil {
			result = append(result, filtered)
		}
	}

	return result
}

// applySingleMentorFilters applies filtering options to a single mentor
// Returns nil if mentor should be filtered out
func (r *MentorRepository) applySingleMentorFilters(mentor *models.Mentor, opts models.FilterOptions) *models.Mentor {
	// Filter by visibility
	if opts.OnlyVisible && !mentor.IsVisible {
		return nil
	}

	// Only copy if modifications are needed
	if opts.DropLongFields || !opts.ShowHidden {
		m := *mentor // Copy only when necessary

		if opts.DropLongFields {
			m.About = ""
			m.Description = ""
		}

		if !opts.ShowHidden {
			m.CalendarURL = ""
		}

		return &m
	}

	// Return original pointer if no modifications needed
	return mentor
}

// InvalidateCache forces cache invalidation
func (r *MentorRepository) InvalidateCache() {
	r.mentorCache.Clear()
}

// UpdateSingleMentorCache updates a single mentor in cache
// Called by webhook or profile update flow
func (r *MentorRepository) UpdateSingleMentorCache(mentorSlug string) error {
	return r.mentorCache.UpdateSingleMentor(mentorSlug)
}

// RemoveMentorFromCache removes a mentor from cache
// Called when a mentor is deleted
func (r *MentorRepository) RemoveMentorFromCache(mentorSlug string) error {
	return r.mentorCache.RemoveMentor(mentorSlug)
}

// RefreshCache triggers a background cache refresh
func (r *MentorRepository) RefreshCache() error {
	_, err := r.mentorCache.ForceRefresh()
	return err
}
