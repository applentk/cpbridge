# cpbridge — Current Database Schema

cpbridge uses PostgreSQL 16. The server currently calls `apps/api/internal/db.EnsureSchema`, which applies idempotent `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`, and index statements at startup. SQL files in `migrations/` describe the schema history, but the running server does not contain a separate migration-runner.

All timestamps are `TIMESTAMPTZ`. Application-generated IDs are opaque, prefixed values such as `usr_...`, `prb_...`, `set_...`, `con_...`, and `sub_...`.

## Tables

### `users`

- `id`: primary key, normally `usr_...`
- `email`, `username`: unique login/profile fields
- `password_hash`: bcrypt password hash
- `role`: `USER` or `ADMIN`
- `is_active`: account enable/disable flag
- `created_at`, `updated_at`

### `problems`

- `id`: primary key, normally `prb_...`
- `platform`: currently `CODEFORCES` or `ATCODER`
- `external_id`: normalized platform ID, for example `1900/A` or `abc350/abc350_f`
- `title`, `url`, `difficulty`
- `tags`: JSONB array
- `metadata`: JSONB object; custom statements store `statement`, `timeLimit`, `memoryLimit`, and `sampleCases` here
- unique constraint on `(platform, external_id)`

### `problem_sets`

- `id`: primary key, normally `set_...`
- `owner_id`: user foreign key with cascade delete
- `name`, `description`
- `visibility`: `PUBLIC`, `UNLISTED`, or `PRIVATE`; unlisted sets are available by direct ID but omitted from public listings
- `created_at`, `updated_at`

### `problem_set_items`

Join table between sets and problems. It stores `position` and has a composite primary key `(problem_set_id, problem_id)`.

### `contests`

- `id`: primary key, normally `con_...`
- `owner_id`: contest owner
- `name`, `description`
- `start_at`, `end_at`
- `visibility`: the web/contracts use `PUBLIC`, `UNLISTED`, or `PRIVATE`; list queries expose only public contests to unrelated users, while an unlisted contest remains available by direct ID
- `scoring_type`: `ICPC` or `SIMPLE`
- `publication_status`: `DRAFT` or `PUBLISHED`
- `created_at`, `updated_at`

### `contest_problems`

Contest snapshot table. It stores `contest_id`, `problem_id`, `position`, optional `points`, and a label such as `A`, `B`, or `P27`. Contest reads use this table rather than dynamically reading the source problem set.

### `contest_participants`

Join table between contests and users. It stores `joined_at` and has a composite primary key `(contest_id, user_id)`.

### `submissions`

- `id`: primary key, normally `sub_...`
- `user_id`, `problem_id`, optional `contest_id`
- `platform`, `language`, `source_code`
- optional `source_hash`: SHA-256 digest used to prevent the same user from submitting identical source in the same language for the same problem
- `status`: normally `PENDING`, `DISPATCHING`, `JUDGING`, `ACCEPTED`, `WRONG_ANSWER`, `TIME_LIMIT`, `MEMORY_LIMIT`, `RUNTIME_ERROR`, `COMPILE_ERROR`, or `FAILED`
- optional `external_submission_id`
- `submitted_at`, optional `judged_at`
- `metadata`: JSONB for errors, execution time, memory, failed testcase, and raw platform data

A partial unique index on `(user_id, problem_id, language, source_hash)` enforces duplicate prevention for rows that have a digest. The service also compares exact source text so legacy rows created before `source_hash` was added are covered.

### `integrations`

Stores `(user_id, platform)`, `external_username`, `connection_status`, and `updated_at`. This table is an account-linking record only; it does not contain passwords, cookies, or platform tokens.

## Relationship behavior

- Deleting a user cascades to their sets, contests, participants, submissions, and integrations.
- Deleting a contest cascades to its problem snapshots and participants.
- Deleting a problem is rejected while it is used by a contest; set references are removed before the problem is deleted.
- Deleting a contest sets `submissions.contest_id` to `NULL`.
