# Deploy

## Local (dev)

```bash
cp .env.example .env
# choose: Go run + Next.js dev (see root README)
go run -tags sqlite_fts5 ./services/api/cmd/api   # from services/api/
cd apps/web && npm run dev
```

## Docker Compose

```bash
cd deploy
docker compose up --build
# → API:  http://localhost:8080  (healthz, openapi.json)
# → Web:  http://localhost:3000
```

- SQLite persists in the named volume `dbdata` (`/data/personal-os.db` inside the api container) and survives restarts.
- Set `API_TOKEN` to require bearer auth on `/v1/*`; the MCP server then needs the same value.
- `NEXT_PUBLIC_API_URL` is baked into the web bundle at **build** time because the browser calls the API directly. The default `http://localhost:8080` matches the published port. For remote hosts, set it to an externally reachable URL before building.

## Backup

SQLite is a single file — safe, atomic backup via the sqlite3 CLI (installed in the api image):

```bash
# On the host against the repo-local dev DB:
sqlite3 data/personal-os.db ".backup data/personal-os.backup.db"

# Inside compose:
docker compose exec api sqlite3 /data/personal-os.db ".backup /data/backup.db"
docker compose cp api:/data/backup.db ./backup.db
```

Back up while the API runs is safe with `.backup` (WAL-aware). Avoid copying `personal-os.db` alone while WAL files are live.

## Restore

```bash
docker compose stop api
docker compose cp ./backup.db api:/data/personal-os.db
docker compose start api
```

## Postgres portability audit

All schema SQL is written to migrate without query changes. Verified column-by-column against `docs/data-model.md`:

| SQLite (current) | Postgres target | Notes |
|---|---|---|
| `TEXT` RFC3339 timestamps | `timestamptz` | string compare tricks (`substr(col,1,10)`) move to `date_trunc`/ranges |
| `TEXT JSON + CHECK(json_valid)` | `jsonb` (+ GIN where queried) | `json_each(tags)` → `jsonb_array_elements_text` |
| FTS5 virtual table + triggers | `tsvector` + GIN + triggers | porter unicode61 ≈ `to_tsquery('english')` ranking differs; bm25 → `ts_rank` |
| `TEXT PRIMARY KEY` ULIDs | `text PRIMARY KEY` | unchanged |
| signed INTEGER minor units | `bigint` | unchanged |
| REAL kg/pct | `double precision` | unchanged |
| goose migrations | goose supports postgres dialect natively | same files, dialect switch |

Known divergences to handle during migration (all app-level, no domain rewrite):

1. `substr(measured_at,1,10)` day-bucketing (body metrics upsert, weight series) — swap for `measured_at::date`.
2. `'~'` upper-bound trick for month prefix scans (finance) — replace with `< date_trunc + interval`.
3. `INSERT OR IGNORE` / `ON CONFLICT DO NOTHING` — identical syntax works on Postgres.
4. FTS `MATCH ?` queries — centralize behind the store's search methods (already isolated in `internal/store/items.go`).

## What is NOT deployed

- `apps/mcp` runs on your machine over stdio (it must reach the API from wherever your agent runs). Point `PERSONAL_OS_URL` at the published API URL for remote use.
