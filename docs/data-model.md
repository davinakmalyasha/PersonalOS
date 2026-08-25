# Data Model

SQLite v1, Postgres-portable SQL. One database file: `DB_PATH` (default `./data/personal-os.db`). Migrations via `goose`, types via `sqlc` where beneficial. All timestamps `TIMESTAMPTZ`-ish as `TEXT` RFC3339 UTC in SQLite (migrates to `timestamptz` in Postgres without query changes). Money as integer minor units.

## ERD (textual)

```
items ──< item_links >── items
 items ──< item_fts (FTS5) — triggers
 finance: accounts ──< transactions >── categories (self-ref parent)
                     categories ──< categorization_rules
                     categories ──< budgets (by month)
 planner: tasks
          habits ──< habit_checkoffs
          events (recurrence expanded at read time)
 knowledge: notes ─┐
            bookmarks├─ mirrored to items for unified FTS/links (via app write, not FK)
            reading_list ┘
 health: meals
         recipes ──< grocery_items (recipe_id nullable)
         workouts
         body_metrics
```

All FKs `ON DELETE RESTRICT` except join/child rows `CASCADE` where noted. `PRAGMA foreign_keys = ON` at every connection open.

---

## 0. Universal core

### items
| col | type (sqlite) | notes |
|---|---|---|
| id | TEXT PK (ULID/UUIDv7) | app-generated, not AUTOINCREMENT |
| type | TEXT NOT NULL | open vocab, indexed |
| title | TEXT NOT NULL | 1–300 chars |
| body | TEXT NOT NULL DEFAULT '' | markdown/plain, FTS source |
| data | TEXT NOT NULL DEFAULT '{}' | JSON object, CHECK(json_valid) |
| tags | TEXT NOT NULL DEFAULT '[]' | JSON array of strings |
| source | TEXT NOT NULL DEFAULT 'manual' | `manual|api|mcp|import|promotion` |
| source_item_id | TEXT nullable | FK to original item when promoted |
| created_at | TEXT NOT NULL | RFC3339 UTC |
| updated_at | TEXT NOT NULL | RFC3339 UTC |
Indexes: `(type)`, `(created_at DESC)`, FTS via `items_fts`.

### item_links
| col | type | notes |
|---|---|---|
| from_id | TEXT FK→items.id CASCADE | |
| to_id | TEXT FK→items.id CASCADE | |
| kind | TEXT NOT NULL DEFAULT 'related' | `related|parent|blocks|duplicate` open vocab |
| created_at | TEXT NOT NULL | |
PK `(from_id, to_id, kind)`

### FTS
```sql
CREATE VIRTUAL TABLE items_fts USING fts5(title, body, tags, content='items', content_rowid='rowid', tokenize='porter unicode61');
-- triggers on items insert/update/delete keep it in sync
```
Global search `SELECT ... FROM items_fts WHERE items_fts MATCH ?` joins to `items` for metadata.

Postgres path: replace with `tsvector` + GIN; `items.data` becomes `jsonb`.

---

## 1. Finance

### accounts
`id TEXT PK, name TEXT NOT NULL UNIQUE, type TEXT NOT NULL CHECK(type IN ('checking','savings','cash','card','wallet')), currency TEXT NOT NULL, created_at TEXT NOT NULL`
Index `(type)`.

### categories
`id TEXT PK, name TEXT NOT NULL, parent_id TEXT nullable FK→categories.id RESTRICT, color TEXT nullable, created_at TEXT NOT NULL`
Unique `(coalesce(parent_id,''), name)` enforced in app (SQLite quirky with NULL). Index `(parent_id)`.

### transactions
`id TEXT PK, account_id TEXT NOT NULL FK→accounts.id RESTRICT, amount INTEGER NOT NULL, currency TEXT NOT NULL, date TEXT NOT NULL -- YYYY-MM-DD, merchant TEXT NOT NULL, raw_description TEXT NOT NULL, category_id TEXT nullable FK→categories RESTRICT, hash TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL`
Unique `(date, amount, hash)` — the dedupe key (partial index could exclude soft-deleted future). Indexes: `(account_id)`, `(category_id)`, `(date DESC)`, `(merchant)`.

`amount` is signed integer minor units (e.g., cents). Negative = outflow in the app's sign convention (or store absolute + direction — ADR: signed, spend is negative, income positive; summaries use abs).

`hash` = hex(SHA256(normalize(raw_description))) — normalize = `lower(trim(collapse_whitespace(strip_punct)))`.

### categorization_rules
`id TEXT PK, pattern TEXT NOT NULL, category_id TEXT NOT NULL FK→categories.id RESTRICT, priority INTEGER NOT NULL, created_at TEXT NOT NULL`
Index `(priority, id)`.

### budgets
`id TEXT PK, category_id TEXT NOT NULL FK→categories.id RESTRICT, month TEXT NOT NULL -- YYYY-MM, amount INTEGER NOT NULL, created_at TEXT NOT NULL`
Unique `(category_id, month)`. Index `(month)`.

---

## 2. Planner

### tasks
`id TEXT PK, title TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '', status TEXT NOT NULL CHECK(status IN ('todo','doing','done')), priority TEXT NOT NULL CHECK(priority IN ('low','med','high')), due_date TEXT nullable -- YYYY-MM-DD, project TEXT nullable, tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, completed_at TEXT nullable`
Indexes: `(status)`, `(due_date)`, `(priority)`, `(project)`.

### habits
`id TEXT PK, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', cadence TEXT NOT NULL DEFAULT 'daily' CHECK(cadence IN ('daily','weekly')), target_per_week INTEGER NOT NULL DEFAULT 7, color TEXT nullable, created_at TEXT NOT NULL, archived_at TEXT nullable`
Index `(archived_at)`.

### habit_checkoffs
`id TEXT PK, habit_id TEXT NOT NULL FK→habits.id CASCADE, date TEXT NOT NULL -- YYYY-MM-DD, created_at TEXT NOT NULL`
Unique `(habit_id, date)`. Index `(habit_id, date DESC)`.

### events
`id TEXT PK, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', starts_at TEXT NOT NULL -- RFC3339, ends_at TEXT nullable -- RFC3339, location TEXT NOT NULL DEFAULT '', recurrence_rule TEXT nullable -- RRULE-lite, tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
Index `(starts_at)`. Recurrence expanded at read time; no materialized instances.

RRULE-lite grammar (v1): `FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=YYYYMMDD[T...Z]`. Unknown tokens → 400.

---

## 3. Knowledge

Three tables with similar shape, all mirrored to `items` on write by the application layer (transactional) for unified FTS.

### notes
`id TEXT PK, title TEXT NOT NULL, body TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '[]' JSON, pinned INTEGER NOT NULL DEFAULT 0, archived_at TEXT nullable, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
FTS fallback via app; also reachable via `items.type='note'`.

### bookmarks
`id TEXT PK, url TEXT NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
Unique `(url)` on normalized URL. Index `(created_at DESC)`.

### reading_list
`id TEXT PK, title TEXT NOT NULL, author TEXT nullable, url TEXT nullable, status TEXT NOT NULL CHECK(status IN ('to-read','reading','done')), rating INTEGER nullable CHECK(rating BETWEEN 1 AND 5), notes TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, finished_at TEXT nullable`
Index `(status)`.

Mirroring: on note/bookmark/reading write, the service writes a corresponding `items` row in the same DB transaction (`type` = `note|bookmark|reading`, `data` = JSON of native fields). This keeps `GET /search` and `item_links` trivial.

---

## 4. Health

### meals
`id TEXT PK, eaten_at TEXT NOT NULL -- RFC3339, title TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '', items TEXT NOT NULL DEFAULT '[]' -- JSON array, calories INTEGER nullable, tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
Index `(eaten_at DESC)`.

### recipes
`id TEXT PK, title TEXT NOT NULL, ingredients TEXT NOT NULL DEFAULT '[]' -- JSON, instructions TEXT NOT NULL DEFAULT '', servings INTEGER nullable, calories_per_serving INTEGER nullable, tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`

### grocery_items
`id TEXT PK, name TEXT NOT NULL, qty TEXT NOT NULL DEFAULT '' , unit TEXT nullable, checked INTEGER NOT NULL DEFAULT 0, recipe_id TEXT nullable FK→recipes.id SET NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
Index `(checked)`.

### workouts
`id TEXT PK, performed_at TEXT NOT NULL -- RFC3339, title TEXT nullable, notes TEXT NOT NULL DEFAULT '', duration_minutes INTEGER nullable, exercises TEXT NOT NULL DEFAULT '[]' -- JSON, tags TEXT NOT NULL DEFAULT '[]' JSON, created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
Index `(performed_at DESC)`.

### body_metrics
`id TEXT PK, measured_at TEXT NOT NULL -- RFC3339, weight_kg REAL nullable, body_fat_pct REAL nullable, notes TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL`
Unique index on `date(measured_at)` (app-enforced upsert: same calendar day replaces).

---

## Conventions

- **IDs:** ULID (lexicographically sortable, via `oklog/ulid` or equivalent) — avoids AUTOINCREMENT portability quirks. UUIDv7 also acceptable; pick one and document.
- **JSON fields:** stored `TEXT` with `CHECK(json_valid(col))` and `CHECK(json_type(col)='array'|'object')` where appropriate; Postgres maps to `jsonb` with GIN where beneficial.
- **Monotonic `updated_at`:** set server-side on every write to `now UTC RFC3339`.
- **Soft delete:** not in v1; `DELETE` is hard. Trash is a stretch requiring `deleted_at` + filtered views.
- **Indexes:** only those listed are created in migrations; add as query plans demand and record in ADR.
- **Migrations:** one file per logical change, reversible `up/down`, `goose` run automatically on API boot.

## SQL portability notes (kept in `deploy/README.md` too)

- Avoid `AUTOINCREMENT`, `STRICT` edge cases, `RETURNING` relied on in app — guard with compat layer if needed (Postgres supports `RETURNING` natively).
- `TEXT` timestamps migrate cleanly to `timestamptz`; arithmetic moves to `interval` on Postgres.
- FTS5 → `tsvector`; JSON `TEXT` → `jsonb`; `PRAGMA foreign_keys` → `SET CONSTRAINTS` (default on in Postgres).
