# Personal OS

Local-first personal platform — one SQLite backbone, a Go API, a Next.js dashboard, and an MCP server so **any agent** can read/write your life data on your machine.

> **Integration note:** this repo is a component package, not a website. `apps/web` is only a reference host — `apps/web/components/*` are the pull-and-embed units (all API-driven, monochrome, dark/light). The MCP server is the primary interface for agents; a live bento board at `/` reacts to agent writes within seconds (poll-diff via `/v1/activity` — see ADR-0020).

> Status: **v1.1** — all pillars + universal capture + MCP (38 tools) + live board + agentic depth. See `docs/roadmap.md`.

## Pillars

| Pillar | What lives there |
|---|---|
| **Finance** | accounts, transactions (CSV import + dedupe), hierarchical categories/rules, budgets, summaries |
| **Planner** | tasks, habits with streaks/checkoffs, calendar events with RRULE-lite recurrence |
| **Knowledge** | notes, bookmarks (URL-normalized dedupe), reading list — unified FTS5 search, tags, links |
| **Health** | meals, recipes → grocery list, workouts, body metrics (day-upsert), weight series |
| **Universal Capture** | `items` core for anything else — searchable day-one, promotable to a pillar |

Every pillar is **REST + MCP writable** — **Go owns the database**, TypeScript never touches SQLite directly.

## Architecture

```
┌──────────────┐   stdio JSON-RPC    ┌───────────────────┐
│  MCP client  │◄───────────────────►│  apps/mcp         │
│ (opencode,   │      33 tools       │  personal-os      │
│ Claude Code) │                     └────────┬──────────┘
└──────────────┘                              │ HTTPS + Bearer
                                              ▼
┌──────────────┐   browser/HTTP      ┌───────────────────┐     ┌────────────────┐
│ apps/web     │◄───────────────────►│ services/api (Go) │────►│ data/*.db      │
│ Next.js dash │  REST /v1/*         │ chi · zerolog ·   │     │ SQLite (WAL)   │
│ monochrome   │                     │ goose migrations  │     │ FTS5 + triggers│
└──────────────┘                     └───────────────────┘     └────────────────┘
```

- `services/api` is the single writer. All state flows through `/v1/*` (OpenAPI served at `/openapi.json`).
- Knowledge rows mirror into `items` transactionally so global search covers everything.
- Recurring events expand at read time; habit streaks compute from checkoff history.

## Quick start

```bash
# 1. env
cp .env.example .env   # edit if needed

# 2. API (Go 1.22+, gcc required for mattn/go-sqlite3)
cd services/api
go run -tags sqlite_fts5 ./cmd/api
# → http://localhost:8080/healthz
# → http://localhost:8080/openapi.json

# 3. Dashboard (Node 20+)
cd ../apps/web
npm install        # self-contained, not a workspace
npm run dev
# → http://localhost:3000

# 4. Agent access (MCP over stdio)
cd ../mcp && npm install && npm run build
# wire dist/index.js into your MCP client — see apps/mcp/README.md

# 5. Tests / lint
cd ../../services/api && go test -tags sqlite_fts5 ./... && go vet -tags sqlite_fts5 ./...
cd ../../apps/web && npm run lint && npx tsc --noEmit
```

> The `sqlite_fts5` build tag is **required** — FTS5 is compiled into mattn/go-sqlite3 only when set.

## Monorepo layout

```
personal-os/
├─ services/
│  ├─ api/           # Go REST API — all pillars + universal core
│  └─ scheduler/     # (stretch) background jobs
├─ apps/
│  ├─ web/           # Next.js dashboard (monochrome, dark/light, chart-first)
│  └─ mcp/           # personal-os MCP stdio server (33 tools) + smoke driver
├─ deploy/           # docker-compose.yml, backup & Postgres portability docs
├─ docs/             # vision, spec, data-model, architecture, api-outline,
│                    # mcp-tools, design-system, roadmap, decisions (ADRs)
└─ data/             # SQLite file (gitignored)
```

## Pages

| Route | What you get |
|---|---|
| `/` | pillar overview + agent wiring |
| `/finance` | month picker, income/outcome/net cards, category chart, budget bars, CSV import, transactions table |
| `/planner` | today column (overdue/due/habits/events), month calendar + agenda, habit heatmap + completion bars, tasks table |
| `/knowledge` | search-first results with type/tag filters, quick capture tabs, link editor, reading kanban |
| `/health` | summary cards, weight trend line, training-minutes bars, meal+workout timeline, grocery checklist |

## Architecture rules

- **Go owns the DB.** TS calls the Go API; the MCP server is a thin HTTP wrapper.
- Local-first, bearer-token auth now; cloud path stays open (env-driven, Postgres-portable SQL, `deploy/` compose + portability audit).
- CSV import only for banks — no credential scraping.
- `goose` migrations, chi router, zerolog, ULIDs, monochrome design system (`docs/design-system.md`).

## Docs

- `docs/vision.md` — product thesis
- `docs/spec.md` — feature spec per pillar
- `docs/data-model.md` — ERD + schemas
- `docs/architecture.md` — system design
- `docs/api-outline.md` — endpoint inventory
- `docs/mcp-tools.md` — MCP catalog (33 tools)
- `docs/design-system.md` — monochrome tokens, dark/light
- `docs/roadmap.md` — phases + acceptance gates
- `docs/decisions.md` — ADRs

## License

Private — not yet licensed for redistribution.
