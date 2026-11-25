-- Mentee dashboard: one row per mentee email that has logged in at least once.
-- Mentees have no password and no registration; the row only exists to hold the
-- one-time login token (hash only) and the session_version that logout bumps.
-- Additive: nothing existing is changed.

CREATE TABLE IF NOT EXISTS mentees (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email CITEXT NOT NULL,
  -- SHA-256 hex of the emailed login token; the raw token is never stored
  login_token TEXT,
  login_token_expires_at TIMESTAMPTZ,
  -- Embedded in every session JWT and re-checked on each request; logout increments it
  session_version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT mentees_email_uniq UNIQUE (email)
);

CREATE UNIQUE INDEX IF NOT EXISTS mentees_login_token_uniq
  ON mentees (login_token)
  WHERE login_token IS NOT NULL;

-- The dashboard lists requests by the mentee's email, and login checks that one exists
CREATE INDEX IF NOT EXISTS client_requests_email_idx ON client_requests (email);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_mentees_updated_at') THEN
    CREATE TRIGGER trg_mentees_updated_at
    BEFORE UPDATE ON mentees
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;
