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

2. **Server-Authoritative UTC Contest Time**:
   - Never rely on client clocks for contest start, end, remaining time, or submission validation.
   - `contest.GetByID` and `contest.GetProblems` prefer PostgreSQL `NOW()`; list and submission paths currently use injectable UTC service clocks. Keep every path server-side and UTC.
   - Upcoming contest problem lists are withheld from non-owner/non-admin users. Direct problem routes are not a universal secrecy boundary, especially for public external-platform problems, so use contest-scoped reads as the reveal gate.

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
   - Problem entities carry the official `url`, and statement parsing falls back to an official link. The web UI exposes that source link when scraping or extension bridging encounters edge cases.
   - Admin submission responses may include `sourceUrl` for the official external submission page; regular-user responses do not currently expose that field.

6. **Thin HTTP Handlers, Rich Domain Services**:
   - Handlers in `apps/api/internal/*` must only parse parameters, validate inputs, invoke the domain service, and serialize JSON responses.
   - Business logic, validation rules, transaction boundaries, and database queries belong in domain services and stores.

---

## 3. Core Domain Patterns

### Adding / Modifying Platform Adapters
Platform adapters live in `apps/api/internal/platform/` and must implement the `platform.Platform` interface:

```go
type Platform interface {
    Type() Type
    MatchURL(rawURL string) (externalID string, matched bool)
    GetProblem(ctx context.Context, externalID string) (*NormalizedProblem, error)
    GetStatement(ctx context.Context, externalID string) (*ProblemStatement, error)
    GetSubmission(ctx context.Context, externalSubmissionID string) (*SubmissionStatus, error)
}
```
- When adding a new platform (e.g. LeetCode, CSES), register the adapter in `platRegistry.Register(...)` inside `apps/api/cmd/server/main.go`.
- The currently supported submission language IDs are `cpp23`, `python3`, and `java21`. Submission itself remains an extension responsibility; Go platform adapters only normalize problems/statements and read verdicts.

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

After **any** development or code change (even if only touching backend/API files in `apps/api`), run the full verification suite including both backend and frontend tests.

### Zero Errors & Zero Warnings Policy
- **No unresolved errors or warnings**: Code must pass all linter checks, type diagnostics, test suites, and builds cleanly with **0 errors and 0 warnings**.
- **Iterative Fix Loop**: If any error or warning is encountered at any stage, fix the root cause immediately and re-run the full verification suite in a loop until all checks pass cleanly with zero errors and zero warnings.

```bash
# 1. Run full monorepo linting (zero warnings & zero errors)
rtk pnpm lint

# 2. Run full monorepo type checks (contracts, web, extension)
rtk pnpm check

# 3. Run all Go backend unit tests
cd apps/api && go test -v ./...

# 4. Run frontend tests (MANDATORY even when only modifying apps/api)
rtk pnpm --filter @cpbridge/web test

# 5. Build browser extension & frontend web app
rtk pnpm --filter @cpbridge/extension build
rtk pnpm --filter @cpbridge/web build
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

<!-- rtk-instructions v2 -->
# RTK (Rust Token Killer) - Token-Optimized Commands

## Golden Rule

**Always prefix commands with `rtk`**. If RTK has a dedicated filter, it uses it. If not, it passes through unchanged. This means RTK is always safe to use.

**Important**: Even in command chains with `&&`, use `rtk`:
```bash
# ❌ Wrong
git add . && git commit -m "msg" && git push

# ✅ Correct
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## RTK Commands by Workflow

### Build & Compile (80-90% savings)
```bash
rtk cargo build         # Cargo build output
rtk cargo check         # Cargo check output
rtk cargo clippy        # Clippy warnings grouped by file (80%)
rtk tsc                 # TypeScript errors grouped by file/code (83%)
rtk lint                # ESLint/Biome violations grouped (84%)
rtk prettier --check    # Files needing format only (70%)
rtk next build          # Next.js build with route metrics (87%)
```

### Test (60-99% savings)
```bash
rtk cargo test          # Cargo test failures only (90%)
rtk go test             # Go test failures only (90%)
rtk jest                # Jest failures only (99.5%)
rtk vitest              # Vitest failures only (99.5%)
rtk playwright test     # Playwright failures only (94%)
rtk pytest              # Python test failures only (90%)
rtk rake test           # Ruby test failures only (90%)
rtk rspec               # RSpec test failures only (60%)
rtk test <cmd>          # Generic test wrapper - failures only
```

### Git (59-80% savings)
```bash
rtk git status          # Compact status
rtk git log             # Compact log (works with all git flags)
rtk git diff            # Compact diff (80%)
rtk git show            # Compact show (80%)
rtk git add             # Ultra-compact confirmations (59%)
rtk git commit          # Ultra-compact confirmations (59%)
rtk git push            # Ultra-compact confirmations
rtk git pull            # Ultra-compact confirmations
rtk git branch          # Compact branch list
rtk git fetch           # Compact fetch
rtk git stash           # Compact stash
rtk git worktree        # Compact worktree
```

Note: Git passthrough works for ALL subcommands, even those not explicitly listed.

### GitHub (26-87% savings)
```bash
rtk gh pr view <num>    # Compact PR view (87%)
rtk gh pr checks        # Compact PR checks (79%)
rtk gh run list         # Compact workflow runs (82%)
rtk gh issue list       # Compact issue list (80%)
rtk gh api              # Compact API responses (26%)
```

### JavaScript/TypeScript Tooling (70-90% savings)
```bash
rtk pnpm list           # Compact dependency tree (70%)
rtk pnpm outdated       # Compact outdated packages (80%)
rtk pnpm install        # Compact install output (90%)
rtk npm run <script>    # Compact npm script output
rtk npx <cmd>           # Compact npx command output
rtk prisma              # Prisma without ASCII art (88%)
rtk uv run <cmd>        # Compact uv project command output
```

### Files & Search (60-75% savings)
```bash
rtk ls <path>           # Tree format, compact (65%)
rtk read <file>         # Code reading with filtering (60%)
rtk grep <pattern>      # Search grouped by file (75%). Format flags (-c, -l, -L, -o, -Z) run raw.
rtk find <pattern>      # Find grouped by directory (70%)
```

### Analysis & Debug (70-90% savings)
```bash
rtk err <cmd>           # Filter errors only from any command
rtk log <file>          # Deduplicated logs with counts
rtk json <file>         # JSON structure without values
rtk deps                # Dependency overview
rtk env                 # Environment variables compact
rtk summary <cmd>       # Smart summary of command output
rtk diff                # Ultra-compact diffs
```

### Infrastructure (85% savings)
```bash
rtk docker ps           # Compact container list
rtk docker images       # Compact image list
rtk docker logs <c>     # Deduplicated logs
rtk kubectl get         # Compact resource list
rtk kubectl logs        # Deduplicated pod logs
```

### Network (65-70% savings)
```bash
rtk curl <url>          # Compact HTTP responses (70%)
rtk wget <url>          # Compact download output (65%)
```

### Meta Commands
```bash
rtk gain                # View token savings statistics
rtk gain --history      # View command history with savings
rtk discover            # Analyze Claude Code sessions for missed RTK usage
rtk proxy <cmd>         # Run command without filtering (for debugging)
rtk init                # Add RTK instructions to CLAUDE.md
rtk init --global       # Add RTK to ~/.claude/CLAUDE.md
```

## Token Savings Overview

| Category | Commands | Typical Savings |
|----------|----------|-----------------|
| Tests | vitest, playwright, cargo test | 90-99% |
| Build | next, tsc, lint, prettier | 70-87% |
| Git | status, log, diff, add, commit | 59-80% |
| GitHub | gh pr, gh run, gh issue | 26-87% |
| Package Managers | pnpm, npm, npx | 70-90% |
| Files | ls, read, grep, find | 60-75% |
| Infrastructure | docker, kubectl | 85% |
| Network | curl, wget | 65-70% |

Overall average: **60-90% token reduction** on common development operations.
<!-- /rtk-instructions -->