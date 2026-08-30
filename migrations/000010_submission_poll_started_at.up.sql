ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS poll_started_at TIMESTAMPTZ;
