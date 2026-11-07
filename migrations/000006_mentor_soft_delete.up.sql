-- Soft delete for mentors: a mentor with deleted_at set is treated as deleted everywhere
ALTER TABLE mentors ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Admin "deleted" list is sorted by deletion time
CREATE INDEX IF NOT EXISTS mentors_deleted_at_idx ON mentors (deleted_at DESC) WHERE deleted_at IS NOT NULL;

-- Deleted mentors must not block a new registration with the same email
DROP INDEX IF EXISTS mentors_active_email_uniq;
CREATE UNIQUE INDEX IF NOT EXISTS mentors_active_email_uniq
  ON mentors (email)
  WHERE status = 'active' AND telegram_chat_id IS NOT NULL AND email IS NOT NULL AND deleted_at IS NULL;
