-- Add role and is_active to users
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('ADMIN', 'USER');
    END IF;
END $$;

ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(16) NOT NULL DEFAULT 'USER';
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

-- Add publication_status to contests
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'contest_publication_status') THEN
        CREATE TYPE contest_publication_status AS ENUM ('DRAFT', 'PUBLISHED');
    END IF;
END $$;

ALTER TABLE contests ADD COLUMN IF NOT EXISTS publication_status VARCHAR(16) NOT NULL DEFAULT 'PUBLISHED';
CREATE INDEX IF NOT EXISTS idx_contests_publication_status ON contests(publication_status);
