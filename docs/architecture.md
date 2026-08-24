# cpbridge — Current System Architecture

cpbridge is a unified competitive-programming web application for Codeforces and AtCoder. The current implementation is a small monorepo with a Go API, a SvelteKit web client, a Manifest V3 Chrome extension, shared TypeScript contracts, PostgreSQL, and Redis.

## Runtime topology

```text
                         ┌──────────────────────┐
                         │  SvelteKit web app   │
                         │  :3000               │
                         └──────────┬───────────┘
                                    │ /api + postMessage
                  ┌─────────────────┴───────────────┐
                  │                                 │
                  ▼                                 ▼
        ┌──────────────────┐              ┌────────────────────┐
        │ Go HTTP API      │              │ Chrome MV3 bridge   │
        │ Chi, :8080       │              │ browser sessions    │
        └───────┬──────────┘              └─────────┬──────────┘
                │                                   │
       ┌────────┴────────┐                 ┌────────┴────────┐
       ▼                 ▼                 ▼                 ▼
 PostgreSQL 16     Redis / Asynq     Codeforces         AtCoder
                   verdict worker   cookies/session    cookies/session
```

The server starts the HTTP API and the Asynq worker in the same process. PostgreSQL is used for application data. Redis is used for asynchronous external-verdict polling after a submission has been dispatched.

## Backend structure

The API is a modular Go monolith. `apps/api/cmd/server/main.go` constructs the database connection, calls `db.EnsureSchema`, registers the Codeforces and AtCoder adapters, creates domain services, starts the Asynq worker, and mounts routes below `/api`.

Main domains:

- `auth`: registration, login, JWT authentication, active-account checks, and admin roles.
- `problem`: problem import, custom problem creation, statement loading, filtering, and deletion safeguards.
- `problemset`: owned problem collections with visibility and ordering.
- `contest`: contest creation, publication/visibility rules, participant joins, problem snapshots, and contest state.
- `submission`: submission records, dispatch state, verdict synchronization, and standings.
- `integration`: external usernames and connection status only; it does not store external credentials.
- `admin`: admin-only management endpoints for users, problems, sets, and contests.

HTTP handlers parse requests and serialize responses, while domain services perform validation and SQL operations.

## Authentication and access

The web client sends a bearer JWT from `localStorage` as `Authorization: Bearer ...`. The API rechecks the user and `is_active` flag in PostgreSQL on every authenticated request and refreshes the role from the database.

Public/optional-auth endpoints include problem reads, contest reads, and contest listings. Submission and integration endpoints require authentication. The `/api/admin` route group requires the `ADMIN` role.

## Problems and statements

Problems are normalized into the `problems` table. Imports are keyed by `(platform, external_id)` and update an existing row when re-imported. Statements are either read from custom problem metadata or fetched from the registered platform adapter.

Contest problem lists are blocked for non-owners/non-admins while a contest is upcoming. However, the current code does not universally redact every direct problem or statement endpoint for upcoming external-platform problems; callers should treat the contest problem endpoint as the authoritative reveal gate.

## Contest timing and scoring

Contest states are `UPCOMING`, `ACTIVE`, or `FINISHED`. `GetByID` and `GetProblems` prefer PostgreSQL `SELECT NOW()` when determining state. Some list and submission-validation paths use the service clock (`time.Now().UTC()`), so timing is server-side but not uniformly database-clock-based.

Creating a contest from a problem set copies problem IDs, positions, and generated labels into `contest_problems`. Active and finished contests cannot have their start time or problem list changed.

Standings are calculated from contest participants and submissions whose `submitted_at` falls within the contest window. ICPC scoring ranks by solved count first and total penalty second; each accepted problem receives elapsed minutes plus 20 minutes per rejected attempt before acceptance.

## Submission flow

1. The API creates a `PENDING` submission containing the user, problem, language, source code, and optional contest.
2. The web app sends the submission to the extension through `window.postMessage`.
3. The extension submits to Codeforces or AtCoder with the user's existing browser session.
4. The web app sends the returned external submission ID to the API, which changes the row to `JUDGING` and enqueues an Asynq polling task.
5. The backend adapter and the extension may both poll for the verdict. Terminal status and platform metadata are stored in the submission row.

The backend stores source code and verdict metadata, but never receives external platform cookies or passwords.

## Important source locations

- API bootstrap: `apps/api/cmd/server/main.go`
- Schema bootstrap: `apps/api/internal/db/db.go`
- Domain services: `apps/api/internal/{auth,problem,problemset,contest,submission,integration}`
- Web API client: `apps/web/src/lib/api/client.ts`
- Problem solving page: `apps/web/src/routes/problems/[id]/+page.svelte`
- Extension bridge: `apps/extension/src/bridge.ts`
- Extension worker: `apps/extension/src/background.ts`
- Shared protocol/types: `packages/contracts/src`
