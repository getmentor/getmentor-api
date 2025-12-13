-- GetMentor PostgreSQL Schema Migration
-- Version: 001
-- Description: Initial schema for migrating from Airtable

-- Enable UUID extension for review tokens
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Tags table (lookup table)
CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

-- Mentors table
CREATE TABLE IF NOT EXISTS mentors (
    id SERIAL PRIMARY KEY,
    airtable_id VARCHAR(50) UNIQUE,  -- For migration reference, can remove later
    mentor_id INTEGER UNIQUE NOT NULL, -- Original "Id" field from Airtable
    slug VARCHAR(100) UNIQUE NOT NULL, -- "Alias" field
    name VARCHAR(255) NOT NULL,
    job_title VARCHAR(255),
    workplace VARCHAR(255),
    details TEXT,
    about TEXT,
    competencies TEXT,
    experience VARCHAR(50),
    price VARCHAR(100),
    sessions_count INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    is_visible BOOLEAN DEFAULT false,
    status VARCHAR(50) DEFAULT 'pending',
    auth_token VARCHAR(255),
    calendar_url VARCHAR(500),
    is_new BOOLEAN DEFAULT false,
    image_url VARCHAR(500),
    -- Telegram bot fields
    telegram_username VARCHAR(100),
    telegram_chat_id VARCHAR(50),
    tg_secret VARCHAR(20),
    --
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mentors_slug ON mentors(slug);
CREATE INDEX IF NOT EXISTS idx_mentors_mentor_id ON mentors(mentor_id);
CREATE INDEX IF NOT EXISTS idx_mentors_is_visible ON mentors(is_visible);
CREATE INDEX IF NOT EXISTS idx_mentors_status ON mentors(status);
CREATE INDEX IF NOT EXISTS idx_mentors_sort_order ON mentors(sort_order);
CREATE INDEX IF NOT EXISTS idx_mentors_telegram_chat_id ON mentors(telegram_chat_id);
CREATE INDEX IF NOT EXISTS idx_mentors_tg_secret ON mentors(tg_secret);

-- Mentor-Tags junction table (many-to-many)
CREATE TABLE IF NOT EXISTS mentor_tags (
    mentor_id INTEGER REFERENCES mentors(id) ON DELETE CASCADE,
    tag_id INTEGER REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (mentor_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_mentor_tags_mentor_id ON mentor_tags(mentor_id);
CREATE INDEX IF NOT EXISTS idx_mentor_tags_tag_id ON mentor_tags(tag_id);

-- Request status enum type
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'request_status') THEN
        CREATE TYPE request_status AS ENUM (
            'pending',      -- New request, not yet contacted
            'contacted',    -- Mentor contacted mentee
            'working',      -- Meeting scheduled
            'done',         -- Session completed (terminal)
            'declined',     -- Mentor declined (terminal)
            'unavailable',  -- Couldn't reach mentee
            'reschedule'    -- Meeting rescheduled
        );
    END IF;
END$$;

-- Client requests table
CREATE TABLE IF NOT EXISTS client_requests (
    id SERIAL PRIMARY KEY,
    airtable_id VARCHAR(50) UNIQUE,  -- For migration reference
    -- Core fields (used by API)
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    telegram VARCHAR(100),
    level VARCHAR(50),
    mentor_id INTEGER REFERENCES mentors(id),
    -- Status workflow (used by bot)
    status request_status DEFAULT 'pending',
    status_changed_at TIMESTAMP WITH TIME ZONE,
    -- Scheduling
    scheduled_at TIMESTAMP WITH TIME ZONE,  -- Kept for future use
    -- Review (populated via future review page)
    review TEXT,
    review_token UUID DEFAULT gen_random_uuid(),  -- For secure review submission links
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_client_requests_mentor_id ON client_requests(mentor_id);
CREATE INDEX IF NOT EXISTS idx_client_requests_email ON client_requests(email);
CREATE INDEX IF NOT EXISTS idx_client_requests_created_at ON client_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_client_requests_status ON client_requests(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_client_requests_review_token ON client_requests(review_token);

-- Partial indexes for Telegram bot queries (active and archived requests)
CREATE INDEX IF NOT EXISTS idx_client_requests_mentor_active
    ON client_requests(mentor_id, created_at)
    WHERE status NOT IN ('done', 'declined', 'unavailable');

CREATE INDEX IF NOT EXISTS idx_client_requests_mentor_archived
    ON client_requests(mentor_id, updated_at DESC)
    WHERE status NOT IN ('pending', 'working', 'contacted');

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for updated_at
DROP TRIGGER IF EXISTS update_mentors_updated_at ON mentors;
CREATE TRIGGER update_mentors_updated_at
    BEFORE UPDATE ON mentors
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_client_requests_updated_at ON client_requests;
CREATE TRIGGER update_client_requests_updated_at
    BEFORE UPDATE ON client_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Schema migrations tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Record this migration
INSERT INTO schema_migrations (version) VALUES (1) ON CONFLICT DO NOTHING;
