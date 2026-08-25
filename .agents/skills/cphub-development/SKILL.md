---
name: cphub-development
description: >-
  Comprehensive guide and step-by-step procedures for implementing features, platform adapters,
  database migrations, contest scoring logic, web components, and extension bridges in cpbridge.
  Use this skill whenever building, refactoring, or verifying features in this codebase.
---

# cpbridge Development & Implementation Skill

This skill provides step-by-step procedures, architectural patterns, and validation checklists for implementing features across the cpbridge monorepo.

---

## 1. Monorepo Overview

| Path | Tech Stack | Role & Responsibility |
| :--- | :--- | :--- |
| `apps/api` | Go 1.26+, Chi v5, `pq` | Modular monolith REST API (auth, problems, problem sets, contests, submissions, integrations, platforms). |
| `apps/web` | SvelteKit 2, Svelte 5 runes, Tailwind CSS v4, Monaco Editor | Responsive web client with Monaco editor, KaTeX statement renderer, and live standings. |
| `apps/extension` | Manifest V3 Chrome Extension, TypeScript, Vite | Client-side zero-cookie submission bridge using browser sessions. |
| `packages/contracts` | TypeScript | Shared schemas, DTOs, and extension bridge protocol definitions. |
| `migrations/` | PostgreSQL SQL | DDL migration scripts (`.up.sql` and `.down.sql`). |

---

## 2. Invariants Checklist (Must Uphold)

When writing or modifying code, always verify:
- [ ] **Zero External Cookies Stored**: Backend never receives or stores external passwords, session cookies, or platform tokens.
- [ ] **Authoritative UTC Clocks**: Never use client clocks. Contest detail/problem reads prefer PostgreSQL `NOW()`; list and submission paths use injectable UTC service clocks.
- [ ] **Deep Contest Snapshotting**: When creating a contest from a problem set, snapshot problem IDs, positions, and labels (`A`, `B`, `C`, ...) into `contest_problems`.
- [ ] **Prefixed Entity IDs**: `usr_` (user), `prb_` (problem), `set_` (problem set), `con_` (contest), `sub_` (submission) via `internal/idgen`.
- [ ] **Official Link Fallbacks**: Problem metadata and statement fallbacks expose the official problem URL. Admin submission responses may additionally expose the official external submission URL.
- [ ] **Thin HTTP Handlers**: Handlers only parse input, call domain services, and return JSON responses.

---

## 3. Step-by-Step Implementation Workflows

### Workflow A: Adding a New Platform Adapter

1. **Create Adapter Package**:
   Create `apps/api/internal/platform/<platform_name>/<platform_name>.go` and implement the `platform.Platform` interface:
   ```go
   package myplatform

   import (
       "context"
       "github.com/cpbridge/api/internal/platform"
   )

   type Platform struct{}

   func New() *Platform { return &Platform{} }

   func (p *Platform) Type() platform.Type { return platform.Type("MYPLATFORM") }
   func (p *Platform) MatchURL(rawURL string) (string, bool) { /* return externalId and whether it matched */ }
   func (p *Platform) GetProblem(ctx context.Context, externalID string) (*platform.NormalizedProblem, error) { /* fetch metadata */ }
   func (p *Platform) GetStatement(ctx context.Context, externalID string) (*platform.ProblemStatement, error) { /* fetch statement */ }
   func (p *Platform) GetSubmission(ctx context.Context, externalSubmissionID string) (*platform.SubmissionStatus, error) { /* poll verdict */ }
   ```

2. **Register Platform**:
   In `apps/api/cmd/server/main.go`:
   ```go
   platRegistry.Register(myplatform.New())
   ```

   Go adapters do not submit source code. If the new platform supports browser-side submissions, update the shared protocol and extension separately. Current language IDs are `cpp23`, `python3`, and `java21`.

3. **Update Contracts**:
   In `packages/contracts/src/problem.ts`:
   ```typescript
   export type PlatformType = 'CODEFORCES' | 'ATCODER' | 'LEETCODE' | 'MYPLATFORM';
   ```

4. **Add Unit Tests**:
   Add test cases in `apps/api/internal/platform/platform_test.go` covering URL matching and problem ID parsing.

---

### Workflow B: Modifying Database Schema

1. **Create Migration Files**:
   Create sequential migration files in `migrations/`:
   - `migrations/00000X_feature_name.up.sql`
   - `migrations/00000X_feature_name.down.sql`

2. **Update In-Memory `EnsureSchema`**:
   In `apps/api/internal/db/db.go`, add idempotent DDL (`ALTER TABLE ... ADD COLUMN IF NOT EXISTS ...` or `CREATE TABLE IF NOT EXISTS ...`) to `EnsureSchema` for smooth local development.

3. **Security Invariant**:
   Always use parameterized queries (`$1`, `$2`, ...) in database stores and services. Never concatenate raw SQL strings.

---

### Workflow C: Contest & ICPC Scoring Rules

When implementing contest or scoreboard features:
1. **Contest States**:
   - `UPCOMING`: `now < start_at`. Contest-scoped problem reads return `CONTEST_NOT_STARTED` to non-owner/non-admin users.
   - `ACTIVE`: `start_at <= now < end_at`.
   - `FINISHED`: `now >= end_at`. Standings include only submissions with `start_at <= submitted_at < end_at`.
2. **ICPC Penalty Scoring**:
   - Penalty time for problem \(i\) solved at time \(T_i\) (minutes from contest start) with \(W_i\) earlier recorded submissions prior to first AC:
     $$\text{Penalty}_i = T_i + 20 \times W_i$$
   - Total score = Number of solved problems.
   - Tie-breaker = Lowest total penalty time.
   - The current implementation counts every earlier non-accepted record toward \(W_i\), not only wrong-answer verdicts. Submissions after first AC do not increase penalty.
   - Unsolved problems do not contribute to penalty time.

---

### Workflow D: Frontend Component & Page Development

1. **Svelte 5 Runes**:
   - State: `let count = $state(0)`
   - Derived: `let doubled = $derived(count * 2)`
   - Props: `let { problem, contest } = $props<{ problem: Problem, contest?: Contest }>()`
   - Effects: `$effect(() => { ... })`
2. **Client-Side Monaco Editor & KaTeX**:
   - Monaco must be dynamically imported on the client side (`onMount` or dynamic `import()`).
   - Use KaTeX for rendering mathematical formulas in problem statements (`$ ... $` or `$$ ... $$`).
3. **Contracts Usage**:
   - Import all types from `@cpbridge/contracts`.

---

## 4. Full Verification & Quality Loop (Mandatory)

After **any** development or code change (even if only touching backend/API files in `apps/api`), agents **must** run the complete verification suite including both backend and frontend tests.

### Zero Errors & Zero Warnings Policy
- **No unresolved errors or warnings**: Code must pass all linter checks, type diagnostics, test suites, and builds cleanly with **0 errors and 0 warnings**.
- **Iterative Fix Loop**: If any error or warning is reported at any stage (linter, TypeScript compiler, svelte-check, Go compiler/tests, Playwright tests, or Vite bundler), fix the underlying issue immediately and re-run the full verification suite from the start. Repeat this loop until the entire suite passes with zero errors and zero warnings.

### Verification Suite Commands

```bash
# 1. Full monorepo linting (must have 0 errors and 0 warnings)
rtk pnpm lint

# 2. Full monorepo type checking (contracts, web, extension)
rtk pnpm check

# 3. Go backend unit tests
cd apps/api && go test -v ./...

# 4. Frontend web tests (MANDATORY even when only modifying apps/api)
rtk pnpm --filter @cpbridge/web test

# 5. Build Extension & Web client
rtk pnpm --filter @cpbridge/extension build
rtk pnpm --filter @cpbridge/web build
```

