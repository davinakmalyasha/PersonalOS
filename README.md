# Personal OS

Local-first personal platform — one SQLite backbone, a Go API, a Next.js dashboard, and MCP tooling so **any agent** can read/write your life data on your machine.

> Status: Phase 4 (Knowledge) live — Finance, Planner, Knowledge pillars + universal capture core. See `docs/roadmap.md`.

## Pillars

| Pillar | What lives there |
|---|---|
| **Finance** | accounts, transactions, categories/rules, budgets. CSV import + dedupe |
| **Planner** | tasks + habits (streaks) + calendar events (RRULE-lite) — LIVE |
| **Knowledge** | notes + bookmarks + reading — FTS5 search-first, tags, links — LIVE |
| **Health** | meals + recipes + grocery, workouts + body metrics |
| **Universal Capture** | `items` core for any random personal data; agent-native, promotable |

Every pillar is **REST + MCP writable** — `Go owns the database`, TS never touches SQLite directly.

## Quick start

```bash
# 1. env
cp .env.example .env   # edit if needed

# 2. API (Go 1.22+, gcc required for mattn/go-sqlite3)
cd services/api
go run ./cmd/api
# → http://localhost:8080/healthz
# → http://localhost:8080/openapi.json

# 3. Dashboard (Node 18+)
npm install        # in apps/web (self-contained, not a workspace)
npm run dev
# → http://localhost:3000

# 4. Tests / lint
cd services/api && go test -tags sqlite_fts5 ./... && go vet -tags sqlite_fts5 ./...
cd apps/web && npm run lint && npx tsc --noEmit
```

> The `sqlite_fts5` build tag is **required** — FTS5 is compiled into mattn/go-sqlite3 only when it's set.

## Monorepo layout

```
personal-os/
├─ services/
│  ├─ api/           # Go REST API — all pillars
│  └─ scheduler/     # (Phase 6) background jobs
├─ apps/
│  ├─ web/           # Next.js dashboard (monochrome, dark/light)
│  └─ mcp/           # TS MCP servers: personal-os, scout, files
├─ packages/
│  └─ sdk/           # generated TS client from /openapi.json
├─ deploy/           # docker-compose, Dockerfiles
├─ docs/             # vision, spec, data-model, architecture, …
└─ data/             # SQLite file (gitignored)
```

## Docs

- `docs/vision.md` — product thesis
- `docs/spec.md` — feature spec per pillar
- `docs/data-model.md` — ERD + schemas
- `docs/architecture.md` — system design
- `docs/api-outline.md` — endpoint inventory
- `docs/mcp-tools.md` — MCP catalog
- `docs/design-system.md` — monochrome tokens, dark/light
- `docs/roadmap.md` — phases + acceptance gates
- `docs/decisions.md` — ADRs

## Architecture rules

- **Go owns the DB.** TS calls the Go API.
- Local-first, bearer-token auth now; cloud path stays open (env-driven, Postgres-portable SQL, `deploy/` compose).
- CSV import only for banks — no credential scraping.
- `sqlc` + `goose` + `chi` + `zerolog`.

## License

Private — not yet licensed for redistribution.

---

Built for portfolio + personal use. See `docs/vision.md` for design principles.
