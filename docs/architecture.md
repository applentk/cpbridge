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

Problems are normalized into the `problems` table. Imports are keyed by `(platform, external_id)` and update an existing row when re-imported. Importing stores problem metadata, not a scraped statement snapshot. Statements are read from custom problem metadata when present or fetched from the registered platform adapter when requested.

Administrators can also import a public, revealed external contest as a new problem set through the optional platform `ContestProvider` capability. Codeforces regular contests use the public API, Codeforces Gyms use their public dashboard, and AtCoder contests use their public tasks page. The problem-set domain then upserts all normalized problems and inserts the ordered memberships in a single PostgreSQL transaction. Re-importing reuses existing problem records but creates a separate set so administrators can customize multiple collections independently.

Contest problem lists are blocked for non-owners/non-admins while a contest is upcoming. `GetStatement` has an additional upcoming-contest check, but it only blocks stored statements whose platform value is `CUSTOM` or empty. The public direct problem endpoint still returns stored metadata, and external-platform statements remain directly addressable. The code therefore does not provide universal secrecy through every direct problem route; callers must treat the contest problem endpoint as the authoritative reveal gate.

## Contest timing and scoring

Contest states are `UPCOMING`, `ACTIVE`, or `FINISHED`, with the active interval defined as `start_at <= now < end_at`. `GetByID` and `GetProblems` prefer PostgreSQL `SELECT NOW()` when determining state. Some list and submission-validation paths use the service clock (`time.Now().UTC()`), so timing is server-side but not uniformly database-clock-based.

Creating a contest from a problem set copies problem IDs, positions, and generated labels into `contest_problems`. Active and finished contests cannot have their start time or problem list changed.

Standings are calculated from contest participants and submissions whose verified external timestamp (falling back to `submitted_at` for legacy rows) falls within the half-open contest window. ICPC scoring ranks by solved count first and total penalty second; the current implementation adds 20 minutes for every earlier recorded submission to that problem before the first acceptance, regardless of its stored non-accepted status. `SIMPLE` uses solved count with total elapsed solve time as the tie-breaker. The submission service blocks contest submissions before the start and auto-joins users during an active contest. Contest-linked external submissions must also pass the short dispatch window and the contest window before they can be attached.

## Supported submission languages

The API, shared contracts, web editor, and extension currently accept only:

- `cpp23` — C++23 (GCC)
- `python3` — Python 3
- `java21` — Java 21

The extension reads the target platform's current compiler options from its submit form and uses hard-coded compiler IDs only as fallbacks. Depending on what a particular contest offers, the matching logic can fall back to the closest C++20, Java 17, or Python/PyPy 3 option.

## Submission flow

1. The API validates the language and contest context, rejects an exact duplicate source/language/problem submission, and creates a `PENDING` row containing the user, problem, language, source code, and optional contest.
2. The web app sends the submission to the extension through `window.postMessage`. It retries a timed-out bridge request using the same cpbridge submission ID.
3. The extension deduplicates in-flight requests by that ID, stores only the handoff state in `chrome.storage.local`, and submits to Codeforces or AtCoder with the user's existing browser session.
4. The extension identifies the new external submission from the authenticated user's submission list. It refuses to guess if the new ID is ambiguous.
5. The web app sends the external ID to `/api/submissions/{id}/dispatched`; the API changes the row to `JUDGING` and enqueues an Asynq polling task.
6. The worker and normal API reads use the Go adapter to synchronize verdicts. The web page can additionally ask the extension to poll while the submission is active.
7. If a reload interrupts the handoff, the web app recovers the stored extension result, finishes the API update, and acknowledges the record so the extension can remove it.

Terminal verdict and platform metadata are stored in the submission row. The backend stores source code, but never receives external platform cookies or passwords. An official external submission-page URL is added to submission responses only for admins.

## Important source locations

- API bootstrap: `apps/api/cmd/server/main.go`
- Schema bootstrap: `apps/api/internal/db/db.go`
- Domain services: `apps/api/internal/{auth,problem,problemset,contest,submission,integration}`
- Web API client: `apps/web/src/lib/api/client.ts`
- Problem solving page: `apps/web/src/routes/problems/[id]/+page.svelte`
- Extension bridge: `apps/extension/src/bridge.ts`
- Extension worker: `apps/extension/src/background.ts`
- Shared protocol/types: `packages/contracts/src`
