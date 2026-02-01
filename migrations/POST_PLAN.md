Comprehensive Post-Migration Audit Report

Executive Summary

After auditing all 5 repositories (getmentor-api, getmentor-func, getmentor-bot, getmentor.dev, getmentor-infra + root CLAUDE.md), I
found 68 issues across the stack. The migration code compiles but contains runtime-breaking bugs, hardcoded Airtable URLs,
missing/wrong PostgreSQL column names, and extensively outdated documentation.
┌──────────┬───────┐
│ Severity │ Count │
├──────────┼───────┤
│ CRITICAL │ 15    │
├──────────┼───────┤
│ HIGH     │ 22    │
├──────────┼───────┤
│ MEDIUM   │ 19    │
├──────────┼───────┤
│ LOW      │ 12    │
└──────────┴───────┘
---
CRITICAL ISSUES (Will cause runtime failures or broken user flows)

C1. Wrong/Missing PostgreSQL Column Names in getmentor-func Queries

Multiple Azure Functions reference columns that do not exist in the PostgreSQL schema:
┌────────────────────────┬────────────────────────────────────────────┬──────────────────────────────────────────────────────┐
│      Column Used       │            Actual Schema Column            │                        Files                         │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ pending_sessions_count │ Does not exist (must be computed via JOIN) │ sessions-watcher/index.ts                            │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ created_days_ago       │ Does not exist (compute from created_at)   │ sessions-watcher/index.ts                            │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ is_visible             │ Does not exist in mentors table            │ tg-mass-send/index.ts, randomize-sort-order/index.ts │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ last_status_change     │ status_changed_at                          │ update-status-reminder/index.ts                      │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ comms_type             │ Does not exist                             │ new-mentor-watcher/index.ts                          │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ auth_token             │ login_token                                │ new-mentor-watcher/index.ts                          │
├────────────────────────┼────────────────────────────────────────────┼──────────────────────────────────────────────────────┤
│ auth_token_expiry      │ login_token_expires_at                     │ new-mentor-watcher/index.ts                          │
└────────────────────────┴────────────────────────────────────────────┴──────────────────────────────────────────────────────┘
Impact: Every one of these functions will crash at runtime with "column does not exist" errors.

C2. Wrong/Missing PostgreSQL Columns in getmentor-bot Queries
┌────────────────────┬──────────────────────────────────────────────────────┬─────────────────────────────────────┐
│    Column Used     │                 Actual Schema Column                 │                Files                │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ profile_url        │ Does not exist                                       │ PostgresStorage.ts field mapping    │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ image              │ Does not exist                                       │ PostgresStorage.ts field mapping    │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ tags / tags_links  │ Stored in junction table mentor_tags, not as columns │ PostgresStorage.ts field mapping    │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ review / review2   │ Stored in separate reviews table                     │ PostgresStorage.ts field mapping    │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ review_form_url    │ Does not exist                                       │ PostgresStorage.ts field mapping    │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ auth_token         │ Should be login_token                                │ PostgresStorage.ts field mapping    │
├────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────┤
│ last_status_change │ status_changed_at                                    │ PostgresStorage.ts:223 UPDATE query │
└────────────────────┴──────────────────────────────────────────────────────┴─────────────────────────────────────┘
Impact: Bot will return undefined for mentor photos, tags, reviews, and auth tokens. Status updates will crash.

C3. Hardcoded Airtable URLs in Telegram Messages

- getmentor-func/lib/telegram/messages/NewRequestModeratorNotificationMessage.ts:28 —
  https://airtable.com/tblCA5xeV12ufn0iQ/.../${this._request.id}
- getmentor-func/lib/telegram/messages/NewMentorModeratorNotificationMessage.ts:57 —
  https://airtable.com/tblt7APgEGkR5VwTR/.../${this._mentor.id}

Impact: Moderators clicking these links get 404s. Critical admin workflow broken.

C4. Hardcoded Airtable Form URL in Email Templates

- getmentor-func/lib/postbox/templates/session-complete.ts —
  https://airtable.com/shrFNIXY2dRqqGjAi?prefill_RequestRecordId={{request_id}}
- Same template exists in getmentor-bot/getmentor-bot/postbox/templates/session-complete.ts

Impact: Mentee review/feedback form completely broken. Critical user flow.

C5. Airtable Record IDs in HIGHLIGHTED_MENTORS Config

- getmentor-func/local.settings.json:31 — "HIGHLIGHTED_MENTORS": "recybFvnrWxphlbz6,recrYwJ92K9OX11W0,rec9IBOl74sE8hOnx"

Impact: randomize-sort-order function will fail to match any mentors. Highlighting feature broken.

C6. Airtable Import and AirtableID Still in Go API Core

- go.mod:13 — github.com/mehanizm/airtable v0.3.4 still a dependency
- internal/models/mentor.go:8 — imports github.com/mehanizm/airtable
- internal/models/mentor.go:15 — AirtableID *string json:"airtableId" exposed in JSON responses
- internal/models/mentor_client_request.go:8 — same airtable import
- internal/models/mentor.go:212-287 — full AirtableRecordToMentor() function still present
- internal/models/mentor_client_request.go:185-220 — full AirtableRecordToMentorClientRequest() still present

C7. Airtable Config Still Validated and Required

- config/config.go:259-266 — Validation requires AIRTABLE_API_KEY and AIRTABLE_BASE_ID when AIRTABLE_WORK_OFFLINE is false
- config/config.go:16 — AirtableConfig struct still in Config

Impact: Backend won't start without setting fake Airtable env vars.

C8. AirtableID in JWT Claims

- pkg/jwt/jwt.go:21 — AirtableID string json:"airtable_id" in MentorClaims
- pkg/jwt/jwt.go:44 — GenerateToken(mentorID int, airtableID, email, name string)

C9. Frontend MentorSession Type Has airtable_id

- getmentor.dev/src/types/mentor-requests.ts:128 — airtable_id: string in MentorSession interface

Impact: Type contract mismatch with Go API. Will break when backend changes JWT claims.

C10. Missing DATABASE_URL in Production Config

- getmentor-infra/.env.production.example — DATABASE_URL is completely absent

Impact: Production deployments won't have database connectivity configured.

C11. Root CLAUDE.md Declares Airtable as Database

- CLAUDE.md:13 — **Database**: Airtable (tables: Mentors, Client Requests, Moderators)
- CLAUDE.md:39-91 — Entire architecture diagram shows Airtable
- CLAUDE.md:93-106 — Data flow references "creates Airtable record" and "Airtable webhook"

C12. Mentor Photo Field References Airtable Attachment Format

- getmentor-func/lib/data/mentor.ts:53 — record.fields['Image_Attachment'] ? record.fields['Image_Attachment'][0] : {}

Impact: Photo extraction completely broken. Airtable attachment format doesn't exist in PostgreSQL.

  ---
HIGH SEVERITY ISSUES

H1. Backup File Left in Codebase

- getmentor-api/internal/repository/mentor_repository_airtable.go.bak — Entire old Airtable repository implementation

H2. Retry Package Has Airtable-Specific Config

- getmentor-api/pkg/retry/retry.go:45-51 — AirtableConfig() function

H3. Trigger Package Comments Reference Airtable

- getmentor-api/pkg/trigger/trigger.go:13-14 — Comments say "trigger Azure Functions after Airtable operations"

H4. Airtable Environment Variables in CI/CD

- getmentor-api/.github/workflows/build-and-test.yml:85-87 — Sets AIRTABLE_WORK_OFFLINE=1, AIRTABLE_API_KEY=test,
  AIRTABLE_BASE_ID=test

H5. Config Tests Still Test Airtable Settings

- getmentor-api/test/config/config_test.go:107-109, 133-135, 171-172, 187-188, 222-223, 254-256

H6. Mentor Tests Reference AirtableID

- getmentor-api/test/internal/models/mentor_test.go:111-114
- getmentor-api/test/internal/models/mentor_scan_test.go:70, 92, 123-125

H7. README.md in getmentor-api References Airtable Throughout

- Lines 6-7, 14, 41-42, 70-74, 134, 174-175, 200

H8. DEPLOYMENT.md in getmentor-api References Airtable

- Lines 148-151, 235-236, 245

H9. Grafana Alert Names Reference Airtable

- getmentor-infra/grafana/alerts/alerts.jsonnet:633-746 — Alerts named "Airtable API High Latency" and "Airtable API High Error Rate"
  with dependency: 'airtable' labels

H10. Grafana Dashboard Panels Named "Airtable"

- getmentor-infra/grafana/dashboards/backend-deep-dive.jsonnet:97-117 — Panels titled "Airtable Request Duration" and "Airtable
  Requests by Status"

H11. Airtable Env Vars in Production Example

- getmentor-infra/.env.production.example:74-77 — AIRTABLE_API_KEY, AIRTABLE_BASE_ID

H12. Outdated Deployment Rollback Strategy

- getmentor-infra/docs/deployment.md:325-333 — Says "currently you use Airtable, which is external" and "No database rollback needed"

H13. Root CLAUDE.md Schema Section

- CLAUDE.md:183-205 — Entire "Airtable Schema" section with Airtable field names

H14. Root CLAUDE.md Environment Variables

- CLAUDE.md:301-304 — Lists AIRTABLE_API_KEY and AIRTABLE_BASE_ID as required

H15. Frontend Metrics Regex Matches rec Prefix

- getmentor.dev/src/lib/with-observability.ts:20-24 — Normalizes paths matching rec[A-Za-z0-9]+ pattern. With UUIDs, this won't match
  and metrics cardinality will explode.

H16. getmentor-func CLAUDE.md Completely Outdated

- References Airtable as primary database throughout (lines 18-23, 49-50, 86)

H17. getmentor-bot CLAUDE.md Completely Outdated

- References Airtable throughout (lines 7, 32, 38, 45-54, 98-99, 107-108, 116-121)

H18. Deprecated Profile Service Methods Still Present

- getmentor-api/internal/services/profile_service.go:41-52, 115-126 — SaveProfile() and UploadProfilePicture() deprecated but fully
  implemented

H19. PgRowAdapter Field Mapping Inconsistency (calendly_url vs calendar_url)

- Schema has calendar_url but PgRowAdapter maps to calendly_url

H20. Frontend getOneMentorByRecordId Still Exposed

- getmentor.dev/src/server/mentors-data.ts:48-58 and src/lib/go-api-client.ts:176-191

H21. ENVIRONMENT_VARIABLES.md References Airtable as Required

- getmentor-infra/ENVIRONMENT_VARIABLES.md — Multiple lines list Airtable variables as required

H22. Airtable Comments in getmentor-func Code

- request-process-finished/index.ts:17 — "The Airtable record ID"
- tg-mass-send/index.ts:23 — "Airtable formula conversion"

  ---
MEDIUM & LOW SEVERITY ISSUES

Medium (19): Stale comments, misleading variable names (selectedAirtableExperience), outdated package-lock.json with airtable, test
mock data using rec prefix, various documentation inconsistencies across repos.

Low (12): Documentation cleanup, legacy TODO comments, unused Grafana panel library functions, migration plan doc references.

  ---
REMEDIATION PLAN

Phase 1: Fix Runtime-Breaking Bugs (CRITICAL)

Task 1.1: Fix PostgreSQL Column Names in getmentor-func

Files: sessions-watcher/index.ts, tg-mass-send/index.ts, randomize-sort-order/index.ts, update-status-reminder/index.ts,
new-mentor-watcher/index.ts
Work:
- Replace pending_sessions_count with a subquery: (SELECT COUNT(*) FROM client_requests WHERE mentor_id = m.id AND status IN
  ('pending','contacted','working'))
- Replace created_days_ago with EXTRACT(DAY FROM NOW() - created_at)::int AS created_days_ago
- Replace is_visible with appropriate status check (status = 'active')
- Replace last_status_change → status_changed_at
- Replace comms_type → remove (or add column to schema)
- Replace auth_token → login_token, auth_token_expiry → login_token_expires_at

Task 1.2: Fix PostgreSQL Column Names in getmentor-bot

Files: lib/storage/postgres/PostgresStorage.ts
Work:
- Fix last_status_change → status_changed_at in UPDATE query (line 223)
- Fix auth_token → login_token in field mapping
- Handle tags via JOIN query to mentor_tags + tags tables
- Handle reviews via JOIN to reviews table or remove from bot model
- Handle profile_url — check if it exists in schema or compute from slug
- Handle image / Image_Attachment — check schema for photo_url or similar
- Handle review_form_url — either add to schema or remove

Task 1.3: Fix Mentor Photo Field in getmentor-func

File: lib/data/mentor.ts:53
Work: Replace record.fields['Image_Attachment'] with PostgreSQL photo_url column access

Task 1.4: Replace Hardcoded Airtable URLs

Files:
- getmentor-func/lib/telegram/messages/NewRequestModeratorNotificationMessage.ts
- getmentor-func/lib/telegram/messages/NewMentorModeratorNotificationMessage.ts
- getmentor-func/lib/postbox/templates/session-complete.ts
- getmentor-bot/getmentor-bot/postbox/templates/session-complete.ts
  Work: Replace Airtable links with appropriate admin panel URLs or remove if no replacement exists yet

Task 1.5: Update HIGHLIGHTED_MENTORS Config

File: getmentor-func/local.settings.json
Work: Replace rec IDs with actual PostgreSQL UUIDs (or legacy_ids)

Phase 2: Remove Airtable from Go API Core (CRITICAL)

Task 2.1: Remove Airtable Dependency from go.mod

Work: Delete AirtableRecordToMentor(), AirtableRecordToMentorClientRequest(), remove airtable import from models, run go mod tidy

Task 2.2: Remove AirtableConfig Validation

File: config/config.go
Work: Remove AirtableConfig struct, remove validation block (lines 259-266), remove defaults

Task 2.3: Refactor JWT Claims

File: pkg/jwt/jwt.go
Work: Rename AirtableID → MentorUUID (or similar), update GenerateToken() signature

Task 2.4: Clean Up Remaining Legacy Code

Work:
- Delete mentor_repository_airtable.go.bak
- Remove/rename AirtableConfig() in pkg/retry/retry.go
- Update comments in pkg/trigger/trigger.go
- Remove deprecated SaveProfile()/UploadProfilePicture() methods if unused

Phase 3: Fix Frontend Types (HIGH)

Task 3.1: Fix MentorSession Type

File: getmentor.dev/src/types/mentor-requests.ts
Work: Replace airtable_id: string with field names matching Go API

Task 3.2: Fix Metrics Normalization

File: getmentor.dev/src/lib/with-observability.ts
Work: Update regex to match UUIDs instead of rec prefix

Task 3.3: Clean Up Legacy Frontend References

Work: Rename selectedAirtableExperience, update test mock data from rec1 to UUIDs, update comments

Phase 4: Fix Infrastructure & Monitoring (HIGH)

Task 4.1: Update Grafana Alerts

File: getmentor-infra/grafana/alerts/alerts.jsonnet
Work: Rename "Airtable API High Latency" → "Database High Latency", update descriptions, change dependency: 'airtable' → 'database'

Task 4.2: Update Grafana Dashboards

File: getmentor-infra/grafana/dashboards/backend-deep-dive.jsonnet
Work: Rename panel titles from "Airtable" to "Database"

Task 4.3: Update Environment Configs

Files: .env.example, .env.production.example, ENVIRONMENT_VARIABLES.md
Work: Add DATABASE_URL, remove or deprecate Airtable variables

Task 4.4: Update CI/CD

File: getmentor-api/.github/workflows/build-and-test.yml
Work: Remove AIRTABLE_* env vars, add DATABASE_URL for test database

Phase 5: Fix Tests (HIGH)

Task 5.1: Update Go API Tests

Files: test/config/config_test.go, test/internal/models/mentor_test.go, test/internal/models/mentor_scan_test.go
Work: Remove Airtable config tests, update model tests to remove AirtableID references

Phase 6: Update All Documentation (MEDIUM)

Task 6.1: Rewrite Root CLAUDE.md

Work: Replace Airtable with PostgreSQL in: tech stack, architecture diagram, data flow, schema section, env vars, troubleshooting

Task 6.2: Update Per-Repo Documentation

Files: getmentor-api/README.md, getmentor-api/DEPLOYMENT.md, getmentor-func/CLAUDE.md, getmentor-bot/CLAUDE.md,
getmentor.dev/CLAUDE.md
Work: Remove all Airtable references, document PostgreSQL setup

Task 6.3: Update Infrastructure Docs

Files: getmentor-infra/DEPLOYMENT.md, getmentor-infra/docs/deployment.md, getmentor-infra/docs/troubleshooting.md,
getmentor-infra/grafana/README.md
Work: Replace Airtable troubleshooting with PostgreSQL, update rollback strategy, update dependency lists

  ---
This is the full audit. Phases 1-2 are the most urgent as they contain runtime-breaking bugs. Want me to start executing this plan?
