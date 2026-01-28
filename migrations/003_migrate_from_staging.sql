-- Data migration from staging tables into normalized schema

-- Tags
INSERT INTO tags (airtable_id, name, created_at, updated_at)
SELECT DISTINCT
  st.airtable_id,
  st.name,
  now(),
  now()
FROM stg_tags st
WHERE st.airtable_id IS NOT NULL AND st.name IS NOT NULL
ON CONFLICT (airtable_id) DO NOTHING;

-- Mentors
INSERT INTO mentors (
  airtable_id,
  legacy_id,
  slug,
  name,
  job_title,
  workplace,
  about,
  details,
  competencies,
  experience,
  price,
  status,
  email,
  telegram,
  telegram_chat_id,
  tg_secret,
  comms_type,
  privacy,
  avito,
  ex_avito,
  yandex,
  sort_order,
  login_token,
  login_token_expires_at,
  created_at,
  updated_at
)
SELECT
  sm.airtable_id,
  sm.legacy_id,
  CASE
    WHEN slugify(sm.name) = '' THEN sm.legacy_id::TEXT
    ELSE slugify(sm.name) || '-' || sm.legacy_id::TEXT
  END AS slug,
  sm.name,
  sm.job_title,
  sm.workplace,
  sm.about,
  sm.details,
  sm.competencies,
  sm.experience,
  sm.price,
  sm.status,
  sm.email,
  sm.telegram,
  sm.telegram_chat_id,
  sm.tg_secret,
  sm.comms_type,
  COALESCE(sm.privacy, FALSE),
  COALESCE(sm.avito, FALSE),
  COALESCE(sm.ex_avito, FALSE),
  COALESCE(sm.yandex, FALSE),
  sm.sort_order,
  sm.login_token,
  sm.login_token_expires_at,
  COALESCE(sm.created_at, now()),
  COALESCE(sm.updated_at, now())
FROM stg_mentors sm
WHERE sm.airtable_id IS NOT NULL
ON CONFLICT (airtable_id) DO NOTHING;

-- Reset legacy_id sequence after manual inserts
SELECT setval(pg_get_serial_sequence('mentors', 'legacy_id'), COALESCE((SELECT MAX(legacy_id) FROM mentors), 1), true);

-- Mentor tags
INSERT INTO mentor_tags (mentor_id, tag_id)
SELECT
  m.id,
  t.id
FROM stg_mentor_tags smt
JOIN mentors m ON m.airtable_id = smt.mentor_airtable_id
JOIN tags t ON t.airtable_id = smt.tag_airtable_id
ON CONFLICT DO NOTHING;

-- Client requests
INSERT INTO client_requests (
  airtable_id,
  mentor_id,
  email,
  name,
  telegram,
  description,
  level,
  status,
  created_at,
  updated_at,
  status_changed_at,
  scheduled_at,
  decline_reason,
  decline_comment
)
SELECT
  scr.airtable_id,
  m.id,
  scr.email,
  scr.name,
  scr.telegram,
  scr.description,
  scr.level,
  scr.status,
  COALESCE(scr.created_at, now()),
  COALESCE(scr.updated_at, now()),
  scr.status_changed_at,
  scr.scheduled_at,
  scr.decline_reason,
  scr.decline_comment
FROM stg_client_requests scr
LEFT JOIN mentors m ON m.airtable_id = scr.mentor_airtable_id
WHERE scr.airtable_id IS NOT NULL
ON CONFLICT (airtable_id) DO NOTHING;

-- Reviews (one per request)
INSERT INTO reviews (
  airtable_id,
  client_request_id,
  complete,
  helped,
  one_enough,
  again,
  nps,
  mentor_review,
  platform_review,
  improvements,
  created_at,
  updated_at
)
SELECT
  sr.airtable_id,
  cr.id,
  sr.complete,
  sr.helped,
  sr.one_enough,
  sr.again,
  sr.nps,
  sr.mentor_review,
  sr.platform_review,
  sr.improvements,
  COALESCE(sr.created_at, now()),
  COALESCE(sr.updated_at, now())
FROM stg_reviews sr
JOIN client_requests cr ON cr.airtable_id = sr.request_airtable_id
WHERE sr.airtable_id IS NOT NULL
ON CONFLICT (airtable_id) DO NOTHING;

-- Moderators
INSERT INTO moderators (
  airtable_id,
  name,
  telegram,
  email,
  role,
  created_at,
  updated_at
)
SELECT
  sm.airtable_id,
  sm.name,
  sm.telegram,
  sm.email,
  sm.role,
  COALESCE(sm.created_at, now()),
  COALESCE(sm.updated_at, now())
FROM stg_moderators sm
WHERE sm.airtable_id IS NOT NULL AND sm.name IS NOT NULL
ON CONFLICT (airtable_id) DO NOTHING;

-- NOTE: If your data contains multiple active mentors with the same email,
-- the partial unique index on mentors(email) WHERE status = 'active' will fail.
-- In that case, reconcile duplicates before running this migration.
