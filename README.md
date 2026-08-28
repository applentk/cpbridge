# cpbridge

A unified competitive programming platform for Codeforces and AtCoder.

`cpbridge` provides one workspace for administrators to curate problems and problem sets, run virtual contests with ICPC scoring, and for participants to write solutions and submit them to external judges through their existing browser sessions.

---

## Features

- **Multi-Platform Problems**: Administrators import and manage normalized Codeforces and AtCoder problems with official statement links and parsed sample cases.
- **Admin-Managed Problem Sets**: Administrators create and reorder curated collections used to assemble virtual contests.
- **Virtual Contests**: Run timed contests with snapshotted problem lists, controlled problem reveal, participant tracking, and live standings.
- **ICPC Scoring**: Rank participants by solved problems and penalty time using submissions recorded during the contest window.
- **Integrated Code Editor**: Write C++23, Python 3, or Java 21 solutions in Monaco Editor with starter templates and file upload detection.
- **Browser-Based Submissions**: Submit directly to Codeforces and AtCoder through a Manifest V3 extension using the user's active platform sessions.
- **Recoverable Submission Flow**: Preserve external submission handoffs across page reloads without sending platform passwords or cookies to the backend.
- **Administration Dashboard**: Manage users, problems, problem sets, contests, publication state, and account access from the web interface.

---

## Architecture

The project is organized as a monorepo with a Go API, SvelteKit web application, browser extension, shared TypeScript contracts, PostgreSQL, and Redis:

```text
cpbridge/
├── apps/
│   ├── api/                    # Go REST API, domain services, platform adapters, and verdict worker
│   ├── web/                    # SvelteKit dashboard, problem reader, editor, contests, and standings
│   └── extension/              # Chrome MV3 bridge for Codeforces and AtCoder submissions
├── packages/
│   └── contracts/              # Shared TypeScript DTOs and extension protocol types
├── migrations/                 # PostgreSQL schema migrations
├── docs/                       # Architecture, database, adapter, and extension documentation
└── docker-compose.yml          # Local PostgreSQL 16 and Redis 7 services
```

The backend stores application accounts, contest data, source code, and verdicts. External judge credentials and session cookies remain in the browser and are never sent to the cpbridge server.

---

## Tech Stack

- **Backend**: Go 1.26+ with Chi v5, PostgreSQL driver `pq`, JWT authentication, and a modular domain-service architecture
- **Frontend**: SvelteKit 2, Svelte 5, Tailwind CSS v4, Monaco Editor, and KaTeX
- **Database**: PostgreSQL 16 with SQL migrations and idempotent local schema initialization
- **Task Queue**: Redis 7 with Asynq for asynchronous external-verdict polling
- **Browser Extension**: Manifest V3, TypeScript, and Vite
- **Shared Contracts**: TypeScript workspace package consumed by the web app and extension

---

## Documentation

- [System Architecture](docs/architecture.md): Runtime topology, domain boundaries, contest timing, scoring, and submission flow.
- [Database Schema](docs/database.md): Tables, relationships, indexes, and deletion behavior.
- [Platform Adapters](docs/platform-adapters.md): Codeforces and AtCoder normalization, statements, verdicts, and adapter development.
- [Extension Protocol](docs/extension-protocol.md): Browser bridge messages, recovery flow, permissions, and submission lifecycle.
- [Contributing Guide](CONTRIBUTE.md): Local setup, development workflow, verification, and pull request conventions.
- [Agent Handbook](AGENTS.md): Architecture invariants and repository-specific development rules.

---

## Getting Started

### Prerequisites

- [Go 1.26+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/)
- [pnpm 9+](https://pnpm.io/installation)
- [Docker and Docker Compose](https://www.docker.com/)
- Google Chrome, Chromium, Brave, or Microsoft Edge for extension-based submissions

### Local Development

1. **Clone the repository:**

   ```bash
   git clone https://github.com/applentk/cpbridge.git
   cd cpbridge
   ```

2. **Install workspace dependencies:**

   ```bash
   pnpm install
   ```

3. **Start PostgreSQL and Redis:**

   ```bash
   docker-compose up -d
   ```

4. **Run the API:**

   ```bash
   pnpm dev:api
   ```

5. **Run the web application in another terminal:**

   ```bash
   pnpm dev:web
   ```

Open [http://localhost:3000](http://localhost:3000).

### Browser Extension

Build and package the extension:

```bash
pnpm --filter @cpbridge/extension build
```

For local development, run the watch build in another terminal:

```bash
pnpm dev:ext
```

Then load `apps/extension/.dev` as the unpacked extension in `chrome://extensions`. The development build includes the localhost origins; the packaged production build does not. Sign in to Codeforces or AtCoder in the same browser before submitting.

For a packaged build, extract `apps/extension/cpbridge-extension.zip` and load the extracted directory.

### Demo Data

Create an idempotent local demo account, problem set, and active contest with five Codeforces and five AtCoder problems:

```bash
pnpm seed:demo
```

The default account is `demo@cpbridge.local` with password `demo1234`. The seed command refuses to run when `ENV` or `NODE_ENV` is set to `production`.
