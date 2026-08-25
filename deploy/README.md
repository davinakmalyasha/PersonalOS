# Deploy

## Local

```bash
cp .env.example .env
# choose: Go run + Next.js dev (see README)
go run ./services/api/cmd/api
npm run dev:web
```

## Docker (Phase 7)

`deploy/docker-compose.yml` will run `api` + `web` with a mounted `./data` volume for SQLite persistence. Dockerfiles live beside it.

## Backup

```bash
sqlite3 data/personal-os.db ".backup data/personal-os.backup.db"
# or
go run ./services/api/cmd/api --backup data/backup.db  # stretch CLI
```

## Postgres portability audit

Documented in `docs/data-model.md` (SQL notes section). All SQLite SQL is Postgres-portable: `TEXT` timestamps → `timestamptz`, `TEXT JSON` → `jsonb`, `FTS5` → `tsvector`, `AUTOINCREMENT` avoided, `PRAGMA foreign_keys` → default-on.

Cloud migration = new `DB_DSN` + `pgx` driver + JSONB adjustments — no domain rewrite.
