-- A failed client-dispatch record should not prevent retrying the same source.
DROP INDEX IF EXISTS idx_submissions_unique_source;

CREATE UNIQUE INDEX idx_submissions_unique_source
    ON submissions(user_id, problem_id, language, source_hash)
    WHERE source_hash IS NOT NULL AND status <> 'FAILED';
