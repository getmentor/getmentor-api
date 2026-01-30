# Airtable -> Postgres Migration (Mentors, Client Requests, Reviews, Tags, Moderators)

This folder contains plain SQL migrations for initializing the schema and importing Airtable data.

## Files
- `001_schema.sql` - Core schema + constraints + triggers.

## Quick flow
1. Run schema + staging:
   - `001_schema.sql`
2. Load Airtable exports into staging tables.

## Notes
- All timestamps are stored in UTC (`timestamptz`).
- Active email uniqueness is enforced via a partial unique index.
- Mentor deletion does **not** delete client requests (`ON DELETE SET NULL`).
