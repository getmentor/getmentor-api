# Airtable -> Postgres Migration (Mentors, Client Requests, Reviews, Tags, Moderators)

This folder contains plain SQL migrations for initializing the schema and importing Airtable data.

## Files
- `001_schema.sql` - Core schema + constraints + triggers.
- `002_staging.sql` - Staging tables for raw Airtable exports.
- `003_migrate_from_staging.sql` - Data import into normalized tables.

## Quick flow
1. Run schema + staging:
   - `001_schema.sql`
   - `002_staging.sql`
2. Load Airtable exports into staging tables.
3. Run `003_migrate_from_staging.sql`.

## Expected staging data
Populate the following staging tables from Airtable export (CSV or JSON->COPY):

- `stg_mentors`
  - `airtable_id` (record ID)
  - `legacy_id` (Airtable auto-number Id)
  - `name`, `job_title`, `workplace`, `about`, `details`, `competencies`, `experience`, `price`
  - `status`, `email`, `telegram`, `telegram_chat_id`, `tg_secret`, `comms_type`
  - `privacy`, `avito`, `ex_avito`, `yandex`, `sort_order`
  - `login_token`, `login_token_expires_at`
  - `created_at`, `updated_at`

- `stg_tags`
  - `airtable_id`, `name`

- `stg_mentor_tags`
  - `mentor_airtable_id`, `tag_airtable_id`

- `stg_client_requests`
  - `airtable_id`, `mentor_airtable_id`
  - `email`, `name`, `telegram`, `description`, `level`, `status`
  - `created_at`, `updated_at`, `status_changed_at`, `scheduled_at`
  - `decline_reason`, `decline_comment`

- `stg_reviews`
  - `airtable_id`, `request_airtable_id`
  - `complete`, `helped`, `one_enough`, `again`, `nps`
  - `mentor_review`, `platform_review`, `improvements`
  - `created_at`, `updated_at`

- `stg_moderators`
  - `airtable_id`, `name`, `telegram`, `email`, `role`
  - `created_at`, `updated_at`

## Notes
- Slug is generated as `slugify(name) + '-' + legacy_id` during migration.
- All timestamps are stored in UTC (`timestamptz`).
- Active email uniqueness is enforced via a partial unique index.
- Mentor deletion does **not** delete client requests (`ON DELETE SET NULL`).

If you want me to generate import scripts (CSV->COPY or JSON processing), tell me the export format and I will add it.
