-- Map GetMentor mentor slugs to their openmentor.io counterparts for cross-site profile links

CREATE TABLE IF NOT EXISTS openmentor_profiles (
  getmentor_slug TEXT PRIMARY KEY REFERENCES mentors(slug) ON UPDATE CASCADE ON DELETE CASCADE,
  openmentor_slug TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Updated_at trigger
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_openmentor_profiles_updated_at') THEN
    CREATE TRIGGER trg_openmentor_profiles_updated_at
    BEFORE UPDATE ON openmentor_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;
