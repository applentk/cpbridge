---
name: cphub-development
description: >-
  Comprehensive guide and step-by-step procedures for implementing features, platform adapters,
  database migrations, contest scoring logic, web components, and extension bridges in CP Hub.
  Use this skill whenever building, refactoring, or verifying features in this codebase.
---

# CP Hub Development & Implementation Skill

This skill provides step-by-step procedures, architectural patterns, and validation checklists for implementing features across the CP Hub monorepo.

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
- [ ] **Authoritative UTC Clocks**: Contest start, end, remaining time, problem statement reveals, and submission validity are determined strictly by PostgreSQL `NOW()` in UTC.
- [ ] **Deep Contest Snapshotting**: When creating a contest from a problem set, snapshot problem IDs, positions, and labels (`A`, `B`, `C`, ...) into `contest_problems`.
- [ ] **Prefixed Entity IDs**: `usr_` (user), `prb_` (problem), `set_` (problem set), `con_` (contest), `sub_` (submission) via `internal/idgen`.
- [ ] **Official Link Fallbacks**: Every problem metadata object and submission response must include direct official platform statement and submission URLs.
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
       "github.com/cp-hub/api/internal/platform"
   )

   type Platform struct{}

   func New() *Platform { return &Platform{} }

   func (p *Platform) Name() string { return "MYPLATFORM" }
   func (p *Platform) MatchesURL(rawURL string) bool { /* check regex or domain */ }
   func (p *Platform) ParseProblemID(rawURL string) (string, error) { /* return unique externalId */ }
   func (p *Platform) FetchProblem(ctx context.Context, externalID string) (*platform.ProblemMetadata, error) { /* scrape/fetch */ }
   func (p *Platform) SupportedLanguages() []string { return []string{"cpp23", "python3", "java21", "go", "rust"} }
   func (p *Platform) BuildSubmissionPayload(req *platform.SubmissionRequest) (*platform.SubmissionPayload, error) { /* build payload */ }
   func (p *Platform) OfficialProblemURL(externalID string) string { /* return URL */ }
   func (p *Platform) OfficialSubmitURL(externalID string) string { /* return submit URL */ }
   ```

2. **Register Platform**:
   In `apps/api/cmd/server/main.go`:
   ```go
   platRegistry.Register(myplatform.New())
   ```

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
   - `UPCOMING`: `NOW() < start_at`. Problem statements are redacted (`statement = ""` or empty in API response).
   - `ACTIVE`: `start_at <= NOW() <= end_at`. Statements and submission submissions are active.
   - `FINISHED`: `NOW() > end_at`. Contest is over, standings finalized.
2. **ICPC Penalty Scoring**:
   - Penalty time for problem \(i\) solved at time \(T_i\) (minutes from contest start) with \(W_i\) failed attempts prior to first AC:
     $$\text{Penalty}_i = T_i + 20 \times W_i$$
   - Total score = Number of solved problems.
   - Tie-breaker = Lowest total penalty time.
   - Submissions after first AC do not increase penalty.
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
   - Import all types from `@cp-hub/contracts`.

---

## 4. Full Verification Suite

Before marking any task as complete, execute this verification run:

```bash
# 1. Run all Go backend unit tests
cd apps/api && go test -v ./...

# 2. Check TypeScript contracts
pnpm --filter @cp-hub/contracts check

# 3. Build Extension
pnpm --filter @cp-hub/extension build

# 4. Check & build Frontend Web
pnpm --filter @cp-hub/web check
pnpm --filter @cp-hub/web build
```
