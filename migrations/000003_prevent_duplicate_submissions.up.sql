-- A source digest lets us enforce duplicate prevention without indexing the
-- potentially very large source_code text column.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS source_hash VARCHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS idx_submissions_unique_source
    ON submissions(user_id, problem_id, language, source_hash)
    WHERE source_hash IS NOT NULL;
