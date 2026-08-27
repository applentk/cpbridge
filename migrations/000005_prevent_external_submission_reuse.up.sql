-- An external judge submission may be linked to at most one cpbridge
-- submission for a platform. PostgreSQL permits multiple NULL values, so
-- pending submissions remain unlinked until dispatch verification succeeds.
DO $$
BEGIN
    ALTER TABLE submissions
        ADD CONSTRAINT unique_platform_external_submission_id
        UNIQUE (platform, external_submission_id);
EXCEPTION
    WHEN duplicate_object OR duplicate_table THEN NULL;
END $$;
