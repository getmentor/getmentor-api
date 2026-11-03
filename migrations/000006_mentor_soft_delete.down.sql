DROP INDEX IF EXISTS mentors_active_email_uniq;
CREATE UNIQUE INDEX IF NOT EXISTS mentors_active_email_uniq
  ON mentors (email)
  WHERE status = 'active' AND telegram_chat_id IS NOT NULL AND email IS NOT NULL;

DROP INDEX IF EXISTS mentors_deleted_at_idx;
ALTER TABLE mentors DROP COLUMN IF EXISTS deleted_at;
