# cpbridge — Competitive Programming Bridge

Unified web platform for **Codeforces** and **AtCoder**.

---

## Features

- **Single Unified Account**: Browse, practice, and compete with one unified profile.
- **Multi-Platform Problem Ingestion**: Paste a Codeforces or AtCoder problem URL to normalize and import its metadata; statements are loaded from stored custom content or fetched from the platform adapter when opened.
- **Reusable Problem Sets**: Curate training sets, reorder problems, and share collections.
- **Virtual Contests & ICPC Scoring**: Host virtual contests with snapshot problem isolation, automatic problem reveal at start time, and accurate ICPC penalty calculations.
- **Monaco Code Editor & KaTeX Reader**: Integrated editor supporting C++23, Python 3, and Java 21 with file upload and auto-language detection.
- **Zero-Cookie Browser Extension Bridge**: Uses local browser sessions to dispatch submissions safely without sending passwords or cookies to the server.

---

## Quickstart

```bash
# 1. Install workspace dependencies
pnpm install

# 2. Start PostgreSQL and Redis
docker-compose up -d

# 3. Run the Go API (includes the Asynq verdict worker)
cd apps/api
go run cmd/server/main.go

# 4. In another terminal, run the web app
pnpm --filter @cpbridge/web dev
```

Visit `http://localhost:3000`.

### Build and install the extension

The web app can browse without the extension, but submitting to Codeforces or AtCoder requires the built extension and an active login on the target platform:

```bash
pnpm --filter @cpbridge/extension build
```

The build writes `dist/background.js` and `dist/bridge.js`, then packages `cpbridge-extension.zip` both in `apps/extension/` and in the web app's downloads directory. Either load `apps/extension` as an unpacked extension, or extract the ZIP and load the extracted folder in a Chromium browser.

The extension uses the active Codeforces and AtCoder browser sessions. Sign in to the target platform before submitting. The supported submission languages are C++23 (GCC), Python 3, and Java 21; the extension reads the current compiler options from each platform's submit form and uses version-specific IDs only as fallbacks.

### Seed a local test contest

After PostgreSQL is running, populate the local database with 10 official problems (5 Codeforces and 5 AtCoder), a problem set, and an active ICPC contest:

```bash
pnpm seed:demo
```

The command is idempotent and creates the local test account `demo@cphub.local` with password `demo1234`. Override `SEED_DEMO_EMAIL`, `SEED_DEMO_USERNAME`, and `SEED_DEMO_PASSWORD` when needed. It refuses to run when `ENV` or `NODE_ENV` is production.

---

## Documentation & Contributing

- 📖 **[System Architecture](docs/architecture.md)** — Current runtime topology and submission flow
- 🗄️ **[Database Schema](docs/database.md)** — PostgreSQL tables and relation designs
- 🔌 **[Platform Adapters](docs/platform-adapters.md)** — Implementing new competitive programming platforms
- 🧩 **[Extension Protocol](docs/extension-protocol.md)** — Client-side zero-cookie submission bridge protocol
- 🤝 **[Contributing Guide](CONTRIBUTE.md)** — Development workflow, testing, and PR guidelines
- 🤖 **[Agent Handbook](AGENTS.md)** — Invariants, conventions, and operational guide for AI agents and developers
