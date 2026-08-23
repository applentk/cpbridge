# Competitive Programming Hub — Agent Handbook

Welcome to CP Hub! When developing or extending this codebase, adhere to these domain and design guidelines:

---

## 1. Monorepo Organization

- `apps/api`: Go 1.26+ modular monolith.
  - Domain logic is in `internal/` (`auth`, `problem`, `problemset`, `contest`, `submission`, `platform`).
  - Handlers and routes must remain thin and delegate to domain services.
  - IDs are prefixed with `usr_`, `prb_`, `set_`, `con_`, `sub_`.
- `apps/web`: SvelteKit + TypeScript + Tailwind CSS + Monaco Editor.
- `apps/extension`: Manifest V3 browser extension for zero-cookie submission bridging.
- `packages/contracts`: Shared TypeScript interfaces and protocol schemas.
- `migrations/`: SQL migration files for PostgreSQL.

---

## 2. Key Engineering Invariants

- **Zero External Cookies Stored**: Never store user external platform passwords or session cookies on the backend.
- **Strict Server UTC Contest Time**: Never rely on client timestamps for contest start, end, or submission validation.
- **Deep Contest Problem Snapshotting**: Always snapshot problem IDs and labels into `contest_problems` when a contest is created from a Problem Set.
- **Official Submission Fallback**: Every problem and submission failure must expose an official statement and submission link fallback.

---

## 3. Running & Verifying

```bash
# 1. Run Go backend unit tests
cd apps/api && go test -v ./...

# 2. Build Contracts and Extension
pnpm --filter @cp-hub/contracts check
pnpm --filter @cp-hub/extension build

# 3. Build Web App
pnpm --filter @cp-hub/web build

# 4. Start local development stack
docker-compose up -d
go run apps/api/cmd/server/main.go
pnpm --filter @cp-hub/web dev
```
