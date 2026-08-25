# Personal OS

Local-first personal platform — one SQLite backbone, a Go API, a Next.js dashboard, and MCP tooling so **any agent** can read/write your life data on your machine.

> Status: Phase 0–1 (Foundation + Documentation). See `docs/roadmap.md` for the full plan.

## Pillars

| Pillar | What lives there |
|---|---|
| **Finance** | accounts, transactions, categories/rules, budgets. CSV import + dedupe |
| **Planner** | tasks + habits + calendar events (recurrence) |
| **Knowledge** | notes + bookmarks + reading list — FTS5, tags, links |
| **Health** | food (meals/recipes/grocery) + fitness (workouts/body metrics) |
| **Universal Capture** | `items` core for any random personal data; agent-native, promotable |

Every pillar is **REST + MCP writable** — `Go owns the database`, TS never touches SQLite directly.

## Quick start

```bash
# 1. env
cp .env.example .env   # edit if needed

# 2. API (Go 1.22+, gcc required for mattn/go-sqlite3)
go run ./services/api/cmd/api
# → http://localhost:8080/healthz
# → http://localhost:8080/openapi.json

# 3. Dashboard (Node 18+)
npm install
npm run dev:web
# → http://localhost:3000

# 4. Tests / lint
go test ./...
go vet ./...
npm run lint
```

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
