# CP Hub — Competitive Programming Hub

Unified web platform for **Codeforces** and **AtCoder**.

---

## Features

- 👤 **Single Unified Account**: Browse, practice, and compete with one unified profile.
- 🌐 **Multi-Platform Problem Ingestion**: Paste any URL from Codeforces or AtCoder to normalize and import problem metadata and statement text.
- 📚 **Reusable Problem Sets**: Curate training sets, reorder problems, and share collections.
- 🏆 **Virtual Contests & ICPC Scoring**: Host virtual contests with snapshot problem isolation, automatic problem reveal at start time, and accurate ICPC penalty calculations.
- 💻 **Monaco Code Editor & KaTeX Reader**: Integrated editor supporting C++23, Python 3, Java 21, Go, and Rust with file upload and auto-language detection.
- 🛡️ **Zero-Cookie Browser Extension Bridge**: Uses local browser sessions to dispatch submissions safely without sending passwords or cookies to the server.

---

## Quickstart

```bash
# 1. Start PostgreSQL
docker-compose up -d

# 2. Run Go Backend API
cd apps/api
go run cmd/server/main.go

# 3. In another terminal, run Web Frontend
pnpm --filter @cp-hub/web dev
```

Visit `http://localhost:3000`.

---

## Documentation & Contributing

- 📖 **[System Architecture](docs/architecture.md)** — High-level architecture and platform philosophy
- 🗄️ **[Database Schema](docs/database.md)** — PostgreSQL tables and relation designs
- 🔌 **[Platform Adapters](docs/platform-adapters.md)** — Implementing new competitive programming platforms
- 🧩 **[Extension Protocol](docs/extension-protocol.md)** — Client-side zero-cookie submission bridge protocol
- 🤝 **[Contributing Guide](CONTRIBUTING.md)** — Development workflow, testing, and PR guidelines
- 🤖 **[Agent Handbook](AGENTS.md)** — Invariants, conventions, and operational guide for AI agents and developers

