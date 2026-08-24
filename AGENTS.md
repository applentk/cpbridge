# cpbridge — Agent Handbook

Welcome to cpbridge! When developing, extending, or maintaining this codebase, adhere strictly to these architectural guidelines, domain invariants, and code conventions.

---

## 1. Monorepo Organization & Component Map

| Path | Tech Stack | Role & Responsibility |
| :--- | :--- | :--- |
| `apps/api` | Go 1.26+, Chi v5, `pq` | Modular monolith REST API backend for auth, problems, problem sets, contests, standings, and submission logging. |
| `apps/web` | SvelteKit 2, Svelte 5, Tailwind CSS v4, Monaco Editor, KaTeX | Responsive SPA/SSR web client for browsing problems, creating contests, live contest workspace, code editor, and standings. |
| `apps/extension` | Manifest V3 Chrome Extension, TypeScript, Vite | Zero-cookie client-side bridge that executes Codeforces & AtCoder submissions using the user's active browser cookies. |
| `packages/contracts` | TypeScript | Shared type definitions, DTO schemas, and browser extension communication protocol definitions. |
| `migrations/` | PostgreSQL SQL | Database DDL migrations (`.up.sql` and `.down.sql`). |
| `docs/` | Markdown | System architecture, database schema diagrams, extension protocol specifications, and platform adapter docs. |

---

## 2. Critical Engineering Invariants

AI agents modifying this codebase **must** uphold the following invariants without exception:

1. **Zero External Cookies Stored on Server**:
   - **Never** store user external platform passwords, API keys, or session cookies on the backend.
   - Submissions are dispatched from the client side via the browser extension (`apps/extension`) using the user's active session, or via an official fallback link.

2. **Strict Server-Side UTC Contest Time**:
   - Never rely on client clocks for contest start, end, remaining time, or submission validation.
   - Contest states (`UPCOMING`, `ACTIVE`, `FINISHED`) and problem statement reveals are strictly governed by PostgreSQL `NOW()` / UTC timestamps on the server.
   - Problem details and statements for upcoming contests are redacted in API responses until `start_at <= now()`.

3. **Deep Contest Problem Snapshotting**:
   - When a Virtual Contest is created from a Problem Set, always snapshot problem IDs, positions, and assigned labels (`A`, `B`, `C`, ...) into `contest_problems`.
   - Never query `problem_set_items` dynamically during an active contest; modifying a problem set must not alter past or active contests.

4. **Deterministic Prefixed ID Convention**:
   - Always use prefixed IDs generated via `internal/idgen` for all entities:
     - `usr_...` — Users (`internal/auth`)
     - `prb_...` — Problems (`internal/problem`)
     - `set_...` — Problem Sets (`internal/problemset`)
     - `con_...` — Contests (`internal/contest`)
     - `sub_...` — Submissions (`internal/submission`)

5. **Official Submission & Statement Fallback**:
   - Every problem entity and submission response must include direct official platform statement and submission links as fallback options in case scraping or extension bridging encounters edge cases.

6. **Thin HTTP Handlers, Rich Domain Services**:
   - Handlers in `apps/api/internal/*` must only parse parameters, validate inputs, invoke the domain service, and serialize JSON responses.
   - Business logic, validation rules, transaction boundaries, and database queries belong in domain services and stores.

---

## 3. Core Domain Patterns

### Adding / Modifying Platform Adapters
Platform adapters live in `apps/api/internal/platform/` and must implement the `platform.Platform` interface:

```go
type Platform interface {
    Name() string
    MatchesURL(rawURL string) bool
    ParseProblemID(rawURL string) (string, error)
    FetchProblem(ctx context.Context, externalID string) (*ProblemMetadata, error)
    SupportedLanguages() []string
    BuildSubmissionPayload(req *SubmissionRequest) (*SubmissionPayload, error)
    OfficialProblemURL(externalID string) string
    OfficialSubmitURL(externalID string) string
}
```
- When adding a new platform (e.g. LeetCode, CSES), register the adapter in `platRegistry.Register(...)` inside `apps/api/cmd/server/main.go`.

### Database & Schema Migrations
- Standard DDL migrations reside in `migrations/`.
- For developer convenience during local development, `apps/api/internal/db/db.go` contains `EnsureSchema(db)`, which applies initial table creations idempotently (`CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`).
- Always use parameterized queries (`$1`, `$2`, ...) — **never** concatenate raw strings into SQL queries.

### Svelte 5 Web Architecture
- `apps/web` utilizes **Svelte 5 runes** (`$state`, `$derived`, `$props`, `$effect`).
- Monaco Editor is dynamically imported on the client side (`onMount` or dynamic `import()`) to prevent SSR issues.
- Types are imported directly from `@cpbridge/contracts`.

---

## 4. Verification & Testing Playbook

Before finishing any task, run the automated verification suite:

```bash
# 1. Run all Go backend unit tests
cd apps/api && go test -v ./...

# 2. Check TypeScript contracts
pnpm --filter @cpbridge/contracts check

# 3. Build browser extension
pnpm --filter @cpbridge/extension build

# 4. Check & build frontend web app
pnpm --filter @cpbridge/web check
pnpm --filter @cpbridge/web build
```

### Starting the Local Development Stack
```bash
# 1. Start PostgreSQL 16
docker-compose up -d

# 2. Run API server (port 8080)
go run apps/api/cmd/server/main.go
# or: pnpm dev:api

# 3. Run Web client (port 3000)
pnpm --filter @cpbridge/web dev
# or: pnpm dev:web
```
