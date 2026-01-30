# Airtable to PostgreSQL Migration Plan

## Overview

Migrate all four services (getmentor-api, getmentor-func, getmentor-bot, getmentor.dev) from Airtable to PostgreSQL in a single hard cutover. The Go API uses `jackc/pgx` for database access. Azure Functions and Telegram Bot get direct PostgreSQL connections via a Node.js `pg` client. Schema migrations are managed by `golang-migrate`. Data migration script already exists.

## Key Decisions

- **Library**: `jackc/pgx/v5` (pure Go, pool via `pgxpool`)
- **Migration tool**: `golang-migrate/migrate` with `file://` source
- **Func/Bot DB access**: Direct PostgreSQL via `pg` npm package (not through Go API)
- **Cutover**: Hard cutover (no dual-write period)
- **IDs**: Keep `airtable_id` column for backwards compatibility during transition; primary keys become UUIDs
- **Caching**: Keep existing in-memory cache layer in Go API but source data from PostgreSQL instead of Airtable API
- **Webhook flow**: Airtable webhooks disappear. Go API becomes the source of truth and triggers Azure Functions directly via HTTP (existing trigger mechanism stays)

## Architecture Changes

### Before
```
Go API -> Airtable API -> Airtable
Func   -> Airtable API -> Airtable
Bot    -> Airtable API -> Airtable
```

### After
```
Go API -> pgxpool -> PostgreSQL
Func   -> pg      -> PostgreSQL
Bot    -> pg      -> PostgreSQL
```

### What stays the same
- Frontend (`getmentor.dev`) continues calling Go API via HTTP - no direct DB access
- Frontend types (`MentorBase`, `MentorWithSecureFields`, etc.) remain unchanged
- All Go API HTTP endpoints and response formats remain unchanged
- Cache layer remains (just sources from PG instead of Airtable)
- Azure Function trigger mechanism (Go API calls func URLs) remains
- Event triggers for new mentor/request/login email remain

### What changes
- `pkg/airtable/` package is replaced by `pkg/db/` (pgxpool wrapper)
- Repositories get rewritten to use SQL queries instead of Airtable client
- Models lose `AirtableRecordToMentor()` / `AirtableRecordToMentorClientRequest()` converters
- Models gain `pgx` scan methods
- `mehanizm/airtable` dependency removed
- Airtable webhook handler (`POST /api/webhooks/airtable`) removed
- Config: `AIRTABLE_*` env vars replaced by `DATABASE_URL`
- Func/Bot: `airtable` npm package replaced by `pg`
- `ContactMentorRequest.MentorAirtableID` field needs renaming/mapping

## Critical Files to Change

### getmentor-api (Go backend)
| File | Change |
|------|--------|
| `go.mod` | Add `pgx/v5`, `golang-migrate`; remove `mehanizm/airtable` |
| `config/config.go` | Replace `AirtableConfig` with `DatabaseConfig` (DATABASE_URL) |
| `cmd/api/main.go` | Init pgxpool, run migrations, pass pool to repos |
| `pkg/airtable/client.go` | **DELETE** entire file |
| `pkg/db/pool.go` | **NEW** - pgxpool connection wrapper |
| `pkg/db/migrate.go` | **NEW** - golang-migrate runner |
| `internal/models/mentor.go` | Remove `AirtableRecord`, `AirtableRecordToMentor()`, add scan helpers |
| `internal/models/mentor_client_request.go` | Remove `AirtableRecordToMentorClientRequest()`, add scan helpers |
| `internal/models/contact.go` | Change `MentorAirtableID` to `MentorID` (UUID) |
| `internal/models/webhook.go` | Remove `WebhookPayload` (Airtable webhooks gone) |
| `internal/models/mentor_session.go` | Change `AirtableID string` to `MentorUUID string` |
| `internal/repository/mentor_repository.go` | Rewrite all methods to use SQL queries |
| `internal/repository/client_request_repository.go` | Rewrite all methods to use SQL queries |
| `internal/cache/mentor_cache.go` | Change data source from `airtable.Client` to repository/pool |
| `internal/cache/tags_cache.go` | Change data source from `airtable.Client` to repository/pool |
| `internal/services/webhook_service.go` | Remove Airtable webhook handling |
| `internal/services/contact_service.go` | Use UUID mentor ID instead of Airtable record ID |
| `internal/services/registration_service.go` | Write to PG instead of Airtable |
| `internal/services/profile_service.go` | Update to PG; remove Airtable field names |
| `internal/services/mentor_auth_service.go` | Use PG for login token storage |
| `internal/services/mentor_requests_service.go` | Minor: IDs become UUIDs |
| `internal/services/interfaces.go` | Update method signatures for UUID IDs |
| `internal/handlers/webhook_handler.go` | Remove or repurpose |
| `internal/handlers/mentor_profile_handler.go` | Adapt to UUID-based lookups |
| `internal/handlers/mentor_requests_handler.go` | Adapt to UUID-based lookups |
| `internal/middleware/mentor_session.go` | Update session to use UUID |
| `pkg/metrics/metrics.go` | Rename `Airtable*` metrics to `DB*` / `Postgres*` |
| `pkg/trigger/trigger.go` | No change (already generic) |
| `pkg/jwt/jwt.go` | Update claims to use UUID instead of AirtableID |
| `test/pkg/airtable/client_test.go` | **DELETE** or rewrite as `db/pool_test.go` |
| `test/internal/models/mentor_test.go` | Update for new model structure |
| `migrations/` | Restructure for golang-migrate (numbered up/down files) |
| `.env.example` | Replace `AIRTABLE_*` with `DATABASE_URL` |
| `Dockerfile` | Possibly add migrate step |

### getmentor-func (Azure Functions)
| File | Change |
|------|--------|
| `package.json` | Add `pg`; remove `airtable` |
| `lib/utils/airtable.ts` | **DELETE** |
| `lib/utils/db.ts` | **NEW** - PostgreSQL connection pool |
| `lib/data/mentor.ts` | Rewrite constructor to accept PG row instead of Airtable record |
| `new-request-watcher/index.ts` | Replace `AirtableBase('Client Requests').find()` with PG query |
| `new-mentor-watcher/index.ts` | Replace `AirtableBase('Mentors').find/update()` with PG queries |
| `request-process-finished/index.ts` | Replace Airtable calls with PG queries |
| `sessions-watcher/index.ts` | Replace Airtable calls with PG queries |
| `adm-bot-listener/index.ts` | Replace Airtable calls with PG queries |
| `process-mentee-review/index.ts` | Replace Airtable calls with PG queries |
| `mentor-login-email/index.ts` | Replace Airtable calls with PG queries |
| `update-mentor-image/index.ts` | Replace Airtable calls with PG queries |
| `randomize-sort-order/index.ts` | Replace Airtable calls with PG queries |
| `update-status-reminder/index.ts` | Replace Airtable calls with PG queries |
| `tg-mass-send/index.ts` | Replace Airtable calls with PG queries |
| `local.settings.json` / env | Add `DATABASE_URL`; remove `AIRTABLE_*` |

### getmentor-bot (Telegram Bot)
| File | Change |
|------|--------|
| `package.json` | Add `pg`; remove `airtable` |
| `lib/storage/airtable/AirtableBase.ts` | **DELETE** |
| `lib/storage/MentorStorage.ts` | Update interface (keep same contract) |
| `lib/storage/postgres/PostgresStorage.ts` | **NEW** - implements MentorStorage with PG |
| `getmentor-bot/index.ts` | Instantiate PostgresStorage instead of AirtableBase |
| `lib/models/Mentor.ts` | Update constructor for PG row |
| `lib/models/MentorClientRequest.ts` | Update constructor for PG row |
| `local.settings.json` / env | Add `DATABASE_URL`; remove `AIRTABLE_*` |

### getmentor.dev (Frontend)
| File | Change |
|------|--------|
| `src/types/mentor.ts` | Change `airtableId` to `id` (UUID string) throughout |
| `src/lib/go-api-client.ts` | Rename `getOneMentorByRecordId` to `getOneMentorById` (UUID) |
| `src/pages/mentor/[slug]/contact.tsx` | Use UUID `id` instead of `airtableId` for contact form |
| `src/components/hooks/useMentors.ts` | Update any `airtableId` references |
| Various test files | Update mocks for UUID-based IDs |

### getmentor-infra (Infrastructure)
| File | Change |
|------|--------|
| `docker-compose.yml` | Add PostgreSQL service |
| `docker-compose.dev.yml` | Add PostgreSQL service for dev |
| `.env.example` | Add `DATABASE_URL` |

---

## Detailed Task Breakdown

### Phase 0: Infrastructure

#### Task 0.1: Add PostgreSQL to Docker Compose
**Repo**: `getmentor-infra`
**Files**: `docker-compose.yml`, `docker-compose.dev.yml`, `.env.example`
**Details**:
- Add a `postgres` service using `postgres:16-alpine` image
- Mount a named volume `pgdata` for persistence
- Expose port 5432 internally (not publicly)
- Add `DATABASE_URL` to `.env.example`: `postgres://getmentor:password@postgres:5432/getmentor?sslmode=disable`
- Add healthcheck: `pg_isready -U getmentor`
- Make `backend` service depend on `postgres` with `condition: service_healthy`
- In dev compose, optionally expose 5432 to host for local tools

#### Task 0.2: Set up golang-migrate in Go API
**Repo**: `getmentor-api`
**Files**: `go.mod`, `pkg/db/migrate.go` (new), `migrations/` restructure
**Details**:
- Run `go get github.com/golang-migrate/migrate/v4`
- Run `go get github.com/golang-migrate/migrate/v4/database/postgres`
- Run `go get github.com/golang-migrate/migrate/v4/source/file`
- Create `pkg/db/migrate.go` with a `RunMigrations(databaseURL, migrationsPath string) error` function
- Uses `migrate.NewWithDatabaseInstance()` with postgres driver
- Calls `m.Up()`, ignores `migrate.ErrNoChange`
- Restructure `migrations/` folder:
    - Rename `001_schema.sql` to `000001_initial_schema.up.sql`
    - Create `000001_initial_schema.down.sql` with `DROP TABLE` statements (in reverse dependency order: `reviews`, `mentor_tags`, `client_requests`, `moderators`, `tags`, `mentors`)
    - Remove `002_staging.sql` (deleted in git status already)
- Write unit test `pkg/db/migrate_test.go` that verifies migration files exist and are parseable

### Phase 1: Go API Database Layer

#### Task 1.1: Add pgxpool connection package
**Repo**: `getmentor-api`
**Files**: `go.mod`, `pkg/db/pool.go` (new), `config/config.go`
**Details**:
- Run `go get github.com/jackc/pgx/v5`
- Create `pkg/db/pool.go`:
    - `func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error)` - creates connection pool with sensible defaults (max 10 conns, min 2, health check period 30s)
    - `func Close(pool *pgxpool.Pool)` - closes pool
    - Pool config: `MaxConns=10, MinConns=2, HealthCheckPeriod=30s, MaxConnLifetime=1h, MaxConnIdleTime=30m`
- In `config/config.go`:
    - Add `DatabaseConfig` struct: `type DatabaseConfig struct { URL string; MaxConns int32; MinConns int32 }`
    - Replace `AirtableConfig` with `DatabaseConfig`
    - In `Load()`: read `DATABASE_URL` env var (required unless work offline mode)
    - Keep `WorkOffline` bool in config for dev mode
    - Update `Validate()`: require `DATABASE_URL` when not offline
- Write test `test/pkg/db/pool_test.go` that verifies pool creation with invalid URL returns error

#### Task 1.2: Update Mentor model for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/models/mentor.go`
**Details**:
- Remove `import "github.com/mehanizm/airtable"`
- Remove `AirtableRecord` struct entirely
- Remove `ToMentor()` method on `AirtableRecord`
- Remove `AirtableRecordToMentor()` function
- Change `Mentor` struct:
    - Add `UUID string` field (for the PG UUID primary key)
    - Keep `ID int` (maps to `legacy_id`)
    - Keep `AirtableID string` temporarily (maps to `airtable_id` column for backwards compat)
    - Keep all other fields the same
- Add `ScanMentor(row pgx.Row) (*Mentor, error)` function that scans a PG row into a Mentor struct
- Add `ScanMentors(rows pgx.Rows) ([]*Mentor, error)` function for bulk scanning
- Keep `GetCalendarType()`, `GetMentorSponsor()`, `ToPublicResponse()`, `FilterOptions` unchanged
- Write test: `test/internal/models/mentor_scan_test.go` - test `ScanMentor` with mock row (can use `pgx/pgxtest` or manual mock)

#### Task 1.3: Update MentorClientRequest model for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/models/mentor_client_request.go`
**Details**:
- Remove `import "github.com/mehanizm/airtable"`
- Remove `AirtableRecordToMentorClientRequest()` function
- Change `MentorClientRequest` struct:
    - `ID` changes from `string` (Airtable rec ID) to `string` (UUID as string)
    - `MentorID` changes from `string` (Airtable rec ID) to `string` (UUID as string)
    - Add `MentorUUID string` if needed for joins
- Add `ScanClientRequest(row pgx.Row) (*MentorClientRequest, error)` function
- Add `ScanClientRequests(rows pgx.Rows) ([]*MentorClientRequest, error)` function
- Keep all status types, `RequestStatus`, `DeclineReason`, transition logic unchanged
- Write test: verify scan functions work correctly

#### Task 1.4: Update remaining models
**Repo**: `getmentor-api`
**Files**: `internal/models/contact.go`, `internal/models/webhook.go`, `internal/models/mentor_session.go`
**Details**:
- `contact.go`: Change `MentorAirtableID string` to `MentorID string` (UUID). Update JSON tag from `mentorAirtableId` to `mentorId`. Update binding tag from `startswith=rec` to `uuid` (or just `required`).
- `webhook.go`: Remove `WebhookPayload` struct (Airtable webhooks no longer needed). Keep `RevalidateNextJSRequest` and `RevalidateNextJSResponse` (ISR revalidation still needed).
- `mentor_session.go`: Change `AirtableID string` to `MentorUUID string`. Update JSON tag. Update `MentorLoginData` similarly.

#### Task 1.5: Rewrite MentorRepository for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/repository/mentor_repository.go`
**Details**:
- Remove `import "github.com/getmentor/getmentor-api/pkg/airtable"`
- Change struct: replace `airtableClient *airtable.Client` with `pool *pgxpool.Pool`
- Constructor: `NewMentorRepository(pool *pgxpool.Pool, mentorCache *cache.MentorCache, tagsCache *cache.TagsCache) *MentorRepository`
- Rewrite methods:
    - `GetAll()`: `SELECT id, airtable_id, legacy_id, slug, name, ... FROM mentors WHERE status = 'active' ORDER BY sort_order` (still uses cache, same pattern)
    - `GetByID()`: Stays as cache lookup (unchanged logic)
    - `GetBySlug()`: Stays as cache lookup (unchanged logic)
    - `GetByRecordID()` -> rename to `GetByUUID()`: cache lookup by UUID
    - `Update()`: `UPDATE mentors SET name=$1, job_title=$2, ... WHERE id=$3`
    - `UpdateImage()`: `UPDATE mentors SET ... WHERE id=$1` (no more Airtable attachment format)
    - `CreateMentor()`: `INSERT INTO mentors (...) VALUES (...) RETURNING id, legacy_id` - returns UUID and legacy_id
    - `GetTagIDByName()`: cache lookup (unchanged)
    - `GetAllTags()`: cache lookup (unchanged)
    - `GetByEmail()`: `SELECT ... FROM mentors WHERE email = $1 AND status IN ('active', 'inactive') LIMIT 1`
    - `GetByLoginToken()`: `SELECT ... FROM mentors WHERE login_token = $1 LIMIT 1`
    - `SetLoginToken()`: `UPDATE mentors SET login_token = $1, login_token_expires_at = $2 WHERE id = $3`
    - `ClearLoginToken()`: `UPDATE mentors SET login_token = NULL, login_token_expires_at = NULL WHERE id = $1`
    - Keep cache methods: `InvalidateCache()`, `UpdateSingleMentorCache()`, `RemoveMentorFromCache()`, `RefreshCache()`
- Add a method to fetch all mentors from PG (for cache population): `fetchAllMentorsFromDB(ctx context.Context) ([]*models.Mentor, error)`
- Write test: `test/internal/repository/mentor_repository_test.go` with integration tests using testcontainers or a test database

#### Task 1.6: Rewrite ClientRequestRepository for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/repository/client_request_repository.go`
**Details**:
- Remove Airtable import
- Change struct: `pool *pgxpool.Pool` instead of `airtableClient`
- Constructor: `NewClientRequestRepository(pool *pgxpool.Pool)`
- Rewrite methods:
    - `Create()`: `INSERT INTO client_requests (mentor_id, email, name, telegram, description, level, status) VALUES ($1, $2, $3, $4, $5, $6, 'pending') RETURNING id` - returns UUID
    - `GetByMentor()`: `SELECT ... FROM client_requests WHERE mentor_id = $1 AND status = ANY($2) ORDER BY created_at ASC`
    - `GetByID()`: `SELECT ... FROM client_requests WHERE id = $1`
    - `UpdateStatus()`: `UPDATE client_requests SET status = $1, status_changed_at = NOW() WHERE id = $2`
    - `UpdateDecline()`: `UPDATE client_requests SET status = 'declined', decline_reason = $1, decline_comment = $2, status_changed_at = NOW() WHERE id = $3`
- Write test: `test/internal/repository/client_request_repository_test.go`

#### Task 1.7: Update cache layer for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/cache/mentor_cache.go`, `internal/cache/tags_cache.go`
**Details**:
- `mentor_cache.go`:
    - Replace `airtableClient *airtable.Client` with a `fetcher` function type: `type MentorFetcher func(ctx context.Context) ([]*models.Mentor, error)`
    - Constructor: `NewMentorCache(fetcher MentorFetcher, ttlSeconds int)`
    - `UpdateSingleMentor()`: Instead of calling `airtableClient.GetMentorBySlug()`, call a single-mentor fetcher: `type SingleMentorFetcher func(ctx context.Context, slug string) (*models.Mentor, error)`
    - Or simpler: pass the repository reference and call its `fetchAllMentorsFromDB` method
    - All cache logic (slug-based storage, periodic refresh, etc.) stays identical
- `tags_cache.go`:
    - Replace `airtableClient *airtable.Client` with a fetcher: `type TagsFetcher func(ctx context.Context) (map[string]string, error)`
    - The fetcher will be: `SELECT id, name FROM tags` -> map[name]id
    - Constructor: `NewTagsCache(fetcher TagsFetcher)`
- Write test: verify cache still works with mock fetcher functions

### Phase 2: Go API Services & Handlers

#### Task 2.1: Update ContactService for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/services/contact_service.go`
**Details**:
- `SubmitContactForm()`: Change `MentorAirtableID` to `MentorID` (UUID)
- `ClientRequest.MentorID` is now a UUID string instead of an Airtable record ID
- `GetByRecordID()` call changes to `GetByUUID()` or equivalent
- No other logic changes needed - the trigger mechanism stays the same
- Write test: verify contact form submission still works with UUID-based mentor ID

#### Task 2.2: Update RegistrationService for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/services/registration_service.go`
**Details**:
- Remove Airtable field name map (`airtableFields`). Instead, pass a proper struct or direct SQL params to `CreateMentor`
- `CreateMentor()` return value changes: returns `(uuid string, legacyID int, error)` instead of `(recordID, mentorID, error)`
- `UpdateImage()` now takes UUID instead of Airtable record ID
- Trigger calls: `trigger.CallAsync(url, uuid, httpClient)` - the func needs to accept UUID instead of Airtable ID
- Write test

#### Task 2.3: Update ProfileService for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/services/profile_service.go`
**Details**:
- `SaveProfile()`: Replace Airtable field name map with SQL update. `s.mentorRepo.Update()` takes UUID and a struct/map of updates.
- `SaveProfileByAirtableID()` -> rename to `SaveProfileByUUID()` or merge with `SaveProfile()`
- `UploadPictureByAirtableID()` -> rename to `UploadPictureByUUID()`
- Tag handling: `GetTagIDByName` returns UUID now instead of Airtable record ID. The `mentor_tags` join table uses UUIDs.
- Update logic: instead of building `map[string]interface{}` with Airtable field names like `"JobTitle"`, use SQL column names like `job_title`
- Write test

#### Task 2.4: Update MentorAuthService for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/services/mentor_auth_service.go`
**Details**:
- `RequestLogin()`: `mentor.AirtableID` -> `mentor.UUID`
- `SetLoginToken()`: uses UUID
- `VerifyLogin()`: `GetByLoginToken()` returns mentor with UUID
- `ClearLoginToken()`: uses UUID
- JWT token generation: `GenerateToken(mentor.ID, mentor.UUID, ...)` instead of `(mentor.ID, mentor.AirtableID, ...)`
- Session: `AirtableID` field -> `MentorUUID`
- Write test

#### Task 2.5: Update MentorRequestsService for PostgreSQL
**Repo**: `getmentor-api`
**Files**: `internal/services/mentor_requests_service.go`
**Details**:
- `GetRequests()`: `mentorAirtableID` parameter -> `mentorUUID`
- `GetRequestByID()`: ownership check uses UUID
- `UpdateStatus()`, `DeclineRequest()`: use UUID-based IDs
- Trigger calls use UUID
- Write test

#### Task 2.6: Remove Airtable webhook handler
**Repo**: `getmentor-api`
**Files**: `internal/handlers/webhook_handler.go`, `internal/services/webhook_service.go`, `cmd/api/main.go`
**Details**:
- Remove or repurpose `HandleAirtableWebhook` handler. Since there's no Airtable anymore, the webhook endpoint `POST /api/webhooks/airtable` is no longer needed.
- Remove the webhook route registration from `cmd/api/main.go` line 59
- Remove `WebhookService.HandleAirtableWebhook()` and `getSlugFromRecordID()`
- Keep `WebhookService` if it handles ISR revalidation, or merge that into another service
- Remove `WebhookServiceInterface` from `interfaces.go`
- Remove `WebhookPayload` model (already done in Task 1.4)
- Optionally: add a new `POST /api/webhooks/cache-refresh` endpoint for manual cache invalidation
- Write test

#### Task 2.7: Update service interfaces
**Repo**: `getmentor-api`
**Files**: `internal/services/interfaces.go`
**Details**:
- `MentorServiceInterface`: `GetMentorByRecordID` -> `GetMentorByUUID`
- `ProfileServiceInterface`: `SaveProfileByAirtableID` -> `SaveProfileByUUID`, `UploadPictureByAirtableID` -> `UploadPictureByUUID`
- `WebhookServiceInterface`: remove or update
- `MentorRequestsServiceInterface`: parameters change from `mentorAirtableID` to `mentorUUID`
- Update interface satisfaction checks at bottom of file

#### Task 2.8: Update handlers for UUID-based IDs
**Repo**: `getmentor-api`
**Files**: `internal/handlers/mentor_profile_handler.go`, `internal/handlers/mentor_requests_handler.go`, `internal/middleware/mentor_session.go`
**Details**:
- `mentor_session.go`: `MentorSession.AirtableID` -> `MentorSession.MentorUUID`. Update session extraction from JWT claims.
- `mentor_profile_handler.go`: Use `session.MentorUUID` instead of `session.AirtableID` for profile operations
- `mentor_requests_handler.go`: Use `session.MentorUUID` for request filtering
- Contact handler: parse `mentorId` (UUID) instead of `mentorAirtableId` from request body
- Write test

#### Task 2.9: Update main.go bootstrap
**Repo**: `getmentor-api`
**Files**: `cmd/api/main.go`
**Details**:
- Remove Airtable client initialization (lines 154-162)
- Add: create pgxpool from `cfg.Database.URL`
- Add: run migrations via `db.RunMigrations()`
- Update cache initialization: pass fetcher functions instead of Airtable client
- Update repository initialization: pass pool instead of Airtable client
- Remove webhook route or replace with cache-refresh endpoint
- Defer `pool.Close()` for graceful shutdown
- Update health check to include DB ping

#### Task 2.10: Update metrics
**Repo**: `getmentor-api`
**Files**: `pkg/metrics/metrics.go`
**Details**:
- Rename `AirtableRequestDuration` -> `DBRequestDuration` (keep metric name `db_client_operation_duration_seconds` - it's already generic)
- Rename `AirtableRequestTotal` -> `DBRequestTotal` (metric name `db_client_operation_total` already generic)
- Update all references in repository code
- Update the comment "Database Client Metrics (Airtable)" -> "Database Client Metrics (PostgreSQL)"

#### Task 2.11: Delete Airtable package and update dependencies
**Repo**: `getmentor-api`
**Files**: `pkg/airtable/client.go` (delete), `test/pkg/airtable/client_test.go` (delete), `go.mod`, `go.sum`
**Details**:
- Delete `pkg/airtable/` directory entirely
- Remove `github.com/mehanizm/airtable` from `go.mod`
- Run `go mod tidy`
- Verify build: `go build ./...`

#### Task 2.12: Update .env.example and documentation
**Repo**: `getmentor-api`
**Files**: `.env.example`, `README.md`
**Details**:
- Replace `AIRTABLE_API_KEY`, `AIRTABLE_BASE_ID`, `AIRTABLE_WORK_OFFLINE` with `DATABASE_URL`
- Add `DATABASE_URL=postgres://getmentor:password@localhost:5432/getmentor?sslmode=disable`
- Update README sections about Airtable to mention PostgreSQL
- Update any references to "Airtable webhooks" in docs

### Phase 3: Azure Functions (getmentor-func)

#### Task 3.1: Add PostgreSQL client to getmentor-func
**Repo**: `getmentor-func`
**Files**: `package.json`, `lib/utils/db.ts` (new), `lib/utils/airtable.ts` (delete)
**Details**:
- `npm install pg @types/pg`
- `npm uninstall airtable`
- Create `lib/utils/db.ts`:
  ```typescript
  import { Pool } from 'pg';
  const pool = new Pool({ connectionString: process.env.DATABASE_URL });
  export { pool };
  ```
- Delete `lib/utils/airtable.ts`
- Update `local.settings.json` template: add `DATABASE_URL`, remove `AIRTABLE_API_KEY`, `AIRTABLE_BASE_ID`

#### Task 3.2: Migrate new-request-watcher to PostgreSQL
**Repo**: `getmentor-func`
**Files**: `new-request-watcher/index.ts`
**Details**:
- Replace `AirtableBase('Client Requests').find(requestId)` with `pool.query('SELECT * FROM client_requests WHERE id = $1', [requestId])`
- Replace `AirtableBase('Mentors').find(request.mentorId)` with `pool.query('SELECT * FROM mentors WHERE id = $1', [request.mentorId])`
- Replace `AirtableBase('Client Requests').update(...)` with `pool.query('UPDATE client_requests SET telegram = $1, status = $2 WHERE id = $3', [...])`
- Update `Request` and `Mentor` constructors to accept PG row objects instead of Airtable records
- Write test

#### Task 3.3: Migrate new-mentor-watcher to PostgreSQL
**Repo**: `getmentor-func`
**Files**: `new-mentor-watcher/index.ts`
**Details**:
- Replace `AirtableBase('Mentors').find(mentorId)` with PG query
- Replace `AirtableBase('Mentors').update(mentor.id, {...})` with `UPDATE mentors SET ... WHERE id = $1`
- Replace `findDuplicates()` Airtable query with `SELECT COUNT(*) FROM mentors WHERE email = $1 AND status IN ('active', 'inactive')`
- The function still generates `tgSecret`, `authToken`, `alias`, `sortOrder` and writes them to DB
- Write test

#### Task 3.4: Migrate remaining Azure Functions to PostgreSQL
**Repo**: `getmentor-func`
**Files**: `request-process-finished/index.ts`, `sessions-watcher/index.ts`, `adm-bot-listener/index.ts`, `process-mentee-review/index.ts`, `mentor-login-email/index.ts`, `update-mentor-image/index.ts`, `randomize-sort-order/index.ts`, `update-status-reminder/index.ts`, `tg-mass-send/index.ts`
**Details**:
- For each function:
    - Replace `AirtableBase('TableName')` calls with `pool.query()` SQL
    - Table name mappings: `'Mentors'` -> `mentors`, `'Client Requests'` -> `client_requests`, `'Tags'` -> `tags`
    - Field name mappings: Airtable CamelCase -> PostgreSQL snake_case (e.g., `JobTitle` -> `job_title`, `Calendly Url` -> `calendar_url`, `Created Time` -> `created_at`)
    - Update model constructors
- This is a bulk task but each function is independent - can be split into sub-tasks if needed
- Write test for each function

#### Task 3.5: Update data models in getmentor-func
**Repo**: `getmentor-func`
**Files**: `lib/data/mentor.ts` (and related model files)
**Details**:
- Update `Mentor` class constructor to accept a plain JS object (PG row) instead of an Airtable `Record`
- Map PG column names (snake_case) to class properties
- Update `Request` class similarly
- Update Telegram notification messages that reference `mentor.id` (now UUID instead of Airtable record ID)
- Write test

### Phase 4: Telegram Bot (getmentor-bot)

#### Task 4.1: Create PostgresStorage implementation
**Repo**: `getmentor-bot`
**Files**: `package.json`, `lib/storage/postgres/PostgresStorage.ts` (new), `lib/storage/airtable/AirtableBase.ts` (delete)
**Details**:
- `npm install pg @types/pg`
- `npm uninstall airtable`
- Create `lib/storage/postgres/PostgresStorage.ts` implementing `MentorStorage` interface:
    - `getMentorByTelegramId(chatId)`: `SELECT * FROM mentors WHERE telegram_chat_id = $1`
    - `getMentorBySecretCode(code)`: `SELECT * FROM mentors WHERE tg_secret = $1`
    - `setMentorStatus(mentor, newStatus)`: `UPDATE mentors SET status = $1 WHERE id = $2`
    - `getMentorActiveRequests(mentor)`: `SELECT * FROM client_requests WHERE mentor_id = $1 AND status NOT IN ('done', 'declined', 'unavailable') ORDER BY created_at ASC`
    - `getMentorArchivedRequests(mentor)`: `SELECT * FROM client_requests WHERE mentor_id = $1 AND status NOT IN ('pending', 'working', 'contacted') ORDER BY updated_at DESC`
    - `setMentorTelegramChatId(mentorId, chatId)`: `UPDATE mentors SET telegram_chat_id = $1 WHERE id = $2`
    - `setRequestStatus(request, newStatus)`: `UPDATE client_requests SET status = $1, status_changed_at = NOW() WHERE id = $2`
- Keep same NodeCache caching layer for bot (optional - can simplify since PG is fast)
- Write test

#### Task 4.2: Update bot entry point and models
**Repo**: `getmentor-bot`
**Files**: `getmentor-bot/index.ts`, `lib/models/Mentor.ts`, `lib/models/MentorClientRequest.ts`
**Details**:
- Update `index.ts`: instantiate `PostgresStorage` instead of `AirtableBase`
- Update `Mentor` model: constructor accepts PG row (snake_case fields) instead of Airtable record
- Update `MentorClientRequest` model: same change
- Update `local.settings.json` template: add `DATABASE_URL`, remove `AIRTABLE_*`
- Write test

### Phase 5: Frontend (getmentor.dev)

#### Task 5.1: Rename airtableId to mentorId/requestId in frontend types
**Repo**: `getmentor.dev`
**Files**: `src/types/mentor.ts`, `src/types/mentor-requests.ts`, `src/lib/go-api-client.ts`
**Details**:
- In `MentorBase` interface: rename `airtableId: string` to `mentorId: string`
- In `MentorWithSecureFields`: same rename
- In `go-api-client.ts`: rename `getOneMentorByRecordId()` to `getOneMentorById()`. The API path changes from `?rec=` to `?mentorId=`.
- In `MentorClientRequest` type (if it has airtable references): rename to use `mentorId`
- In `MentorSession` type: `airtable_id` -> `mentorId`
- Search all `.ts`/`.tsx` files for `airtableId` and replace with `mentorId`
- Search for `mentorAirtableId` and replace with `mentorId`
- Update all test mocks that use `rec*` IDs to use UUID format
- **Coordinated deploy**: This is a breaking change. Must deploy Go API and frontend together.
- Write test: update test mocks

#### Task 5.2: Update contact form to send mentorId
**Repo**: `getmentor.dev`
**Files**: `src/pages/mentor/[slug]/contact.tsx`, `src/pages/bementor.tsx`
**Details**:
- The contact form currently sends `mentorAirtableId` in the request body
- Rename to `mentorId` (UUID)
- Update the contact form component to use `mentor.mentorId` instead of `mentor.airtableId`
- Go API side (Task 1.4) renames the field and removes `startswith=rec` validation, uses UUID validation instead
- Write test

### Phase 6: Testing & Verification

#### Task 6.1: Write Go API integration tests
**Repo**: `getmentor-api`
**Files**: `test/integration/` (new directory)
**Details**:
- Add `github.com/testcontainers/testcontainers-go` for PostgreSQL test containers
- Create `test/integration/setup_test.go` with test database setup/teardown
- Write integration tests:
    - `TestMentorRepository_CRUD` - create, read, update, delete mentors
    - `TestClientRequestRepository_CRUD` - create, read, update status, decline
    - `TestMentorCache_PopulateFromDB` - verify cache populates from PG
    - `TestTagsCache_PopulateFromDB` - verify tags cache from PG
    - `TestContactService_SubmitForm` - end-to-end contact submission
    - `TestRegistrationService_Register` - end-to-end registration
    - `TestMentorAuthService_LoginFlow` - request login, verify, session
- Run: `go test -v -tags integration ./test/integration/...`

#### Task 6.2: Update existing Go API tests
**Repo**: `getmentor-api`
**Files**: `test/internal/models/mentor_test.go`, `test/internal/handlers/contact_handler_test.go`, `test/config/config_test.go`
**Details**:
- `mentor_test.go`: Remove tests for `AirtableRecordToMentor()`. Add tests for `ScanMentor()`.
- `contact_handler_test.go`: Update mock to use UUID-based mentor IDs
- `config_test.go`: Update to test `DatabaseConfig` instead of `AirtableConfig`
- Run: `go test ./...`

#### Task 6.3: Manual end-to-end verification checklist
**Details**:
- [ ] Docker compose up with PostgreSQL
- [ ] Migrations run automatically on API startup
- [ ] `GET /api/healthcheck` returns 200
- [ ] `GET /api/v1/mentors` returns mentor list
- [ ] `POST /api/v1/contact-mentor` creates client request in PG
- [ ] `POST /api/v1/register-mentor` creates mentor in PG
- [ ] Mentor login flow works (email -> token -> session)
- [ ] Mentor admin: view requests, update status, decline
- [ ] Profile update works
- [ ] Frontend loads mentor list and individual mentor pages
- [ ] Azure Functions: new-request-watcher processes request from PG
- [ ] Azure Functions: new-mentor-watcher processes new mentor from PG
- [ ] Telegram bot: /start with secret code authenticates from PG
- [ ] Cache invalidation works (manual refresh endpoint)
- [ ] Metrics endpoint shows `db_client_operation_*` metrics

## Dependency Order

```
0.1 (infra) ─┐
0.2 (migrate)─┼─> 1.1 (pool) ──> 1.2 (mentor model) ──> 1.5 (mentor repo) ──┐
              │                   1.3 (request model) ──> 1.6 (request repo) ──┤
              │                   1.4 (other models)                           │
              │                                                                ├──> 2.x (services) ──> 2.9 (main.go) ──> 2.11 (cleanup)
              │                                                                │
              │   1.7 (cache) ─────────────────────────────────────────────────┘
              │
              ├──> 3.1 (func pg client) ──> 3.2-3.5 (func migrations)
              ├──> 4.1-4.2 (bot migration)
              └──> 5.1-5.2 (frontend, if needed)

6.x (testing) runs after each phase
```

## Risks & Mitigations

1. **Data loss during cutover**: Mitigate by running data migration script right before cutover, verifying row counts match Airtable.
2. **ID format change breaks external integrations**: Keep `airtable_id` column populated. Any external system referencing `rec*` IDs can still look up via that column.
3. **Connection pool exhaustion**: PG pool configured with `MaxConns=10`, which is shared between cache refresh and request handling. Monitor `db_client_operation_*` metrics. Increase if needed.
4. **Azure Functions cold start + PG connection**: PG connections from Azure Functions may be slow on cold start. Use connection pooling or PgBouncer if needed.
5. **Frontend `airtableId` -> `mentorId` rename**: Requires coordinated deploy of Go API + frontend. Deploy both services simultaneously. Consider a brief maintenance window.
