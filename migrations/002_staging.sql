-- Staging tables for Airtable export data

CREATE TABLE IF NOT EXISTS stg_mentors (
  airtable_id TEXT,
  legacy_id BIGINT,
  name TEXT,
  job_title TEXT,
  workplace TEXT,
  about TEXT,
  details TEXT,
  competencies TEXT,
  experience TEXT,
  price TEXT,
  status TEXT,
  email TEXT,
  telegram TEXT,
  telegram_chat_id TEXT,
  tg_secret TEXT,
  comms_type TEXT,
  privacy BOOLEAN,
  avito BOOLEAN,
  ex_avito BOOLEAN,
  yandex BOOLEAN,
  sort_order INTEGER,
  login_token TEXT,
  login_token_expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS stg_tags (
  airtable_id TEXT,
  name TEXT
);

-- M:N links by Airtable record IDs
CREATE TABLE IF NOT EXISTS stg_mentor_tags (
  mentor_airtable_id TEXT,
  tag_airtable_id TEXT
);

CREATE TABLE IF NOT EXISTS stg_client_requests (
  airtable_id TEXT,
  mentor_airtable_id TEXT,
  email TEXT,
  name TEXT,
  telegram TEXT,
  description TEXT,
  level TEXT,
  status TEXT,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  status_changed_at TIMESTAMPTZ,
  scheduled_at TIMESTAMPTZ,
  decline_reason TEXT,
  decline_comment TEXT
);

CREATE TABLE IF NOT EXISTS stg_reviews (
  airtable_id TEXT,
  request_airtable_id TEXT,
  complete TEXT,
  helped TEXT,
  one_enough TEXT,
  again TEXT,
  nps TEXT,
  mentor_review TEXT,
  platform_review TEXT,
  improvements TEXT,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS stg_moderators (
  airtable_id TEXT,
  name TEXT,
  telegram TEXT,
  email TEXT,
  role TEXT,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ
);
