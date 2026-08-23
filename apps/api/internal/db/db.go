package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		host := os.Getenv("POSTGRES_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("POSTGRES_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("POSTGRES_USER")
		if user == "" {
			user = "cphub"
		}
		password := os.Getenv("POSTGRES_PASSWORD")
		if password == "" {
			password = "cphub_password"
		}
		dbname := os.Getenv("POSTGRES_DB")
		if dbname == "" {
			dbname = "cphub_db"
		}
		sslmode := os.Getenv("POSTGRES_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// EnsureSchema applies the initial migration if tables do not exist
func EnsureSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(36) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		username VARCHAR(64) UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS problems (
		id VARCHAR(36) PRIMARY KEY,
		platform VARCHAR(32) NOT NULL,
		external_id VARCHAR(128) NOT NULL,
		title VARCHAR(255) NOT NULL,
		url TEXT NOT NULL,
		difficulty INTEGER,
		tags JSONB NOT NULL DEFAULT '[]'::jsonb,
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT unique_platform_external_id UNIQUE(platform, external_id)
	);
	CREATE INDEX IF NOT EXISTS idx_problems_platform ON problems(platform);

	CREATE TABLE IF NOT EXISTS problem_sets (
		id VARCHAR(36) PRIMARY KEY,
		owner_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		visibility VARCHAR(32) NOT NULL DEFAULT 'PUBLIC',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_problem_sets_owner ON problem_sets(owner_id);

	CREATE TABLE IF NOT EXISTS problem_set_items (
		problem_set_id VARCHAR(36) NOT NULL REFERENCES problem_sets(id) ON DELETE CASCADE,
		problem_id VARCHAR(36) NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		PRIMARY KEY (problem_set_id, problem_id)
	);
	CREATE INDEX IF NOT EXISTS idx_problem_set_items_set_pos ON problem_set_items(problem_set_id, position);

	CREATE TABLE IF NOT EXISTS contests (
		id VARCHAR(36) PRIMARY KEY,
		owner_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		start_at TIMESTAMPTZ NOT NULL,
		end_at TIMESTAMPTZ NOT NULL,
		visibility VARCHAR(32) NOT NULL DEFAULT 'PUBLIC',
		scoring_type VARCHAR(32) NOT NULL DEFAULT 'ICPC',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_contests_times ON contests(start_at, end_at);

	CREATE TABLE IF NOT EXISTS contest_problems (
		contest_id VARCHAR(36) NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
		problem_id VARCHAR(36) NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		points INTEGER,
		label VARCHAR(8) NOT NULL DEFAULT 'A',
		PRIMARY KEY (contest_id, problem_id)
	);
	CREATE INDEX IF NOT EXISTS idx_contest_problems_contest_pos ON contest_problems(contest_id, position);

	CREATE TABLE IF NOT EXISTS contest_participants (
		contest_id VARCHAR(36) NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
		user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (contest_id, user_id)
	);

	CREATE TABLE IF NOT EXISTS submissions (
		id VARCHAR(36) PRIMARY KEY,
		user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		problem_id VARCHAR(36) NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
		contest_id VARCHAR(36) REFERENCES contests(id) ON DELETE SET NULL,
		platform VARCHAR(32) NOT NULL,
		language VARCHAR(32) NOT NULL,
		source_code TEXT NOT NULL,
		status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
		external_submission_id VARCHAR(128),
		submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		judged_at TIMESTAMPTZ,
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb
	);
	CREATE INDEX IF NOT EXISTS idx_submissions_user ON submissions(user_id);
	CREATE INDEX IF NOT EXISTS idx_submissions_contest ON submissions(contest_id);
	CREATE INDEX IF NOT EXISTS idx_submissions_problem ON submissions(problem_id);
	CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);

	CREATE TABLE IF NOT EXISTS integrations (
		user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		platform VARCHAR(32) NOT NULL,
		external_username VARCHAR(128) NOT NULL,
		connection_status VARCHAR(32) NOT NULL DEFAULT 'CONNECTED',
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (user_id, platform)
	);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to ensure schema: %w", err)
	}
	log.Println("Database schema verified")
	return nil
}
