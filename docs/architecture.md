# CP Hub — System Architecture

Competitive Programming Hub (cp-hub) provides a unified portal for competitive programming across **Codeforces**, **AtCoder**, and **LeetCode**.

---

## High-Level Architecture

```text
[ User Web Browser ] 
       │
       ├── (1) HTTP/REST API ─────────► [ Go Backend API Monolith ] ──► [ PostgreSQL 16 ]
       │                                     │
       │                                     ├── Problem Metadata Scrapers / Adapters
       │                                     └── Server UTC Contest & ICPC Scoring
       │
       └── (2) window.postMessage ───► [ Chrome Extension MV3 ]
                                             │
                                             ├── Codeforces Session Submit (Zero backend cookies)
                                             ├── AtCoder Session Submit (Zero backend cookies)
                                             └── LeetCode Session Submit (Zero backend cookies)
```

---

## Core Principles

1. **Single Application Account**:
   - Users create and manage one account for CP Hub.
   - External platforms (Codeforces, AtCoder, LeetCode) are optional and only needed when submitting solutions.

2. **Zero-Cookie Backend Privacy**:
   - External platform passwords and session cookies are **never** sent to or stored on our backend.
   - The browser extension bridges submissions directly from the client using the user's active browser cookies.

3. **Isolated Platform Adapters**:
   - All platform-specific URL parsing and metadata ingestion are isolated behind the `platform.Platform` Go interface.
   - Core domains (`problemset`, `contest`, `submission`) remain agnostic of external platform internals.

4. **Contest Integrity & Snapshotting**:
   - When a Virtual Contest is created from a Problem Set, problems and ordering are deeply snapshotted into `contest_problems`.
   - Contest states (`UPCOMING`, `ACTIVE`, `FINISHED`) and submissions are strictly enforced by the backend using UTC timestamps.
   - Problem details and titles are redacted in the API for upcoming contests until `start_at <= now()`.
