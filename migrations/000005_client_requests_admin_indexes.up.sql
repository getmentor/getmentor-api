-- Speed up the admin requests list (sorted by creation date, newest first)
CREATE INDEX IF NOT EXISTS client_requests_created_at_idx ON client_requests (created_at DESC);
