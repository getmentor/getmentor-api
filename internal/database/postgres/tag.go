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

// GetAllTags fetches all tags as a map of name -> ID
func (c *Client) GetAllTags(ctx context.Context) (map[string]string, error) {
	start := time.Now()
	operation := "getAllTags"

	query := "SELECT id, name FROM tags ORDER BY name"

	rows, err := c.pool.Query(ctx, query)
	if err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	tags := make(map[string]string)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			duration := metrics.MeasureDuration(start)
			recordMetrics(operation, "error", duration)
			return nil, fmt.Errorf("failed to scan tag row: %w", err)
		}
		// Use string ID for compatibility with existing code
		tags[name] = fmt.Sprintf("%d", id)
	}

	if err := rows.Err(); err != nil {
		duration := metrics.MeasureDuration(start)
		recordMetrics(operation, "error", duration)
		return nil, fmt.Errorf("error iterating tag rows: %w", err)
	}

	duration := metrics.MeasureDuration(start)
	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.Int("count", len(tags)))

	return tags, nil
}

// GetTagIDByName returns the internal ID of a tag by its name
func (c *Client) GetTagIDByName(ctx context.Context, name string) (int, error) {
	start := time.Now()
	operation := "getTagIDByName"

	var id int
	err := c.pool.QueryRow(ctx, "SELECT id FROM tags WHERE name = $1", name).Scan(&id)

	duration := metrics.MeasureDuration(start)

	if err == pgx.ErrNoRows {
		recordMetrics(operation, "not_found", duration)
		return 0, fmt.Errorf("tag with name %s not found", name)
	}
	if err != nil {
		recordMetrics(operation, "error", duration)
		return 0, fmt.Errorf("failed to query tag: %w", err)
	}

	recordMetrics(operation, "success", duration)
	return id, nil
}

// CreateTag creates a new tag
func (c *Client) CreateTag(ctx context.Context, name string) (int, error) {
	start := time.Now()
	operation := "createTag"

	var id int
	err := c.pool.QueryRow(ctx,
		"INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = $1 RETURNING id",
		name).Scan(&id)

	duration := metrics.MeasureDuration(start)

	if err != nil {
		recordMetrics(operation, "error", duration)
		logger.LogAPICall("postgres", operation, "error", duration, zap.Error(err))
		return 0, fmt.Errorf("failed to create tag: %w", err)
	}

	recordMetrics(operation, "success", duration)
	logger.LogAPICall("postgres", operation, "success", duration, zap.String("name", name))

	return id, nil
}
