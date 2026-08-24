# Contributing to cpbridge

Welcome to the **cpbridge** repository! We're thrilled that you are interested in contributing. Whether you are fixing a bug, adding a new competitive programming platform adapter, refining UI components, or improving documentation, your contributions are welcome.

This guide provides everything you need to know to set up your environment, understand our architecture, develop new features, and submit your pull requests.

---

## Table of Contents

1. [Code of Conduct & Philosophy](#code-of-conduct--philosophy)
2. [Prerequisites & Development Environment](#prerequisites--development-environment)
3. [Local Setup & Quickstart](#local-setup--quickstart)
4. [Monorepo Structure](#monorepo-structure)
5. [Core Engineering Invariants](#core-engineering-invariants)
6. [Development Guides](#development-guides)
   - [Adding a Platform Adapter](#adding-a-platform-adapter)
   - [Database Migrations](#database-migrations)
   - [Updating Shared Contracts](#updating-shared-contracts)
   - [Frontend Development (Svelte 5)](#frontend-development-svelte-5)
   - [Extension Development (Manifest V3)](#extension-development-manifest-v3)
7. [Testing & Quality Assurance](#testing--quality-assurance)
8. [Git & Pull Request Guidelines](#git--pull-request-guidelines)

---

## Code of Conduct & Philosophy

- **Respectful Collaboration**: Treat everyone with respect, kindness, and professionalism.
- **Privacy First (Zero External Cookies on Server)**: We never store external platform credentials or cookies on our servers.
- **Fair Competition & Server Authority**: All contest timings and score calculations must be strictly enforced server-side in UTC.
- **Graceful Fallbacks**: If automated scraping or extension bridges fail, always provide direct official platform links.

---

## Prerequisites & Development Environment

Before setting up the repository, ensure you have the following installed:

- **Go**: `1.26+` ([Download Go](https://go.dev/dl/))
- **Node.js**: `v20.x` or higher ([Download Node.js](https://nodejs.org/))
- **pnpm**: `v9.x` or higher (`corepack enable` or `npm install -g pnpm`)
- **Docker & Docker Compose**: For running PostgreSQL 16 ([Download Docker](https://www.docker.com/))
- **Google Chrome / Chromium**: For testing the browser extension

---

## Local Setup & Quickstart

### 1. Clone the Repository

```bash
git clone https://github.com/cpbridge/apple-icpc.git
cd apple-icpc
```

### 2. Install JavaScript/TypeScript Dependencies

```bash
pnpm install
```

### 3. Start PostgreSQL Database

Start the PostgreSQL 16 container via Docker Compose:

```bash
docker-compose up -d
```

PostgreSQL will be available on `localhost:5432` with:
- **Database**: `cphub_db`
- **Username**: `cphub`
- **Password**: `cphub_password`

### 4. Run the Go API Server

In a new terminal:

```bash
# Using root package script:
pnpm dev:api

# Or directly in the api directory:
cd apps/api
go run cmd/server/main.go
```

The API server runs on `http://localhost:8080`.

### 5. Run the SvelteKit Web Application

In another terminal:

```bash
pnpm dev:web
```

Open your browser and navigate to `http://localhost:3000`.

### 6. (Optional) Build & Load the Browser Extension

If you want to test zero-cookie submissions directly in your browser:

```bash
pnpm dev:ext
```

Then in Google Chrome:
1. Navigate to `chrome://extensions`.
2. Toggle on **Developer mode** (top right).
3. Click **Load unpacked** and select the `apps/extension/` directory.

---

## Monorepo Structure

```text
apple-icpc/
├── apps/
│   ├── api/             # Go 1.26+ modular monolith backend REST API
│   │   ├── cmd/server/  # API entrypoint and route mounting
│   │   └── internal/    # Domain services (auth, contest, problem, submission, platform)
│   ├── web/             # SvelteKit 2 + Svelte 5 + Tailwind CSS v4 web application
│   └── extension/       # Manifest V3 browser extension for client-side submission bridge
├── packages/
│   └── contracts/       # Shared TypeScript schemas, DTOs, and protocol definitions
├── migrations/          # PostgreSQL DDL migrations
├── docs/                # Architectural, database, and protocol specifications
├── docker-compose.yml   # Local development PostgreSQL 16 container definition
├── package.json         # Monorepo root scripts and dev tooling
└── AGENTS.md            # Guidelines and invariants for AI agents and contributors
```

---

## Core Engineering Invariants

When working on any part of cpbridge, always ensure these rules are preserved:

1. **Zero External Credentials on Backend**: Never store user external platform passwords, API secrets, or session cookies in the database.
2. **Strict Server-Side UTC Contest Timing**: All contest status transitions (`UPCOMING` -> `ACTIVE` -> `FINISHED`), problem statement visibility, and ICPC penalty calculations are determined strictly by server UTC timestamps (`NOW()`).
3. **Deep Contest Problem Snapshotting**: When creating a contest from a problem set, problems and labels are snapshotted into `contest_problems` so subsequent modifications to problem sets do not alter active/past contests.
4. **Prefixed Entity IDs**: All entity IDs must use consistent domain prefixes:
   - `usr_...` — Users
   - `prb_...` — Problems
   - `set_...` — Problem Sets
   - `con_...` — Contests
   - `sub_...` — Submissions

---

## Development Guides

### Adding a Platform Adapter

To support a new competitive programming platform (e.g., LeetCode, CSES, SPOJ):

1. Create a new package under `apps/api/internal/platform/<platform_name>/`.
2. Implement the `platform.Platform` interface defined in `apps/api/internal/platform/platform.go`:
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
3. Register the new adapter in `apps/api/cmd/server/main.go`:
   ```go
   platRegistry.Register(newplatform.New())
   ```
4. Add unit tests for URL matching and problem ID parsing in `apps/api/internal/platform/platform_test.go`.

### Database Migrations

- Add sequential migration files in `migrations/` (e.g., `000003_add_xyz.up.sql` and `000003_add_xyz.down.sql`).
- Update `EnsureSchema` in `apps/api/internal/db/db.go` so fresh local environments automatically initialize the changes.
- Always use parameterized queries (`$1`, `$2`, ...) to protect against SQL injection.

### Updating Shared Contracts

When adding new DTOs or modifying payload schemas:
1. Update TypeScript definitions in `packages/contracts/src/index.ts`.
2. Verify contracts typecheck:
   ```bash
   pnpm --filter @cpbridge/contracts check
   ```
3. Ensure corresponding Go structs in `apps/api/internal/...` match the JSON fields.

### Frontend Development (Svelte 5)

- We use **Svelte 5 runes** (`$state`, `$derived`, `$props`, `$effect`).
- Styling uses **Tailwind CSS v4**.
- Monaco Editor should be dynamically loaded on the client side to avoid SSR errors.
- Always run `pnpm --filter @cpbridge/web check` to verify TypeScript and template correctness.

### Extension Development (Manifest V3)

- The browser extension communicates with the web app using `window.postMessage` and the bridge script (`src/bridge.ts`).
- Ensure no persistent secret storage is used in the extension.

---

## Testing & Quality Assurance

Before submitting a Pull Request, run the complete verification suite locally:

```bash
# 1. Run all Go backend unit tests
cd apps/api && go test -v ./...

# 2. Check TypeScript contracts
pnpm --filter @cpbridge/contracts check

# 3. Build Extension
pnpm --filter @cpbridge/extension build

# 4. Check & build Frontend Web
pnpm --filter @cpbridge/web check
pnpm --filter @cpbridge/web build
```

---

## Git & Pull Request Guidelines

### 1. Branch Naming

Use clear branch prefixes:
- `feat/feature-name` — for new features
- `fix/bug-description` — for bug fixes
- `docs/doc-update` — for documentation improvements
- `refactor/scope` — for code refactorings

### 2. Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat: add CSES platform adapter`
- `fix: handle edge case in ICPC penalty calculation`
- `docs: update platform adapter documentation`
- `test: add unit tests for contest state transition`

### 3. Pull Request Checklist

When opening a PR:
- [ ] Ensure all Go backend tests pass (`go test -v ./...`).
- [ ] Ensure `pnpm check` and `pnpm build` pass with 0 errors.
- [ ] Provide a concise description of the changes and motivation.
- [ ] If changing database schema, include both `up` and `down` SQL migrations.
- [ ] If changing UI, include screenshots or GIFs where applicable.

---

Thank you for helping make cpbridge better for everyone! 🚀
