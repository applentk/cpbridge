ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS external_submitted_at TIMESTAMPTZ;
