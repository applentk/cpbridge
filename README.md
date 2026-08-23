# CP Hub — Competitive Programming Hub

Unified web platform for **Codeforces**, **AtCoder**, and **LeetCode**.

---

## Features

- 👤 **Single Unified Account**: Browse, practice, and compete without needing 3 different accounts.
- 🌐 **Multi-Platform Problem Ingestion**: Paste any URL from Codeforces, AtCoder, or LeetCode to normalize and import problem metadata.
- 📚 **Reusable Problem Sets**: Curate training sets, reorder problems, and share collections.
- 🏆 **Virtual Contests & ICPC Scoring**: Host virtual contests with snapshot problem isolation, automatic problem reveal at start time, and accurate ICPC penalty calculations.
- 💻 **Monaco Code Editor**: Integrated editor supporting C++23, Python 3, Java 21, Go, and Rust.
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
