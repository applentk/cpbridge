DROP INDEX IF EXISTS idx_submissions_poll_requests;
ALTER TABLE submissions
    DROP COLUMN IF EXISTS poll_request_id,
    DROP COLUMN IF EXISTS poll_requested_at;
