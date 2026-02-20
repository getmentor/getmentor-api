# Post-Migration Cleanup Plan: Airtable → PostgreSQL

This document contains detailed tasks to complete the Airtable-to-PostgreSQL migration cleanup across the entire getmentor monorepo. Each task is self-contained and can be executed without prior context.

---

## Phase 1: Fix Runtime-Breaking Bugs

These tasks fix issues that will cause SQL errors, crashes, or broken functionality at runtime.

---

### Task 1.1: Fix Missing/Wrong Column Names in getmentor-func PgRowAdapter

**Problem:**
File `getmentor-func/lib/utils/db.ts` contains a `PgRowAdapter` class (lines 12-82) that maps Airtable field names to PostgreSQL column names. Several mappings reference columns that do not exist in the PostgreSQL schema (`getmentor-api/migrations/000001_initial_schema.up.sql`):

| Airtable Field Name | Mapped PG Column | Actual PG Column | Status |
|---|---|---|---|
| `Pending Sessions Count` | `pending_sessions_count` | Does not exist | **MISSING** |
| `Created Days Ago` | `created_days_ago` | Does not exist | **MISSING** |
| `Calendly Url` | `calendly_url` | `calendar_url` | **WRONG NAME** |
| `Profile Url` | `profile_url` | Does not exist | **MISSING** |
| `Mentor Name` | `mentor_name` | Does not exist in `client_requests` | **MISSING** |

The `mentors` table schema (lines 8-36 of the schema file) has these columns: `id`, `airtable_id`, `legacy_id`, `slug`, `name`, `job_title`, `workplace`, `about`, `details`, `competencies`, `experience`, `price`, `status`, `email`, `telegram`, `telegram_chat_id`, `tg_secret`, `calendar_url`, `privacy`, `avito`, `ex_avito`, `sort_order`, `login_token`, `login_token_expires_at`, `created_at`, `updated_at`.

The `client_requests` table (lines 64-86) has: `id`, `airtable_id`, `mentor_id`, `email`, `name`, `telegram`, `description`, `level`, `status`, `created_at`, `updated_at`, `status_changed_at`, `scheduled_at`, `decline_reason`, `decline_comment`.

**Proposed Solution:**
1. Fix `Calendly Url` mapping: change `'calendly_url'` → `'calendar_url'` (line 27)
2. Remove `Pending Sessions Count` mapping (line 26) — this must be computed via a JOIN or subquery
3. Remove `Created Days Ago` mapping (line 45) — compute using `EXTRACT(DAY FROM NOW() - created_at)` in SQL
4. Remove `Profile Url` mapping (line 36) — this field no longer exists; the photo URL is constructed from the slug
5. Remove `Mentor Name` mapping (line 47) — must be fetched via a JOIN to `mentors` table
6. Update every SQL query in getmentor-func that uses these non-existent columns

**Files to Change:**
- `getmentor-func/lib/utils/db.ts` — Fix field mappings
- `getmentor-func/sessions-watcher/index.ts` — Fix SQL queries that reference `pending_sessions_count`, `created_days_ago`, `mentor_name`
- `getmentor-func/tg-mass-send/index.ts` — Fix query referencing `is_visible` (column does not exist)
- `getmentor-func/randomize-sort-order/index.ts` — Fix query referencing `is_visible`
- `getmentor-func/update-status-reminder/index.ts` — Fix query referencing `last_status_change` (should be `status_changed_at`)

**Definition of Done:**
- All PgRowAdapter mappings point to columns that actually exist in the schema
- All SQL queries in getmentor-func use correct column names from the schema
- `cd getmentor-func && yarn build` compiles without errors
- Computed fields (`pending_sessions_count`, `created_days_ago`, `mentor_name`) are calculated in SQL queries using JOINs or expressions

**Edge Cases:**
- `pending_sessions_count` = 0 when a mentor has no pending requests — the subquery must handle NULL/empty results
- `created_days_ago` should handle requests created today (result = 0)
- `mentor_name` JOIN must handle orphaned requests where `mentor_id` is NULL (use LEFT JOIN)
- `is_visible` does not exist in the schema — determine the business logic (likely `status = 'active'`) and replace accordingly

**Tests:**
- Manually verify each SQL query against a local PostgreSQL instance with the schema applied
- Run `yarn build` to verify TypeScript compilation
- Create a test script that runs each function's main query against the dev database and verifies results are returned without errors

---

### Task 1.2: Fix Missing/Wrong Column Names in getmentor-bot PostgresStorage

**Problem:**
File `getmentor-bot/lib/storage/postgres/PostgresStorage.ts` contains a `PgRowAdapter` class (lines 12-73) with field mappings that reference non-existent PostgreSQL columns:

| Airtable Field Name | Mapped PG Column | Actual PG Column | Status |
|---|---|---|---|
| `Profile Url` | `profile_url` | Does not exist | **MISSING** |
| `Image` | `image` | Does not exist | **MISSING** |
| `Tags Links` | `tags_links` | Does not exist (separate `mentor_tags` table) | **MISSING** |
| `Tags` | `tags` | Does not exist (separate `mentor_tags` + `tags` tables) | **MISSING** |
| `Calendly Url` | `calendly_url` | `calendar_url` | **WRONG NAME** |
| `AuthToken` | `auth_token` | `login_token` | **WRONG NAME** |
| `Review` | `review` | Does not exist (separate `reviews` table) | **MISSING** |
| `Review2` | `review2` | Does not exist | **MISSING** |
| `ReviewFormUrl` | `review_form_url` | Does not exist | **MISSING** |
| `Last Status Change` | `last_status_change` | `status_changed_at` | **WRONG NAME** |

Additionally, the UPDATE query on line 223 uses the wrong column name:
```sql
UPDATE client_requests SET status = $1, last_status_change = NOW() WHERE id = $2 RETURNING *
```
Should be `status_changed_at` instead of `last_status_change`.

**Proposed Solution:**
1. Fix `Calendly Url` mapping: `'calendly_url'` → `'calendar_url'`
2. Fix `AuthToken` mapping: `'auth_token'` → `'login_token'`
3. Fix `Last Status Change` mapping: `'last_status_change'` → `'status_changed_at'`
4. Remove `Profile Url`, `Image`, `Tags Links`, `Tags`, `Review`, `Review2`, `ReviewFormUrl` mappings — these columns don't exist
5. For `Tags`: update the `getMentorByTelegramId` and `getMentorBySecretCode` queries to JOIN with `mentor_tags` and `tags` tables, or add a separate query to fetch tags
6. For `Review`/`Review2`: update queries to JOIN with the `reviews` table when needed, or accept that these fields will be `undefined` in the bot (check if the bot actually uses them)
7. Fix the UPDATE query on line 223: change `last_status_change` to `status_changed_at`

**Files to Change:**
- `getmentor-bot/lib/storage/postgres/PostgresStorage.ts` — Fix field mappings (lines 17-50) and UPDATE query (line 223)
- `getmentor-bot/lib/models/Mentor.ts` — Verify which fields are actually used and handle missing ones gracefully
- `getmentor-bot/lib/models/MentorClientRequest.ts` — Verify which fields are actually used

**Definition of Done:**
- All PgRowAdapter mappings point to columns that actually exist in the schema
- The UPDATE query on line 223 uses `status_changed_at` instead of `last_status_change`
- `cd getmentor-bot && npm run build` compiles without errors
- Tags are either fetched via JOIN or gracefully handled as empty arrays
- Review fields are either fetched via JOIN to `reviews` table or gracefully handled as undefined

**Edge Cases:**
- A mentor with no tags should get an empty array, not `undefined`
- A request with no review should have `review` = `undefined` without crashing
- The `Image` field is used in `Mentor.ts` constructor — if removed, ensure the code path that accesses `mentor.photo` doesn't crash (it should return `{}` or `null`)
- Cache invalidation must still work correctly after column name fixes

**Tests:**
- Run `npm run build` to verify compilation
- Test `getMentorByTelegramId` with a real database row and verify all Mentor fields are populated
- Test `setRequestStatus` to verify the UPDATE query executes without SQL errors
- Verify cache swap logic still works when moving requests between active/archived

---

### Task 1.3: Fix Wrong Column Names in new-mentor-watcher

**Problem:**
File `getmentor-func/new-mentor-watcher/index.ts` contains an UPDATE query (around lines 62-65) that uses three non-existent column names:

```sql
UPDATE mentors SET
    name = $1,
    telegram = $2,
    tg_secret = $3,
    comms_type = $4,
    auth_token = $5,
    auth_token_expiry = $6,
    slug = $7,
    status = $8,
    sort_order = $9
WHERE id = $10
```

Issues:
- `comms_type` does not exist in the `mentors` table schema
- `auth_token` should be `login_token`
- `auth_token_expiry` should be `login_token_expires_at`

**Proposed Solution:**
1. Remove `comms_type` from the UPDATE — determine if this field is needed and either add a migration to create the column or remove the field entirely
2. Rename `auth_token` → `login_token`
3. Rename `auth_token_expiry` → `login_token_expires_at`
4. Adjust the parameter numbering ($1, $2, ...) accordingly if `comms_type` is removed

**Files to Change:**
- `getmentor-func/new-mentor-watcher/index.ts` — Fix the UPDATE query

**Definition of Done:**
- The UPDATE query uses only columns that exist in the `mentors` table
- `cd getmentor-func && yarn build` compiles without errors
- The function can successfully update a mentor record in PostgreSQL

**Edge Cases:**
- If `comms_type` was carrying important data (e.g., communication preference), a new migration may be needed to add a column. Check the Airtable schema to understand what `comms_type` represented.
- `login_token_expires_at` expects a TIMESTAMPTZ value — ensure the value being set is a proper timestamp

**Tests:**
- Run `yarn build`
- Execute the function against a test database with a sample mentor record and verify the UPDATE succeeds
- Verify the updated record has correct `login_token` and `login_token_expires_at` values

---

### Task 1.4: Fix sessions-watcher SQL Queries

**Problem:**
File `getmentor-func/sessions-watcher/index.ts` contains two SQL queries with non-existent columns:

Query 1 (around line 16):
```sql
SELECT id, name, email, telegram_chat_id, pending_sessions_count
FROM mentors
WHERE pending_sessions_count > 0
```
`pending_sessions_count` does not exist as a column.

Query 2 (around line 24):
```sql
SELECT id, mentor_id, name, telegram, email, description, level,
       created_at, created_days_ago, status, mentor_name,
       decline_reason, decline_comment
FROM client_requests
```
`created_days_ago` and `mentor_name` do not exist as columns in `client_requests`.

**Proposed Solution:**
1. Replace Query 1 with a JOIN/subquery that counts pending sessions:
```sql
SELECT m.id, m.name, m.email, m.telegram_chat_id,
       COUNT(cr.id) AS pending_sessions_count
FROM mentors m
JOIN client_requests cr ON cr.mentor_id = m.id
WHERE cr.status IN ('pending', 'contacted', 'working')
  AND m.status = 'active'
GROUP BY m.id
HAVING COUNT(cr.id) > 0
```

2. Replace Query 2 with computed columns:
```sql
SELECT cr.id, cr.mentor_id, cr.name, cr.telegram, cr.email,
       cr.description, cr.level, cr.created_at,
       EXTRACT(DAY FROM NOW() - cr.created_at)::int AS created_days_ago,
       cr.status, m.name AS mentor_name,
       cr.decline_reason, cr.decline_comment
FROM client_requests cr
LEFT JOIN mentors m ON m.id = cr.mentor_id
```

**Files to Change:**
- `getmentor-func/sessions-watcher/index.ts` — Rewrite both queries

**Definition of Done:**
- Both queries use only columns from the actual schema plus computed expressions
- `yarn build` compiles without errors
- The function correctly identifies mentors with pending sessions and sends reminders

**Edge Cases:**
- Mentors with no `telegram_chat_id` should be excluded from Telegram notifications
- `created_days_ago` for a request created today should be 0
- Orphaned requests (where `mentor_id` references a deleted mentor) should not crash the JOIN — use LEFT JOIN
- Handle timezone differences in `created_days_ago` calculation

**Tests:**
- Insert test data: a mentor with 2 pending requests, one with 0
- Run the function and verify only the mentor with pending requests is selected
- Verify `created_days_ago` is calculated correctly for requests of various ages

---

### Task 1.5: Fix `is_visible` References in tg-mass-send and randomize-sort-order

**Problem:**
Two Azure Functions reference a column `is_visible` that does not exist in the `mentors` table:

1. `getmentor-func/tg-mass-send/index.ts` (line 28): `AND is_visible = true`
2. `getmentor-func/randomize-sort-order/index.ts` (line 20): `AND is_visible = true`

The `mentors` table has a `status` column with values: `pending`, `active`, `inactive`, `declined`. There is no `is_visible` boolean column.

**Proposed Solution:**
Replace `is_visible = true` with `status = 'active'` in both files, as active mentors are the ones that should be visible on the platform.

**Files to Change:**
- `getmentor-func/tg-mass-send/index.ts` — Replace `AND is_visible = true` with `AND status = 'active'`
- `getmentor-func/randomize-sort-order/index.ts` — Replace `AND is_visible = true` with `AND status = 'active'`

**Definition of Done:**
- Neither file references `is_visible`
- Both use `status = 'active'` to filter visible mentors
- `yarn build` compiles without errors

**Edge Cases:**
- Ensure `inactive` mentors are excluded (they opted out)
- Ensure `pending` mentors are excluded (not yet approved)
- Ensure `declined` mentors are excluded

**Tests:**
- Run `yarn build`
- Test query against database with mentors in all 4 statuses; verify only `active` mentors are returned

---

### Task 1.6: Fix HIGHLIGHTED_MENTORS Config with Airtable Record IDs

**Problem:**
File `getmentor-func/local.settings.json` (line 31) contains:
```json
"HIGHLIGHTED_MENTORS": "recybFvnrWxphlbz6,recrYwJ92K9OX11W0,rec9IBOl74sE8hOnx",
```

These are Airtable record IDs (prefixed with `rec`). After PostgreSQL migration, mentor IDs are UUIDs. The `randomize-sort-order` function uses these IDs to give highlighted mentors priority sort order, but the IDs won't match any PostgreSQL records.

**Proposed Solution:**
1. Look up the actual PostgreSQL UUIDs for these three mentors by querying `SELECT id, airtable_id FROM mentors WHERE airtable_id IN ('recybFvnrWxphlbz6', 'recrYwJ92K9OX11W0', 'rec9IBOl74sE8hOnx')`
2. Replace the Airtable record IDs with the corresponding UUIDs
3. Update the `randomize-sort-order` function to use UUID format for ID comparison

**Files to Change:**
- `getmentor-func/local.settings.json` — Update `HIGHLIGHTED_MENTORS` with UUIDs
- `getmentor-func/randomize-sort-order/index.ts` — Verify the function parses UUIDs correctly

**Definition of Done:**
- `HIGHLIGHTED_MENTORS` contains valid PostgreSQL UUIDs
- The randomize-sort-order function correctly identifies and prioritizes highlighted mentors

**Edge Cases:**
- If any of the three Airtable records were not migrated, the UUID lookup will return no results — remove those from the list
- The function should handle the case where `HIGHLIGHTED_MENTORS` is empty or undefined

**Tests:**
- Query the database to verify the UUIDs exist
- Run the randomize-sort-order function and verify highlighted mentors get priority sort order

---

### Task 1.7: Replace Hardcoded Airtable URLs in Telegram Messages

**Problem:**
Two Telegram notification message classes contain hardcoded Airtable URLs:

1. `getmentor-func/lib/telegram/messages/NewRequestModeratorNotificationMessage.ts` (line 28):
```typescript
<a href="https://airtable.com/tblCA5xeV12ufn0iQ/viw69cXOerjigWEVs/${this._request.id}">View on Airtable</a>
```

2. `getmentor-func/lib/telegram/messages/NewMentorModeratorNotificationMessage.ts` (line 57):
```typescript
<a href="https://airtable.com/tblt7APgEGkR5VwTR/viwPV41YKZ1SCAtyB/${this._mentor.id}">More details on Airtable</a>
```

These links are sent to moderators via Telegram. Since IDs are now UUIDs and Airtable is no longer the database, these links will be broken.

**Proposed Solution:**
Option A (Recommended): Remove the Airtable links entirely if there is no admin UI to replace them.
Option B: Replace with links to an admin dashboard or a direct PostgreSQL query reference like `ID: ${this._request.id}`.

For now, replace with a simple text showing the record ID so moderators can look it up:
```typescript
// In NewRequestModeratorNotificationMessage.ts
`Request ID: <code>${this._request.id}</code>`

// In NewMentorModeratorNotificationMessage.ts
`Mentor ID: <code>${this._mentor.id}</code>`
```

**Files to Change:**
- `getmentor-func/lib/telegram/messages/NewRequestModeratorNotificationMessage.ts` (line 28)
- `getmentor-func/lib/telegram/messages/NewMentorModeratorNotificationMessage.ts` (line 57)

**Definition of Done:**
- No Airtable URLs remain in Telegram notification messages
- Moderators can still identify the record (by UUID)
- `yarn build` compiles without errors

**Edge Cases:**
- If the IDs are very long UUIDs, they should still fit in a Telegram message (UUID is 36 chars, well within limits)
- HTML entities must be properly escaped in Telegram HTML mode

**Tests:**
- Run `yarn build`
- Manually trigger a test notification and verify the message renders correctly in Telegram

---

### Task 1.8: Replace Hardcoded Airtable Review Form URL in Email Template

**Problem:**
File `getmentor-func/lib/postbox/templates/session-complete.ts` contains a hardcoded Airtable shared form URL in the email HTML:
```html
<a href="https://airtable.com/shrFNIXY2dRqqGjAi?prefill_RequestRecordId={{request_id}}&hide_RequestRecordId=true">Оставить отзыв</a>
```

This Airtable form was used to collect mentee reviews. After migration, the `request_id` is a UUID, and the Airtable form won't accept it. The form itself is an external Airtable resource that no longer connects to the database.

**Proposed Solution:**
Create a new review form on the getmentor.dev frontend (e.g., `/review/[request_id]`) and update the email template to link to it. If a frontend form doesn't exist yet, link to a placeholder page or remove the review link temporarily.

Interim solution:
```html
<a href="https://getmentor.dev/review/{{request_id}}">Оставить отзыв</a>
```

**Files to Change:**
- `getmentor-func/lib/postbox/templates/session-complete.ts` — Replace the Airtable form URL
- (Optionally) `getmentor.dev/src/pages/review/[id].tsx` — Create the review form page

**Definition of Done:**
- No Airtable URLs remain in email templates
- The review link either works with the new frontend page or is temporarily removed with a comment explaining why
- `yarn build` compiles without errors

**Edge Cases:**
- The `{{request_id}}` placeholder in the template must be replaced at runtime with the actual UUID
- If the review page doesn't exist yet, the link should return a friendly 404 or "coming soon" page, not a crash

**Tests:**
- Run `yarn build`
- Render the email template with a sample UUID and verify the link is correct
- If the review page exists, test the full flow: click link → see review form → submit

---

### Task 1.9: Fix Mentor Photo Field in getmentor-func

**Problem:**
File `getmentor-func/lib/data/mentor.ts` (line 53):
```typescript
this.photo = record.fields['Image_Attachment'] ? record.fields['Image_Attachment'][0] : {};
```

This references `Image_Attachment`, an Airtable-specific field that stores file attachments as arrays of objects. In PostgreSQL, there is no `image` or `Image_Attachment` column in the `mentors` table. Photos are stored in Azure Blob Storage / Yandex Object Storage and accessed via a URL constructed from the mentor's slug.

**Proposed Solution:**
Replace the `Image_Attachment` reference with a photo URL constructed from the mentor's slug:
```typescript
this.photo = record.get('Alias')
  ? { url: `https://${process.env.AZURE_STORAGE_DOMAIN}/${record.get('Alias')}/large` }
  : {};
```

Or, if photo URL is not needed in the functions, set it to an empty object and add a comment.

**Files to Change:**
- `getmentor-func/lib/data/mentor.ts` (line 53) — Fix photo field initialization

**Definition of Done:**
- No reference to `Image_Attachment` in the codebase
- Photo field is either constructed from slug + storage domain or set to empty object
- `yarn build` compiles without errors

**Edge Cases:**
- Mentors without a slug should get an empty photo object
- The storage domain environment variable must be available at runtime

**Tests:**
- Run `yarn build`
- Create a Mentor object from a PgRowAdapter and verify `photo` is not undefined/null

---

## Phase 2: Remove Airtable from Go API Core

These tasks remove the Airtable SDK dependency and legacy code from the Go API.

---

### Task 2.1: Remove Airtable Dependency from go.mod

**Problem:**
File `getmentor-api/go.mod` (line 13) still lists:
```
github.com/mehanizm/airtable v0.3.4
```

This dependency is no longer used since all data access goes through PostgreSQL (pgx). It adds unnecessary binary size and could cause confusion.

**Proposed Solution:**
1. Remove `github.com/mehanizm/airtable` from `go.mod`
2. Run `go mod tidy` to clean up `go.sum`
3. Remove the `pkg/airtable/` package directory if it still exists

**Files to Change:**
- `getmentor-api/go.mod` — Remove airtable dependency
- `getmentor-api/go.sum` — Cleaned automatically by `go mod tidy`
- `getmentor-api/pkg/airtable/` — Delete directory if exists

**Definition of Done:**
- `go mod tidy` succeeds
- `go build ./...` succeeds with no airtable imports
- `grep -r "mehanizm/airtable" getmentor-api/` returns no results (except .bak files to be deleted separately)

**Edge Cases:**
- If any file still imports `github.com/mehanizm/airtable`, the build will fail — those imports must be removed first (see Tasks 2.3, 2.4)

**Tests:**
- `go build ./...`
- `go test ./...`
- `go vet ./...`

---

### Task 2.2: Remove AirtableConfig from config.go

**Problem:**
File `getmentor-api/config/config.go` still contains:

1. `AirtableConfig` struct (around line 16):
```go
type Config struct {
    ...
    Airtable AirtableConfig // Deprecated: will be removed after migration
    ...
}
```

2. Default value (line 131): `v.SetDefault("AIRTABLE_WORK_OFFLINE", false)`

3. Validation (lines 259-266):
```go
if !c.Airtable.WorkOffline {
    if c.Airtable.APIKey == "" {
        return fmt.Errorf("AIRTABLE_API_KEY is required")
    }
    if c.Airtable.BaseID == "" {
        return fmt.Errorf("AIRTABLE_BASE_ID is required")
    }
}
```

This means the application **requires** `AIRTABLE_API_KEY` and `AIRTABLE_BASE_ID` environment variables to start, even though they're never used.

**Proposed Solution:**
1. Remove `Airtable AirtableConfig` field from `Config` struct
2. Remove `AirtableConfig` struct definition
3. Remove the `AIRTABLE_WORK_OFFLINE` default
4. Remove the Airtable validation block from `Validate()`
5. Remove any Airtable-related viper bindings

**Files to Change:**
- `getmentor-api/config/config.go` — Remove all Airtable configuration

**Definition of Done:**
- No `Airtable` field in `Config` struct
- No `AirtableConfig` struct
- Application starts without `AIRTABLE_*` environment variables
- `go build ./...` succeeds
- `go test ./...` succeeds

**Edge Cases:**
- Ensure no other code references `config.Airtable.*` — search with `grep -r "\.Airtable\." getmentor-api/`
- CI/CD pipelines that set `AIRTABLE_*` vars won't break (unused vars are ignored), but should be cleaned up (Task 5.1)

**Tests:**
- `go build ./...`
- `go test ./...`
- Start the application without any `AIRTABLE_*` env vars and verify it boots successfully

---

### Task 2.3: Remove AirtableID from JWT Claims

**Problem:**
File `getmentor-api/pkg/jwt/jwt.go` contains:

```go
type MentorClaims struct {
    MentorID   int    `json:"mentor_id"`
    AirtableID string `json:"airtable_id"`  // LINE 21
    Email      string `json:"email"`
    Name       string `json:"name"`
    ...
}

func (tm *TokenManager) GenerateToken(mentorID int, airtableID, email, name string) (string, error) {
    claims := MentorClaims{
        ...
        AirtableID: airtableID,     // LINE 50
        ...
        Subject:    airtableID,      // LINE 58
    }
}
```

The JWT token still embeds `airtable_id` and uses it as the JWT subject. After migration, the subject should be the mentor's UUID.

**Proposed Solution:**
1. Remove `AirtableID` field from `MentorClaims` struct
2. Change `GenerateToken` signature: remove `airtableID` parameter, add `mentorUUID string` parameter
3. Set JWT `Subject` to `mentorUUID` instead of `airtableID`
4. Update all callers of `GenerateToken` to pass the UUID
5. Update all code that reads `claims.AirtableID` to use `claims.Subject` (the UUID)

**Files to Change:**
- `getmentor-api/pkg/jwt/jwt.go` — Refactor claims and GenerateToken
- All files that call `GenerateToken` — Update parameter
- All files that read `claims.AirtableID` — Use `claims.Subject` instead

**Definition of Done:**
- No `AirtableID` field in `MentorClaims`
- JWT subject is the mentor's UUID
- All callers updated
- `go build ./...` succeeds
- `go test ./...` succeeds

**Edge Cases:**
- **BREAKING CHANGE for existing tokens**: Any JWTs issued before this change will have `airtable_id` in them. If mentors have active sessions with old tokens, they'll break. Mitigation: ensure the login flow issues new tokens on every login.
- The frontend `MentorSession` type (Task 3.1) must be updated simultaneously to match the new JWT structure.

**Tests:**
- `go test ./...`
- Test generating a token and verifying it no longer contains `airtable_id`
- Test that the JWT subject is a valid UUID

---

### Task 2.4: Remove Deprecated Airtable Functions from Models

**Problem:**
Two model files contain deprecated Airtable conversion functions:

1. `getmentor-api/internal/models/mentor.go` (lines 212-287):
```go
// Deprecated: AirtableRecordToMentor is deprecated and will be removed in Task 2.11
func AirtableRecordToMentor(record *airtable.Record) *Mentor { ... }
```
This function imports `github.com/mehanizm/airtable` (line 8).

2. `getmentor-api/internal/models/mentor_client_request.go` (lines 185-220+):
```go
// Deprecated: AirtableRecordToMentorClientRequest is deprecated
func AirtableRecordToMentorClientRequest(record *airtable.Record) *MentorClientRequest { ... }
```
This also imports `github.com/mehanizm/airtable` (line 8).

**Proposed Solution:**
1. Delete `AirtableRecordToMentor` function entirely from `mentor.go`
2. Delete `AirtableRecordToMentorClientRequest` function entirely from `mentor_client_request.go`
3. Remove `"github.com/mehanizm/airtable"` import from both files
4. If the `AirtableID` field is removed from `Mentor` struct (see Task 2.3 note), remove it here too

**Files to Change:**
- `getmentor-api/internal/models/mentor.go` — Delete deprecated function and import
- `getmentor-api/internal/models/mentor_client_request.go` — Delete deprecated function and import

**Definition of Done:**
- No `airtable` import in either model file
- No `AirtableRecordTo*` functions exist
- `go build ./...` succeeds
- `go test ./...` succeeds

**Edge Cases:**
- Verify no other file calls these functions: `grep -r "AirtableRecordTo" getmentor-api/`
- If tests reference these functions, update them (see Task 5.3)

**Tests:**
- `go build ./...`
- `go test ./...`

---

### Task 2.5: Delete Airtable Repository Backup File

**Problem:**
File `getmentor-api/internal/repository/mentor_repository_airtable.go.bak` exists. It contains the full old Airtable-based repository implementation. This file is not compiled (`.bak` extension) but adds confusion and clutter.

**Proposed Solution:**
Delete the file: `rm getmentor-api/internal/repository/mentor_repository_airtable.go.bak`

**Files to Change:**
- `getmentor-api/internal/repository/mentor_repository_airtable.go.bak` — DELETE

**Definition of Done:**
- File no longer exists
- `go build ./...` still succeeds (file wasn't compiled anyway)

**Edge Cases:** None.

**Tests:** `go build ./...`

---

### Task 2.6: Remove AirtableConfig Retry Function

**Problem:**
File `getmentor-api/pkg/retry/retry.go` (lines 45-51) contains:
```go
func AirtableConfig() Config {
    config := DefaultConfig()
    config.MaxRetries = 3
    config.InitialDelay = 500 * time.Millisecond
    config.MaxDelay = 10 * time.Second
    return config
}
```

This function is named for Airtable and may no longer be called.

**Proposed Solution:**
1. Search for callers: `grep -r "AirtableConfig" getmentor-api/`
2. If no callers, delete the function
3. If callers exist, rename to `ExternalAPIConfig()` or `DatabaseConfig()`

**Files to Change:**
- `getmentor-api/pkg/retry/retry.go` — Remove or rename function

**Definition of Done:**
- No function named `AirtableConfig` exists
- `go build ./...` succeeds

**Edge Cases:** None.

**Tests:** `go build ./...`, `go test ./...`

---

### Task 2.7: Remove Deprecated Profile Service Methods

**Problem:**
File `getmentor-api/internal/services/profile_service.go` contains two deprecated methods (around lines 41-52 and 115-126):
```go
// SaveProfile is deprecated - token-based auth has been replaced with login tokens
func (s *ProfileService) SaveProfile(ctx context.Context, id int, token string, req *models.SaveProfileRequest) error { ... }

// UploadProfilePicture is deprecated - token-based auth has been replaced with login tokens
func (s *ProfileService) UploadProfilePicture(ctx context.Context, id int, token string, req *models.UploadProfilePictureRequest) (string, error) { ... }
```

These methods use the old `AuthToken`-based authentication which no longer exists in the schema (`auth_token` was replaced by `login_token`).

**Proposed Solution:**
1. Check if any HTTP handler still routes to these methods: `grep -r "SaveProfile\b" getmentor-api/` and `grep -r "UploadProfilePicture\b" getmentor-api/`
2. If no handlers route to them, delete both methods
3. If handlers exist, remove the route registrations too

**Files to Change:**
- `getmentor-api/internal/services/profile_service.go` — Remove deprecated methods
- Router/handler files if they reference these methods

**Definition of Done:**
- No deprecated `SaveProfile(ctx, id, token, req)` or `UploadProfilePicture(ctx, id, token, req)` methods
- `go build ./...` succeeds
- `go test ./...` succeeds

**Edge Cases:**
- Ensure the non-deprecated versions (`SaveProfileByMentorId`, `UploadPictureByMentorId`) still work correctly

**Tests:** `go build ./...`, `go test ./...`

---

### Task 2.8: Fix Tag Update TODO in Profile Service

**Problem:**
File `getmentor-api/internal/services/profile_service.go` (around line 100):
```go
_ = tagIDs // TODO: Implement tag updates in repository
```

Tag updates are silently ignored. When a mentor updates their profile tags, the changes are lost.

**Proposed Solution:**
1. Implement tag update logic in the mentor repository:
   - DELETE existing entries from `mentor_tags` WHERE `mentor_id = $1`
   - INSERT new entries into `mentor_tags` for each tag
2. Add a `UpdateMentorTags(ctx, mentorID uuid.UUID, tagIDs []uuid.UUID)` method to the repository
3. Call it from the profile service

**Files to Change:**
- `getmentor-api/internal/repository/` — Add `UpdateMentorTags` method
- `getmentor-api/internal/services/profile_service.go` — Call the new method instead of `_ = tagIDs`

**Definition of Done:**
- Tag updates are persisted to the `mentor_tags` table
- No TODO comment about tag updates
- `go build ./...` succeeds
- `go test ./...` succeeds

**Edge Cases:**
- A mentor with no tags should result in all entries being deleted from `mentor_tags`
- A mentor setting the same tags again should not cause errors (DELETE + INSERT is idempotent)
- Invalid tag IDs should return an error, not silently fail
- Use a transaction to ensure atomicity (DELETE all old + INSERT all new)

**Tests:**
- Test updating tags for a mentor: verify `mentor_tags` table has correct entries
- Test clearing all tags: verify `mentor_tags` is empty for that mentor
- Test with invalid tag IDs: verify error is returned

---

## Phase 3: Fix Frontend Types and Code

---

### Task 3.1: Fix MentorSession Type Mismatch

**Problem:**
File `getmentor.dev/src/types/mentor-requests.ts` (lines 126-133):
```typescript
export interface MentorSession {
  mentor_id: number
  airtable_id: string
  email: string
  name: string
  exp: number
  iat: number
}
```

Issues:
- `airtable_id: string` is a legacy field that should not exist
- `mentor_id: number` should be `mentor_id: string` (UUID from Go API)
- Missing `legacy_id: number` if needed for backwards compatibility

The Go API JWT claims (`pkg/jwt/jwt.go`) currently send `mentor_id` as int and `airtable_id` as string. After Task 2.3, it will send `mentor_id` as UUID string.

**Proposed Solution:**
1. Remove `airtable_id` field
2. Change `mentor_id: number` to `mentor_id: string` (to match UUID)
3. Add `legacy_id?: number` if needed for backwards compatibility
4. Update all code that accesses `session.airtable_id` to use `session.mentor_id`

**Files to Change:**
- `getmentor.dev/src/types/mentor-requests.ts` — Update `MentorSession` interface
- All files that reference `session.airtable_id` — Update to use `session.mentor_id`

**Definition of Done:**
- `MentorSession` matches the Go API JWT claims structure
- No references to `airtable_id` in frontend code
- `yarn build` succeeds
- `yarn test` passes

**Edge Cases:**
- If any existing JWT tokens in users' browsers contain `airtable_id`, the frontend must handle the field being absent gracefully
- The `mentor_id` type change from `number` to `string` may break comparisons — search for `session.mentor_id ===` or `== ` checks

**Tests:**
- `yarn build`
- `yarn test`
- Test the login flow and verify session data is correctly parsed

---

### Task 3.2: Fix Metrics Normalization Regex

**Problem:**
File `getmentor.dev/src/lib/with-observability.ts` (lines 20-24):
```typescript
const normalized = path
  .replace(/\/api\/mentor\/requests\/rec[A-Za-z0-9]+/, '/api/mentor/requests/:id')
```

This regex matches Airtable record IDs (prefixed with `rec`). After migration, IDs are UUIDs (e.g., `550e8400-e29b-41d4-a716-446655440000`). The regex won't match UUIDs, causing metrics cardinality explosion — each unique UUID will create a separate metric label.

**Proposed Solution:**
Replace the Airtable-specific regex with a UUID-matching regex:
```typescript
const normalized = path
  .replace(/\/api\/mentor\/requests\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i, '/api/mentor/requests/:id')
```

Or more broadly, match any path segment that looks like an ID:
```typescript
  .replace(/\/api\/mentor\/requests\/[^/]+/, '/api/mentor/requests/:id')
```

**Files to Change:**
- `getmentor.dev/src/lib/with-observability.ts` — Update regex pattern

**Definition of Done:**
- UUID-format IDs in request paths are normalized to `:id`
- No `rec` prefix matching in the regex
- `yarn build` succeeds
- `yarn test` passes

**Edge Cases:**
- Path segments that are not IDs (like `/api/mentor/requests/status`) should NOT be normalized
- If both old `rec` IDs and new UUIDs might appear during transition, match both patterns

**Tests:**
- Unit test: `'/api/mentor/requests/550e8400-e29b-41d4-a716-446655440000'` normalizes to `'/api/mentor/requests/:id'`
- Unit test: `'/api/mentor/requests/status'` is NOT normalized

---

### Task 3.3: Rename Legacy Variable `selectedAirtableExperience`

**Problem:**
File `getmentor.dev/src/components/hooks/useMentors.ts` (line 75):
```typescript
const selectedAirtableExperience = selectedExperience.map(
  (e) => filters.experience[e as keyof typeof filters.experience]
)
```

The variable name `selectedAirtableExperience` implies Airtable-specific logic, but it's just a mapping of experience filter values. This is confusing for maintainers.

**Proposed Solution:**
Rename `selectedAirtableExperience` → `selectedExperienceValues`

**Files to Change:**
- `getmentor.dev/src/components/hooks/useMentors.ts` — Rename variable (lines 75-80)

**Definition of Done:**
- Variable is renamed
- `yarn build` succeeds
- `yarn test` passes

**Edge Cases:** None.

**Tests:** `yarn build`, `yarn test`

---

### Task 3.4: Update Test Mock Data from `rec` Prefix to UUIDs

**Problem:**
Multiple test files use Airtable-style `rec` prefix IDs in mock data:
- `getmentor.dev/src/__tests__/server/mentors-data.test.ts` — `mentorId: 'rec1'`
- `getmentor.dev/src/__tests__/components/MentorsList.test.tsx` — `mentorId: 'rec1'`
- `getmentor.dev/src/components/hooks/__tests__/useMentors.test.tsx` — `mentorId: 'rec1'`
- `getmentor.dev/src/__tests__/pages/api/contact-mentor.test.ts` — `mentorId: 'rec123'`

**Proposed Solution:**
Replace all `rec*` mock IDs with UUID-format strings (e.g., `'550e8400-e29b-41d4-a716-446655440000'`).

**Files to Change:**
- All test files listed above

**Definition of Done:**
- No `rec` prefix IDs in test mock data
- `yarn test` passes

**Edge Cases:** None — these are test fixtures only.

**Tests:** `yarn test`

---

### Task 3.5: Update Stale Comments and Legacy Function Names

**Problem:**
Several files contain misleading comments or function names:
1. `getmentor.dev/src/lib/html-content.ts` (line 14): `// First, handle wysiwyg v1 (airtable) format migration`
2. `getmentor.dev/src/server/mentors-data.ts` (line 48): `getOneMentorByRecordId` with comment `Get a single mentor by Airtable record ID`
3. `getmentor.dev/src/lib/go-api-client.ts` (line 176): Same function with Airtable documentation

**Proposed Solution:**
1. Update comment in `html-content.ts` to: `// Handle wysiwyg v1 format compatibility`
2. Update JSDoc for `getOneMentorByRecordId` to: `Get a single mentor by legacy record ID`
3. Update JSDoc in `go-api-client.ts` similarly

**Files to Change:**
- `getmentor.dev/src/lib/html-content.ts`
- `getmentor.dev/src/server/mentors-data.ts`
- `getmentor.dev/src/lib/go-api-client.ts`

**Definition of Done:**
- No comments reference "Airtable" in frontend code
- `yarn build` succeeds

**Edge Cases:** None.

**Tests:** `yarn build`

---

## Phase 4: Fix Infrastructure and Monitoring

---

### Task 4.1: Rename Grafana Alert Rules from "Airtable" to "Database"

**Problem:**
File `getmentor-infra/grafana/alerts/alerts.jsonnet` (lines 633-746) contains two alert rules with Airtable naming:

1. UID `airtable-high-latency`, title `Airtable API High Latency` — but the PromQL query uses `db_client_operation_duration_seconds_bucket` (a PostgreSQL metric)
2. UID `airtable-high-error-rate`, title `Airtable API High Error Rate` — uses `db_client_operation_total` (PostgreSQL metric)

Both alerts have:
- `dependency: 'airtable'` label
- Summaries/descriptions mentioning "Airtable API"

**Proposed Solution:**
1. Rename UIDs: `airtable-high-latency` → `database-high-latency`, `airtable-high-error-rate` → `database-high-error-rate`
2. Update titles: `Database High Latency`, `Database High Error Rate`
3. Update labels: `dependency: 'database'`
4. Update summaries and descriptions to reference "Database" instead of "Airtable"

**Files to Change:**
- `getmentor-infra/grafana/alerts/alerts.jsonnet` (lines 633-746)

**Definition of Done:**
- No "Airtable" in alert names, titles, summaries, descriptions, or labels
- Alert UIDs reference "database"
- Grafana alerts build correctly (test with `jsonnet` if available)

**Edge Cases:**
- Changing alert UIDs may create new alerts in Grafana while old ones persist. Old alerts may need to be manually deleted from the Grafana instance.
- Alert routing rules that match on `dependency: 'airtable'` will need updating

**Tests:**
- Validate jsonnet compiles: `jsonnet alerts.jsonnet` (if tooling available)
- After deployment, verify alerts appear correctly in Grafana UI

---

### Task 4.2: Rename Grafana Dashboard Panels from "Airtable" to "Database"

**Problem:**
File `getmentor-infra/grafana/dashboards/backend-deep-dive.jsonnet` (lines 97-117) contains panels titled:
- `Airtable Request Duration (p95)`
- `Airtable Requests by Status`

These panels use PostgreSQL metrics (`db_client_operation_*`).

File `getmentor-infra/grafana/lib/panels.libsonnet` (lines 292-304) contains:
```jsonnet
airtableLatencyTimeseries(title)::
```
with a metric `gm_api_airtable_request_duration_seconds_bucket` that doesn't exist anymore.

**Proposed Solution:**
1. Rename dashboard panels to `Database Request Duration (p95)` and `Database Requests by Status`
2. Rename `airtableLatencyTimeseries` → `dbLatencyTimeseries` in panels library
3. Update the metric in panels library to `db_client_operation_duration_seconds_bucket`

**Files to Change:**
- `getmentor-infra/grafana/dashboards/backend-deep-dive.jsonnet`
- `getmentor-infra/grafana/lib/panels.libsonnet`

**Definition of Done:**
- No "Airtable" in dashboard panel titles or function names
- Metrics reference correct PostgreSQL-based metric names
- Dashboards compile and render correctly

**Edge Cases:**
- If the old metric name `gm_api_airtable_request_duration_seconds_bucket` was ever emitted, historical data won't match the new metric name — this is acceptable as the old metric was for Airtable

**Tests:**
- Validate jsonnet compiles
- After deployment, verify dashboard panels show data

---

### Task 4.3: Add DATABASE_URL to Production Environment Config

**Problem:**
File `getmentor-infra/.env.production.example` is missing `DATABASE_URL`. The `docker-compose.yml` (line 102) references it:
```yaml
- DATABASE_URL=${DATABASE_URL}
```
But `.env.production.example` doesn't include it, so production deployments will fail.

**Proposed Solution:**
1. Add `DATABASE_URL` to `.env.production.example`:
```
# PostgreSQL Database (primary data source)
DATABASE_URL=postgres://getmentor:password@your-db-host:5432/getmentor?sslmode=require
```
2. Remove or clearly mark Airtable variables as deprecated in `.env.production.example`

**Files to Change:**
- `getmentor-infra/.env.production.example`
- `getmentor-infra/.env.example` (verify it already has DATABASE_URL)

**Definition of Done:**
- `DATABASE_URL` is documented in production example
- Airtable variables are either removed or clearly marked deprecated

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

### Task 4.4: Remove Airtable Variables from CI/CD

**Problem:**
File `getmentor-api/.github/workflows/build-and-test.yml` (lines 85-87):
```yaml
-e AIRTABLE_WORK_OFFLINE=1 \
-e AIRTABLE_API_KEY=test \
-e AIRTABLE_BASE_ID=test \
```

These are set in CI/CD but never used. After Task 2.2 removes the validation, they become completely unnecessary.

**Proposed Solution:**
Remove all `AIRTABLE_*` environment variables from the CI/CD workflow.

**Files to Change:**
- `getmentor-api/.github/workflows/build-and-test.yml`

**Definition of Done:**
- No `AIRTABLE_*` env vars in CI/CD
- CI/CD pipeline passes

**Edge Cases:**
- This must be done AFTER Task 2.2 (removing AirtableConfig validation), otherwise CI will fail because the validation requires these vars

**Tests:**
- Push a test branch and verify CI passes

---

### Task 4.5: Update Deployment Database Rollback Documentation

**Problem:**
File `getmentor-infra/docs/deployment.md` (lines 325-333):
```
### Database Rollback (if applicable)
If you use a database (currently you use Airtable, which is external):
# No database rollback needed for GetMentor
# Data is in Airtable (external) and Azure Storage (permanent)
```

This is dangerously incorrect. PostgreSQL IS the database now and requires proper backup/rollback procedures.

**Proposed Solution:**
Replace with actual PostgreSQL rollback documentation:
```markdown
### Database Rollback
PostgreSQL is the primary database. Before major deployments:
1. Create a database backup: `pg_dump $DATABASE_URL > backup_$(date +%Y%m%d).sql`
2. Run migrations: `migrate -path migrations -database $DATABASE_URL up`
3. To rollback: `migrate -path migrations -database $DATABASE_URL down 1`
4. To restore from backup: `psql $DATABASE_URL < backup_YYYYMMDD.sql`
```

**Files to Change:**
- `getmentor-infra/docs/deployment.md`

**Definition of Done:**
- Database rollback section references PostgreSQL
- No mention of "Airtable" in rollback context

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

## Phase 5: Fix Tests

---

### Task 5.1: Update Go API Config Tests

**Problem:**
File `getmentor-api/test/config/config_test.go` extensively tests Airtable configuration (lines 107-109, 133-135, 171-172, 187-188, 222-223, 254-256, 301, 303):
```go
Airtable: config.AirtableConfig{WorkOffline: true},

os.Setenv("AIRTABLE_WORK_OFFLINE", "false")
os.Setenv("AIRTABLE_API_KEY", "test-key-123")
os.Setenv("AIRTABLE_BASE_ID", "test-base-456")

assert.Equal(t, "test-key-123", cfg.Airtable.APIKey)
```

After Task 2.2 removes AirtableConfig, these tests will fail.

**Proposed Solution:**
1. Remove all `Airtable` fields from test config structs
2. Remove all `os.Setenv("AIRTABLE_*", ...)` calls
3. Remove all `assert.*` calls that reference `cfg.Airtable.*`
4. Add tests for `DATABASE_URL` configuration if not already present

**Files to Change:**
- `getmentor-api/test/config/config_test.go`

**Definition of Done:**
- No Airtable references in config tests
- Tests pass: `go test ./test/config/...`
- DATABASE_URL is tested

**Edge Cases:**
- Tests that verify validation should be updated to test DATABASE_URL validation instead

**Tests:** `go test ./test/config/...`

---

### Task 5.2: Update Go API Model Tests

**Problem:**
File `getmentor-api/test/internal/models/mentor_test.go` (lines 111-114):
```go
airtableID := "rec123"
mentor := &models.Mentor{
    AirtableID: &airtableID,
}
```

File `getmentor-api/test/internal/models/mentor_scan_test.go` (lines 70, 92, 123-125, 174, 209-211):
```go
airtableID := "rec123456"
values: []interface{}{mentorID, airtableID, ...}
if mentor.AirtableID == nil || *mentor.AirtableID != airtableID { ... }
```

After Task 2.4 removes the AirtableID field from the Mentor model, these tests will fail.

**Proposed Solution:**
1. Remove `AirtableID` from test Mentor structs
2. Remove `airtableID` from scan test value arrays (adjust column count/order)
3. Remove assertions that check `mentor.AirtableID`
4. Update scan tests to match the current column order returned by `SELECT * FROM mentors`

**Files to Change:**
- `getmentor-api/test/internal/models/mentor_test.go`
- `getmentor-api/test/internal/models/mentor_scan_test.go`

**Definition of Done:**
- No `AirtableID` references in tests
- `go test ./test/internal/models/...` passes

**Edge Cases:**
- The scan test simulates PostgreSQL row scanning — the column order must exactly match the `ScanMentor` function's scan order after AirtableID removal

**Tests:** `go test ./test/internal/models/...`

---

## Phase 6: Update All Documentation

---

### Task 6.1: Update Root CLAUDE.md

**Problem:**
File `CLAUDE.md` (root of monorepo) contains extensive Airtable references:
- Line 13: `- **Database**: Airtable (tables: Mentors, Client Requests, Moderators)`
- Lines 39-91: Architecture diagram shows Airtable as database
- Lines 93-106: Data flow references "creates Airtable record" and "Airtable webhook"
- Lines 183-205: Entire "Airtable Schema" section
- Lines 297-304: Environment variables section lists Airtable keys
- Lines 533, 579: Security section mentions Airtable
- Lines 563-574: Troubleshooting section for Airtable

**Proposed Solution:**
1. Line 13: Change to `- **Database**: PostgreSQL (migrated from Airtable)`
2. Lines 39-91: Redraw architecture diagram replacing Airtable box with PostgreSQL
3. Lines 93-106: Update data flow: "creates PostgreSQL record", "trigger notification via HTTP call"
4. Lines 183-205: Replace "Airtable Schema" with "PostgreSQL Schema" showing actual table definitions
5. Lines 297-304: Replace Airtable env vars with `DATABASE_URL`
6. Lines 533, 579: Remove Airtable security references
7. Lines 563-574: Replace with PostgreSQL troubleshooting

**Files to Change:**
- `CLAUDE.md` (root)

**Definition of Done:**
- No reference to Airtable as the current database
- Architecture diagram shows PostgreSQL
- All environment variable docs reference DATABASE_URL
- Troubleshooting section covers PostgreSQL

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

### Task 6.2: Update getmentor-api README.md and DEPLOYMENT.md

**Problem:**
`getmentor-api/README.md` contains multiple Airtable references:
- Line 6-7: "handles all backend operations including Airtable integration"
- Line 14: "Airtable integration for mentor data"
- Line 41-42: `` `pkg/airtable/` - Airtable client``
- Lines 70-74: Airtable environment variables
- Line 134: `POST /api/webhooks/airtable`
- Line 174-175: Airtable API request metrics
- Line 200: `AIRTABLE_WORK_OFFLINE`

`getmentor-api/DEPLOYMENT.md` contains:
- Lines 148-151: Airtable configuration section
- Lines 235-236, 245: Airtable env vars in tables

**Proposed Solution:**
Replace all Airtable references with PostgreSQL equivalents. Remove references to the Airtable webhook endpoint and `pkg/airtable/` package.

**Files to Change:**
- `getmentor-api/README.md`
- `getmentor-api/DEPLOYMENT.md`

**Definition of Done:**
- No reference to Airtable as current integration
- PostgreSQL is documented as the database
- `DATABASE_URL` replaces `AIRTABLE_*` in environment documentation

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

### Task 6.3: Update getmentor-func CLAUDE.md

**Problem:**
File `getmentor-func/CLAUDE.md` contains:
- Line 18-23: "The system is built around **Airtable as the primary database**"
- Line 49-50: References non-existent `lib/utils/airtable.ts`
- Line 86: References Airtable views and `.select()` queries
- Various: Describes `MentorStorageRecord` as "Airtable record format"

**Proposed Solution:**
Rewrite CLAUDE.md to describe the PostgreSQL-based architecture, referencing:
- `lib/utils/db.ts` (PgRowAdapter and pool)
- PostgreSQL queries instead of Airtable views
- `MentorStorageRecord` as the data access interface (not Airtable-specific)

**Files to Change:**
- `getmentor-func/CLAUDE.md`

**Definition of Done:**
- No Airtable references as current database
- Documentation matches actual code architecture

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

### Task 6.4: Update getmentor-bot CLAUDE.md

**Problem:**
File `getmentor-bot/CLAUDE.md` contains:
- Line 7: "It uses Airtable as the backend database"
- Line 32: "Attaches Airtable storage and loads mentor data"
- Line 38: "Airtable database interface"
- Lines 45-54: Section about `AirtableBase` class that no longer exists
- Lines 98-99: Documents `AIRTABLE_API_KEY` and `AIRTABLE_BASE_ID`
- Lines 118-121: "Airtable Schema" section

**Proposed Solution:**
Rewrite CLAUDE.md to describe the PostgreSQL-based architecture:
- Reference `PostgresStorage` class instead of `AirtableBase`
- Update environment variable docs to `DATABASE_URL`
- Replace schema section with PostgreSQL table descriptions

**Files to Change:**
- `getmentor-bot/CLAUDE.md`

**Definition of Done:**
- No Airtable references as current database
- Documentation matches actual code architecture

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

### Task 6.5: Update getmentor-infra Documentation

**Problem:**
Multiple infrastructure docs reference Airtable:
- `getmentor-infra/ENVIRONMENT_VARIABLES.md` — Lists `AIRTABLE_*` as required variables
- `getmentor-infra/grafana/README.md` — References Airtable as external dependency
- `getmentor-infra/docs/troubleshooting.md` — Has entire "Airtable API Errors" section and "Airtable Connection Errors" troubleshooting

**Proposed Solution:**
1. Update `ENVIRONMENT_VARIABLES.md`: Replace Airtable vars with `DATABASE_URL`, mark old vars as deprecated
2. Update `grafana/README.md`: Replace Airtable dependency with PostgreSQL
3. Update `docs/troubleshooting.md`: Replace Airtable troubleshooting with PostgreSQL sections

**Files to Change:**
- `getmentor-infra/ENVIRONMENT_VARIABLES.md`
- `getmentor-infra/grafana/README.md`
- `getmentor-infra/docs/troubleshooting.md`

**Definition of Done:**
- No Airtable references as current dependency in infrastructure docs
- PostgreSQL troubleshooting documented
- Environment variables documentation is accurate

**Edge Cases:** None.

**Tests:** None — documentation change only.

---

### Task 6.6: Update Trigger Package Comments

**Problem:**
File `getmentor-api/pkg/trigger/trigger.go` (lines 13-14):
```go
// CallAsync calls a trigger URL asynchronously with a record_id query parameter.
// This is used to trigger Azure Functions after Airtable operations.
```

The comment still says "after Airtable operations" when it should say "after database operations".

**Proposed Solution:**
Update comment to: `// This is used to trigger Azure Functions after database operations.`

**Files to Change:**
- `getmentor-api/pkg/trigger/trigger.go`

**Definition of Done:**
- Comment updated
- `go build ./...` succeeds

**Edge Cases:** None.

**Tests:** `go build ./...`

---

## Execution Order and Dependencies

```
Phase 1 (Runtime Fixes) — Can be done in parallel:
  1.1, 1.2, 1.3, 1.4, 1.5 (SQL column fixes)
  1.6 (HIGHLIGHTED_MENTORS — requires database access)
  1.7, 1.8 (URL replacements)
  1.9 (Photo field)

Phase 2 (Go API Cleanup) — Sequential:
  2.5 (Delete .bak file) — Independent
  2.4 (Remove deprecated functions) — Before 2.1
  2.2 (Remove AirtableConfig) — Before 2.1
  2.1 (Remove go.mod dependency) — After 2.2 and 2.4
  2.3 (JWT claims) — Independent but affects Phase 3
  2.6 (Retry function) — Independent
  2.7 (Profile service) — Independent
  2.8 (Tag updates) — Independent

Phase 3 (Frontend) — After 2.3:
  3.1 (MentorSession type) — After 2.3
  3.2, 3.3, 3.4, 3.5 — Independent

Phase 4 (Infrastructure) — Independent:
  4.1, 4.2, 4.3, 4.5 — Independent
  4.4 — After 2.2

Phase 5 (Tests) — After Phase 2:
  5.1 — After 2.2
  5.2 — After 2.4

Phase 6 (Documentation) — Last, after all code changes:
  6.1 through 6.6 — Can be done in parallel
```

---

## Total Task Count

| Phase | Tasks | Priority |
|---|---|---|
| Phase 1: Runtime Fixes | 9 | CRITICAL |
| Phase 2: Go API Cleanup | 8 | HIGH |
| Phase 3: Frontend | 5 | HIGH |
| Phase 4: Infrastructure | 5 | MEDIUM |
| Phase 5: Tests | 2 | MEDIUM |
| Phase 6: Documentation | 6 | LOW |
| **Total** | **35** | |
