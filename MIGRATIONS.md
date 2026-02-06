# Database Migrations

This document explains how to run database migrations for the GetMentor API.

## Overview

Migrations are now run as a **separate process** before the application starts. This approach:
- ✅ Prevents race conditions in multi-instance deployments
- ✅ Allows zero-downtime deployments
- ✅ Separates schema management from application logic
- ✅ Enables independent monitoring of migrations

## Migration Tool

The migration tool is a separate binary (`bin/migrate`) built from `cmd/migrate/main.go`.

## Local Development

### Quick Start

Run migrations with Make:
```bash
make migrate
```

This will:
1. Build the migrate binary
2. Load environment from `.env`
3. Run all pending migrations

### Manual Steps

Build the migrate binary:
```bash
go build -o bin/migrate cmd/migrate/main.go
# or
make migrate-build
```

Run migrations:
```bash
./bin/migrate
# or
./scripts/migrate.sh
```

### Script Options

```bash
# Run migrations
./scripts/migrate.sh

# Build first, then run
./scripts/migrate.sh --build
```

## Production Deployment (Docker Compose)

Migrations run automatically before the backend starts via a dedicated service.

### Docker Compose Configuration

```yaml
services:
  migrate:
    image: getmentor-backend:latest
    command: ["/app/migrate"]
    restart: "no"  # Run once and exit

  backend:
    image: getmentor-backend:latest
    depends_on:
      migrate:
        condition: service_completed_successfully
```

### Deployment Flow

1. **Pull new image:**
   ```bash
   docker-compose pull
   ```

2. **Run migrations:**
   ```bash
   docker-compose up migrate
   ```

   The migrate service:
   - Runs once
   - Executes all pending migrations
   - Exits with status 0 on success
   - Exits with non-zero on failure

3. **Start application:**
   ```bash
   docker-compose up -d backend
   ```

   Backend starts only after migrations complete successfully.

### Full Deployment

Standard deployment runs migrations automatically:
```bash
docker-compose up -d
```

This:
1. Starts migrate service first
2. Waits for it to complete successfully
3. Then starts backend service

### Manual Migration Run

Run migrations manually without restarting the app:
```bash
docker-compose run --rm migrate
```

## Migration Files

Location: `migrations/`

Naming convention:
- Up migrations: `NNNN_description.up.sql`
- Down migrations: `NNNN_description.down.sql`

Example:
```
migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_tags_table.up.sql
└── 000002_add_tags_table.down.sql
```

## Migration Tracking

The migration tool uses a tracking table (`schema_migrations`) to record which migrations have been applied.

**Important:** Migrations are idempotent - they only run once per version.

## Common Tasks

### Check migration status

```bash
# Local
./bin/migrate

# Docker
docker-compose run --rm migrate
```

If no new migrations, output: "Database migrations completed successfully"

### Create new migration

```bash
migrate create -ext sql -dir migrations -seq description_here
```

This creates:
- `NNNN_description_here.up.sql`
- `NNNN_description_here.down.sql`

### Rollback migration (USE WITH CAUTION)

Down migrations are **NOT** run automatically. Only use in emergencies.

```bash
# Not yet implemented in the migrate binary
# Would require adding a --down flag
```

## Environment Variables

Required:
- `DATABASE_URL` - PostgreSQL connection string

Optional:
- `DATABASE_TLS_SERVER_NAME` - For production SSL connections
- `LOG_LEVEL` - Logging verbosity (default: info)

## Troubleshooting

### Migration fails in Docker

Check logs:
```bash
docker-compose logs migrate
```

### Migration succeeds but app doesn't start

Check backend logs:
```bash
docker-compose logs backend
```

### Reset migrations in development (DANGEROUS)

⚠️ This will DROP ALL DATA:

```bash
# Drop all tables
psql $DATABASE_URL -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Re-run migrations
make migrate
```

### Multiple instances trying to migrate

If running multiple backend replicas, ensure only ONE migrate service runs:
- Docker Compose: Already configured correctly
- Kubernetes: Use a Job or init container
- Manual: Run migrate separately before scaling up

## Best Practices

1. **Always test migrations** in development/staging first
2. **Keep migrations small** and focused on one change
3. **Make migrations reversible** when possible (write down scripts)
4. **Never modify** an already-applied migration
5. **Always run migrations** before deploying new application code
6. **Monitor migration duration** - long migrations need special handling

## Architecture

```
┌─────────────────┐
│ Docker Compose  │
└────────┬────────┘
         │
    ┌────▼─────┐
    │ migrate  │  (runs once, exits)
    └────┬─────┘
         │
    ┌────▼─────┐
    │ backend  │  (starts after migrate completes)
    └──────────┘
```

## See Also

- [golang-migrate documentation](https://github.com/golang-migrate/migrate)
- Main application code: `cmd/api/main.go`
- Migration tool code: `cmd/migrate/main.go`
- Migration library: `pkg/db/migrate.go`
