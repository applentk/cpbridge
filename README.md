# cpbridge — Competitive Programming Bridge

Unified web platform for **Codeforces** and **AtCoder**.

---

## Features

- 👤 **Single Unified Account**: Browse, practice, and compete with one unified profile.
- 🌐 **Multi-Platform Problem Ingestion**: Paste any URL from Codeforces or AtCoder to normalize and import problem metadata and statement text.
- 📚 **Reusable Problem Sets**: Curate training sets, reorder problems, and share collections.
- 🏆 **Virtual Contests & ICPC Scoring**: Host virtual contests with snapshot problem isolation, automatic problem reveal at start time, and accurate ICPC penalty calculations.
- 💻 **Monaco Code Editor & KaTeX Reader**: Integrated editor supporting C++23, Python 3, and Java 21 with file upload and auto-language detection.
- 🛡️ **Zero-Cookie Browser Extension Bridge**: Uses local browser sessions to dispatch submissions safely without sending passwords or cookies to the server.

---

## Quickstart

```bash
# 1. Start PostgreSQL and Redis
docker-compose up -d

# 2. Run Go Backend API
cd apps/api
go run cmd/server/main.go

# 3. In another terminal, run Web Frontend
pnpm --filter @cpbridge/web dev
```

Visit `http://localhost:3000`.

### Build and install the extension

The web app can browse without the extension, but submitting to Codeforces or AtCoder requires the built extension and an active login on the target platform:

```bash
pnpm --filter @cpbridge/extension build
```

Load `apps/extension` as an unpacked extension in Chrome after the build. The manifest points to the generated `dist/background.js` and `dist/bridge.js` files.

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
