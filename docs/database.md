# Database Schema & Entity Relationships

CP Hub uses PostgreSQL 16 with versioned migrations in `migrations/`.

---

## Tables

### `users`
- `id`: VARCHAR(36) PRIMARY KEY (`usr_...`)
- `email`: VARCHAR(255) UNIQUE NOT NULL
- `username`: VARCHAR(64) UNIQUE NOT NULL
- `password_hash`: TEXT NOT NULL
- `created_at`: TIMESTAMPTZ NOT NULL DEFAULT NOW()
- `updated_at`: TIMESTAMPTZ NOT NULL DEFAULT NOW()

### `problems`
- `id`: VARCHAR(36) PRIMARY KEY (`prb_...`)
- `platform`: VARCHAR(32) NOT NULL (`CODEFORCES`, `ATCODER`)
- `external_id`: VARCHAR(128) NOT NULL (e.g. `1900/A`, `abc350/abc350_f`)
- `title`: VARCHAR(255) NOT NULL
- `url`: TEXT NOT NULL
- `difficulty`: INTEGER NULL
- `tags`: JSONB NOT NULL DEFAULT '[]'
- `metadata`: JSONB NOT NULL DEFAULT '{}'
- `created_at`, `updated_at`: TIMESTAMPTZ
- `CONSTRAINT unique_platform_external_id UNIQUE(platform, external_id)`

### `problem_sets`
- `id`: VARCHAR(36) PRIMARY KEY (`set_...`)
- `owner_id`: VARCHAR(36) REFERENCES users(id) ON DELETE CASCADE
- `name`: VARCHAR(255) NOT NULL
- `description`: TEXT NOT NULL DEFAULT ''
- `visibility`: VARCHAR(32) NOT NULL DEFAULT 'PUBLIC' (`PUBLIC`, `UNLISTED`, `PRIVATE`)
- `created_at`, `updated_at`: TIMESTAMPTZ

### `problem_set_items`
- `problem_set_id`: VARCHAR(36) REFERENCES problem_sets(id) ON DELETE CASCADE
- `problem_id`: VARCHAR(36) REFERENCES problems(id) ON DELETE CASCADE
- `position`: INTEGER NOT NULL
- `PRIMARY KEY (problem_set_id, problem_id)`

### `contests`
- `id`: VARCHAR(36) PRIMARY KEY (`con_...`)
- `owner_id`: VARCHAR(36) REFERENCES users(id) ON DELETE CASCADE
- `name`: VARCHAR(255) NOT NULL
- `description`: TEXT NOT NULL DEFAULT ''
- `start_at`: TIMESTAMPTZ NOT NULL
- `end_at`: TIMESTAMPTZ NOT NULL
- `visibility`: VARCHAR(32) NOT NULL DEFAULT 'PUBLIC'
- `scoring_type`: VARCHAR(32) NOT NULL DEFAULT 'ICPC' (`ICPC`, `SIMPLE`)
- `created_at`, `updated_at`: TIMESTAMPTZ

### `contest_problems` (Deep Snapshot)
- `contest_id`: VARCHAR(36) REFERENCES contests(id) ON DELETE CASCADE
- `problem_id`: VARCHAR(36) REFERENCES problems(id) ON DELETE CASCADE
- `position`: INTEGER NOT NULL
- `points`: INTEGER NULL
- `label`: VARCHAR(8) NOT NULL DEFAULT 'A'
- `PRIMARY KEY (contest_id, problem_id)`

### `contest_participants`
- `contest_id`: VARCHAR(36) REFERENCES contests(id) ON DELETE CASCADE
- `user_id`: VARCHAR(36) REFERENCES users(id) ON DELETE CASCADE
- `joined_at`: TIMESTAMPTZ NOT NULL DEFAULT NOW()
- `PRIMARY KEY (contest_id, user_id)`

### `submissions`
- `id`: VARCHAR(36) PRIMARY KEY (`sub_...`)
- `user_id`: VARCHAR(36) REFERENCES users(id) ON DELETE CASCADE
- `problem_id`: VARCHAR(36) REFERENCES problems(id) ON DELETE CASCADE
- `contest_id`: VARCHAR(36) REFERENCES contests(id) ON DELETE SET NULL
- `platform`: VARCHAR(32) NOT NULL
- `language`: VARCHAR(32) NOT NULL
- `source_code`: TEXT NOT NULL
- `status`: VARCHAR(32) NOT NULL DEFAULT 'PENDING'
- `external_submission_id`: VARCHAR(128) NULL
- `submitted_at`: TIMESTAMPTZ NOT NULL DEFAULT NOW()
- `judged_at`: TIMESTAMPTZ NULL
- `metadata`: JSONB NOT NULL DEFAULT '{}'

### `integrations`
- `user_id`: VARCHAR(36) REFERENCES users(id) ON DELETE CASCADE
- `platform`: VARCHAR(32) NOT NULL
- `external_username`: VARCHAR(128) NOT NULL
- `connection_status`: VARCHAR(32) NOT NULL DEFAULT 'CONNECTED'
- `updated_at`: TIMESTAMPTZ NOT NULL DEFAULT NOW()
- `PRIMARY KEY (user_id, platform)`
