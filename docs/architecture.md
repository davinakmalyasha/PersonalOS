# Architecture

## System map

```
                         ┌─────────────────┐
     browser ─────────────►  apps/web       │  Next.js App Router, Tailwind, shadcn, Recharts
                         │  (monochrome)  │  SSR where useful, client fetch to Go API
                         └────────┬────────┘
                                  │ REST + bearer token
                         ┌────────▼────────┐
 any MCP client ──────────►  apps/mcp       │  TS, @modelcontextprotocol/sdk
  (opencode,             │  server:         │  thin wrappers over Go API, no DB access
   claude-code, …)       │  personal-os    │  bearer token from env
                         └────────┬────────┘
                                  │ REST
                         ┌────────▼────────┐
                         │  services/api   │  Go 1.22+, chi, sqlc, goose, zerolog
                         │  ┌────────────┐ │  config (.env + env), auth middleware
                         │  │  SQLite    │ │  single file DB_PATH, WAL, FK on
                         │  │  (goose)   │ │  migrations embedded via embed.FS
                         │  └────────────┘ │
                         └─────────────────┘
                                  │
                         ┌────────▼────────┐
                         │  deploy/        │  docker-compose, Dockerfiles, backup script
                         └─────────────────┘
```

**Rule:** Go owns the database. TS apps never open SQLite. Auth is bearer token in `Authorization: Bearer <API_TOKEN>`; when `API_TOKEN` is empty the API runs without auth (dev only) and `/healthz` + `/openapi.json` are always public.

## Go API structure

```
services/api/
├─ cmd/api/main.go        # wire config→logger→db→migrations→router→listen
├─ migrations/            # goose *.sql, embedded
├─ internal/
│  ├─ config/             # env + .env (godotenv), typed Config struct
│  ├─ logging/            # zerolog setup, pretty console in dev
│  ├─ db/                 # sql.Open("sqlite3"), pragmas, goose runner, health ping
│  ├─ server/             # chi router, middleware, routes, openapi.json, handlers
│  │  ├─ server.go
│  │  ├─ openapi.json     # hand-maintained until generation later
│  │  ├─ finance.go       # pillar handlers (added per phase)
│  │  ├─ planner.go
│  │  ├─ knowledge.go
│  │  └─ health.go …
│  └─ domain/             # pure logic: dedupe, rules engine, recurrence expansion, FTS helpers
└─ go.mod                 # module github.com/davinakmalyasha/PersonalOS (repo root)
```

At `services/api/go.mod` the module is `github.com/davinakmalyasha/PersonalOS/services/api` (per-service modules — see ADR-0014). Run Go commands from that directory; `go test ./...` covers the service. JS apps install their own deps (no workspaces) so Node's tree never pollutes Go's.

## Module plugin pattern

Each pillar is a **vertical slice**: its table(s) + queries (`sqlc` or `database/sql`) + domain logic + handlers + OpenAPI addition + tests + dashboard page. Adding a pillar never edits another pillar's files except `server.go` route mounting and `openapi.json`. This keeps PRs small and portfolio review linear.

## Request lifecycle

1. Middleware: `RequestID` → `RealIP` → `Recoverer` → `Logger(zerolog)` → `Timeout(15s)` → `Auth` (bearer check if `API_TOKEN` set and route not public).
2. Handler: parse + validate → domain call → DB transaction where needed → JSON response.
3. Errors: domain returns typed errors; server maps to `{error, code, details[]}` with field-level `{field, message}` for 400, 404 with `{error:"not_found"}`, 500 only for unexpected.

## Config

Loaded via `joho/godotenv` (`.env` if present, then real env overrides). Struct:

```
PORT          string (default 8080)
DB_PATH       string (default ./data/personal-os.db)
LOG_LEVEL     string (default info)
API_TOKEN     string (default "" — dev open)
OPENAI_API_KEY string (unused in v1, reserved)
```

No secrets in repo; `.env.example` documents every key.

## Database

- Driver: `mattn/go-sqlite3` (cgo, via MinGW on Windows). Pragmas on every open: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`.
- Migrations: `goose` with `embed.FS` of `migrations/*.sql`; run on boot before listening. Each migration is transactional (`-- +goose Up/Down`).
- Queries: `sqlc` for typed pillars as they grow; v1 foundation can use `database/sql` directly for health/migration paths. `sqlc.yaml` added when the first typed query lands (Finance).
- Backups: `sqlite3 data/personal-os.db ".backup data/backup.db"` documented in `deploy/README.md`; future Postgres path uses `pg_dump`.

## OpenAPI + SDK

- `services/api/internal/server/openapi.json` is the canonical spec, embedded and served at `GET /openapi.json` with `Content-Type: application/json`.
- `packages/sdk` is later generated from that spec (e.g., `openapi-typescript` / `openapi-generator`). Until generation lands, consumers fetch the spec directly.

## Dashboard

- `apps/web` — Next.js 14+ App Router, Tailwind, `shadcn/ui`. Monochrome design system (`docs/design-system.md`). Data fetching via `fetch` to `NEXT_PUBLIC_API_URL` (defaults to `http://localhost:8080`).
- Global search bar → `GET /search` (universal FTS). Each pillar has a dedicated page with filters + charts (Recharts, monochrome palette).

## MCP

- `apps/mcp` — `server: personal-os` exposing one tool per logical operation (see `docs/mcp-tools.md`). Auth: `PERSONAL_OS_URL` + `PERSONAL_OS_TOKEN` from env. Transport: stdio (primary, simplest for local clients) + optional SSE/HTTP stretch.
- Clients wire it with one JSON block (opencode / Claude Desktop format); `docs/mcp-tools.md` documents both.

## Security (current)

- Single-user, bearer token.
- Routes `/healthz` and `/openapi.json` are public; all `/v1/*` require `Authorization` when `API_TOKEN` is set.
- Input validation is strict: unknown JSON fields rejected, enum checks, length caps, URL normalization for bookmarks.

## Testing strategy

- **Go:** `go test ./...` must be green. Domain logic (dedupe hash, rules engine, recurrence expansion, URL normalization, CSV column detection) has unit tests with table-driven cases. HTTP handlers tested with `httptest`. Migrations tested against temp SQLite files (up then down).
- **Web:** `npm run lint && npx tsc --noEmit` green; visual smoke of each pillar page (monochrome renders, charts mount without data).
- **MCP:** manual smoke via an MCP inspector/Muse client against the live API.

## Cloud-migration path (kept open, not built)

- All SQL is Postgres-portable per `docs/data-model.md` notes.
- `deploy/docker-compose.yml` runs `api` + `web` locally; swapping SQLite path for a Postgres DSN and `sql.DB` driver is a config + query shim, not a rewrite. Documented in `deploy/README.md`.

## Logging

`zerolog` with level from `LOG_LEVEL`. Request logs include `request_id`, `method`, `path`, `status`, `duration_ms`. No PII in logs beyond what the request already carries.

## Failure modes

- DB unavailable at boot → log + exit non-zero (systemd/docker restart handles it).
- Migration failure → log + exit; never listen on a half-migrated DB.
- CSV import: per-row errors collected, not fatal; response includes `errors[]` with row numbers.
