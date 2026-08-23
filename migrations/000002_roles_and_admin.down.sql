ALTER TABLE contests DROP COLUMN IF EXISTS publication_status;
DROP TYPE IF EXISTS contest_publication_status;

ALTER TABLE users DROP COLUMN IF EXISTS is_active;
ALTER TABLE users DROP COLUMN IF EXISTS role;
DROP TYPE IF EXISTS user_role;
