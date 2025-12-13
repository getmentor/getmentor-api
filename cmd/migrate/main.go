package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getmentor/getmentor-api/config"
	"github.com/getmentor/getmentor-api/internal/database/postgres"
	"github.com/getmentor/getmentor-api/pkg/airtable"
	"github.com/getmentor/getmentor-api/pkg/logger"
	"go.uber.org/zap"
)

//nolint:gocyclo // Migration main has expected complexity for initialization logic
func main() {
	// Parse command line flags
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without writing to PostgreSQL")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	migrateAll := flag.Bool("all", false, "Migrate all data (mentors, tags, client requests)")
	migrateMentors := flag.Bool("mentors", false, "Migrate mentors and tags only")
	migrateRequests := flag.Bool("requests", false, "Migrate client requests only")
	flag.Parse()

	if !*migrateAll && !*migrateMentors && !*migrateRequests {
		fmt.Println("Usage: migrate [flags]")
		fmt.Println()
		fmt.Println("Flags:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  migrate --all                 # Migrate all data")
		fmt.Println("  migrate --mentors --dry-run   # Dry run for mentors")
		os.Exit(1)
	}

	// Initialize logger
	logLevel := "info"
	if *verbose {
		logLevel = "debug"
	}
	if err := logger.Initialize(logger.Config{
		Level:       logLevel,
		Environment: "development",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Airtable to PostgreSQL migration",
		zap.Bool("dry_run", *dryRun),
		zap.Bool("migrate_mentors", *migrateMentors || *migrateAll),
		zap.Bool("migrate_requests", *migrateRequests || *migrateAll),
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Validate PostgreSQL is enabled
	if !cfg.Postgres.Enabled {
		logger.Fatal("PostgreSQL must be enabled (POSTGRES_ENABLED=true)")
	}

	// Initialize Airtable client
	airtableClient, err := airtable.NewClient(
		cfg.Airtable.APIKey,
		cfg.Airtable.BaseID,
		false, // Don't work offline
	)
	if err != nil {
		logger.Fatal("Failed to initialize Airtable client", zap.Error(err))
	}

	// Initialize PostgreSQL client
	ctx := context.Background()
	pgConfig := &postgres.Config{
		Host:         cfg.Postgres.Host,
		Port:         cfg.Postgres.Port,
		Database:     cfg.Postgres.Database,
		User:         cfg.Postgres.User,
		Password:     cfg.Postgres.Password,
		SSLMode:      cfg.Postgres.SSLMode,
		MaxConns:     int32(cfg.Postgres.MaxConns),
		MinConns:     int32(cfg.Postgres.MinConns),
		MaxConnLife:  time.Duration(cfg.Postgres.MaxConnLife) * time.Second,
		MaxConnIdle:  time.Duration(cfg.Postgres.MaxConnIdle) * time.Second,
		HealthPeriod: time.Duration(cfg.Postgres.HealthPeriod) * time.Second,
	}

	pgClient, err := postgres.NewClient(ctx, pgConfig)
	if err != nil {
		logger.Fatal("Failed to initialize PostgreSQL client", zap.Error(err))
	}
	defer pgClient.Close()

	// Verify connection
	if err := pgClient.Ping(ctx); err != nil {
		logger.Fatal("Failed to ping PostgreSQL", zap.Error(err))
	}
	logger.Info("Connected to PostgreSQL")

	// Run migrations
	migrator := &Migrator{
		airtable: airtableClient,
		pg:       pgClient,
		dryRun:   *dryRun,
	}

	if *migrateMentors || *migrateAll {
		if err := migrator.MigrateTags(ctx); err != nil {
			logger.Fatal("Failed to migrate tags", zap.Error(err))
		}
		if err := migrator.MigrateMentors(ctx); err != nil {
			logger.Fatal("Failed to migrate mentors", zap.Error(err))
		}
	}

	if *migrateRequests || *migrateAll {
		if err := migrator.MigrateClientRequests(ctx); err != nil {
			logger.Fatal("Failed to migrate client requests", zap.Error(err))
		}
	}

	logger.Info("Migration completed successfully!")
}

// Migrator handles the data migration from Airtable to PostgreSQL
type Migrator struct {
	airtable *airtable.Client
	pg       *postgres.Client
	dryRun   bool
	tagMap   map[string]int // Airtable tag name -> PostgreSQL tag ID
}

// MigrateTags migrates tags from Airtable to PostgreSQL
func (m *Migrator) MigrateTags(ctx context.Context) error {
	logger.Info("Migrating tags...")

	tags, err := m.airtable.GetAllTags()
	if err != nil {
		return fmt.Errorf("failed to fetch tags from Airtable: %w", err)
	}

	logger.Info("Found tags in Airtable", zap.Int("count", len(tags)))

	if m.dryRun {
		for name, id := range tags {
			logger.Debug("Would migrate tag", zap.String("name", name), zap.String("airtable_id", id))
		}
		return nil
	}

	m.tagMap = make(map[string]int)

	for name := range tags {
		// Check if tag already exists
		existingID, err := m.pg.GetTagIDByName(ctx, name)
		if err == nil && existingID > 0 {
			m.tagMap[name] = existingID
			logger.Debug("Tag already exists", zap.String("name", name), zap.Int("id", existingID))
			continue
		}

		// Create new tag
		newID, err := m.pg.CreateTag(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to create tag %s: %w", name, err)
		}
		m.tagMap[name] = newID
		logger.Debug("Created tag", zap.String("name", name), zap.Int("id", newID))
	}

	logger.Info("Tags migration completed", zap.Int("count", len(m.tagMap)))
	return nil
}

// MigrateMentors migrates mentors from Airtable to PostgreSQL
func (m *Migrator) MigrateMentors(ctx context.Context) error {
	logger.Info("Migrating mentors...")

	mentors, err := m.airtable.GetAllMentors()
	if err != nil {
		return fmt.Errorf("failed to fetch mentors from Airtable: %w", err)
	}

	logger.Info("Found mentors in Airtable", zap.Int("count", len(mentors)))

	if m.dryRun {
		for _, mentor := range mentors {
			logger.Debug("Would migrate mentor",
				zap.Int("id", mentor.ID),
				zap.String("slug", mentor.Slug),
				zap.String("name", mentor.Name),
			)
		}
		return nil
	}

	// Get database pool for direct queries
	pool := m.pg.Pool()

	migrated := 0
	skipped := 0

	for _, mentor := range mentors {
		// Check if mentor already exists
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM mentors WHERE airtable_id = $1)", mentor.AirtableID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check mentor existence: %w", err)
		}

		if exists {
			skipped++
			logger.Debug("Mentor already exists, skipping",
				zap.String("airtable_id", mentor.AirtableID),
				zap.String("slug", mentor.Slug),
			)
			continue
		}

		// Insert mentor
		query := `
			INSERT INTO mentors (
				airtable_id, mentor_id, slug, name, job_title, workplace,
				details, about, competencies, experience, price, sessions_count,
				sort_order, is_visible, status, auth_token, calendar_url, is_new,
				image_url, telegram_username
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, $12,
				$13, $14, $15, $16, $17, $18,
				$19, $20
			) RETURNING id
		`

		var mentorPK int
		err = pool.QueryRow(ctx, query,
			mentor.AirtableID,
			mentor.ID,
			mentor.Slug,
			mentor.Name,
			nilIfEmpty(mentor.Job),
			nilIfEmpty(mentor.Workplace),
			nilIfEmpty(mentor.Description),
			nilIfEmpty(mentor.About),
			nilIfEmpty(mentor.Competencies),
			nilIfEmpty(mentor.Experience),
			nilIfEmpty(mentor.Price),
			mentor.MenteeCount,
			mentor.SortOrder,
			mentor.IsVisible,
			"active", // Default status
			nilIfEmpty(mentor.AuthToken),
			nilIfEmpty(mentor.CalendarURL),
			mentor.IsNew,
			nil, // image_url - will need separate handling
			nil, // telegram_username - will need separate handling
		).Scan(&mentorPK)
		if err != nil {
			return fmt.Errorf("failed to insert mentor %s: %w", mentor.Slug, err)
		}

		// Insert mentor tags
		if len(mentor.Tags) > 0 && m.tagMap != nil {
			for _, tagName := range mentor.Tags {
				tagID, ok := m.tagMap[tagName]
				if !ok {
					logger.Warn("Tag not found in tagMap", zap.String("tag", tagName))
					continue
				}

				_, err = pool.Exec(ctx,
					"INSERT INTO mentor_tags (mentor_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
					mentorPK, tagID)
				if err != nil {
					logger.Error("Failed to insert mentor tag",
						zap.String("mentor", mentor.Slug),
						zap.String("tag", tagName),
						zap.Error(err),
					)
				}
			}
		}

		migrated++
		logger.Debug("Migrated mentor",
			zap.String("slug", mentor.Slug),
			zap.Int("pk", mentorPK),
		)
	}

	logger.Info("Mentors migration completed",
		zap.Int("migrated", migrated),
		zap.Int("skipped", skipped),
	)
	return nil
}

// MigrateClientRequests migrates client requests from Airtable to PostgreSQL
//
//nolint:unparam // Returns nil for now, will return errors when fully implemented
func (m *Migrator) MigrateClientRequests(_ context.Context) error {
	logger.Info("Migrating client requests...")
	logger.Warn("Client request migration not fully implemented - Airtable client needs GetAllClientRequests method")

	// TODO: Implement when Airtable client has GetAllClientRequests method
	// For now, this is a placeholder showing the structure

	if m.dryRun {
		logger.Info("Dry run: would migrate client requests")
		return nil
	}

	// The actual implementation would:
	// 1. Fetch all client requests from Airtable
	// 2. Map mentor Airtable IDs to PostgreSQL IDs
	// 3. Insert each request with proper status mapping using mapAirtableStatus()

	logger.Info("Client requests migration skipped (not implemented)")
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// mapAirtableStatus maps Airtable status values to PostgreSQL status values
// Used when MigrateClientRequests is fully implemented
//
//nolint:unused // Will be used when client request migration is implemented
func mapAirtableStatus(airtableStatus string) string {
	statusMap := map[string]string{
		"Ожидает":    "pending",
		"В работе":   "working",
		"Связались":  "contacted",
		"Завершено":  "done",
		"Отклонено":  "declined",
		"Недоступен": "unavailable",
		"Перенесено": "reschedule",
	}

	if mapped, ok := statusMap[strings.TrimSpace(airtableStatus)]; ok {
		return mapped
	}
	return "pending"
}
