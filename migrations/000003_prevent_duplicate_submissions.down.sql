DROP INDEX IF EXISTS idx_submissions_unique_source;
ALTER TABLE submissions DROP COLUMN IF EXISTS source_hash;
