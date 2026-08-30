ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS poll_request_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS poll_requested_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_submissions_poll_requests
    ON submissions(poll_requested_at)
    WHERE poll_request_id IS NOT NULL;
