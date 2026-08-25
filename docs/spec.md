# Spec — Feature Specification (Coverage-Maximal)

This spec defines the **full intended surface** of Personal OS across its four pillars + universal core. Phase roadmap slices this into shippable increments; nothing here is optional without an ADR.

---

## 0. Universal Capture Core (`items`) — the extensibility guarantee

**Purpose:** absorb *any* random personal data without waiting for a new module. Every niche category tomorrow is already supported.

**Model:** `items(id, type, title, body, data JSON, tags[], source, created_at, updated_at)` plus `item_links(from_id, to_id, kind)` and FTS.

**Features:**

- `type` is open-vocabulary (`note`, `idea`, `warranty`, `gift`, `contact`, `snippet`, …); UI groups by type but never rejects an unknown type.
- `data` JSON stores type-specific structured fields (e.g., warranty expiry date) while staying SQLite TEXT ↔ Postgres JSONB portable. Indexed via expression indexes where needed later.
- `tags` array + `links` give cross-pillar graph (link a receipt item → transaction, a workout → goal note).
- CRUD: create, get by id, list (filter by type/tags/query), patch, delete (soft or hard — hard in v1, ADR if soft needed).
- FTS5 over `title` + `body` (+ `data` stringified on write), <100ms on 10k rows.
- Global search bar on dashboard queries `items` FTS across all pillars.
- **Promotion:** one-click/tool call to convert an item into a typed pillar record (e.g., `item.type=meal-draft` → `health.meals` row), preserving `source_item_id`.
- Inbox view: untyped / recently captured items first; zero friction capture (dashboard quick-add + `save_item`).

**Acceptance:** saving an item with a never-seen-before `type` succeeds and is findable via FTS without migration.

---

## 1. Finance — standalone, richest domain ⭐

### 1.1 Accounts
- Fields: `id, name, type (checking/savings/cash/card/wallet), currency (ISO 4217, default from env), created_at`.
- CRUD; list with balance rollup (sum of transactions per account).
- Deleting an account with transactions is blocked (or reassigns — ADR; v1 blocks).

### 1.2 Transactions
- Fields: `id, account_id FK, amount (integer minor units + currency), date (YYYY-MM-DD), merchant (normalized), raw_description, category_id FK nullable, hash (for dedupe), notes, created_at`.
- Amount stored as integer cents to avoid float drift; display formatted per currency.
- `merchant` is cleaned (trim, collapse spaces) on write; `raw_description` preserved verbatim.
- **Dedupe:** natural key = `(date, amount, description_hash)`. `description_hash` = SHA256 of normalized `raw_description` (lowercase, strip punctuation/whitespace). Import checks hash set per file + existing DB; duplicate counted as `skipped`, never inserted.
- REST CRUD + bulk import (see 1.6).
- Filters/search/pagination: by account, category, date range, amount range, merchant substring, `q` FTS over merchant + raw_description.

### 1.3 Categories (hierarchical)
- Fields: `id, name, parent_id FK nullable, color (optional, monochrome grey scale derived if absent), created_at`.
- Tree: e.g., `Food → Groceries`, `Food → Dining`. Depth unbounded but UI shows 2 levels well.
- CRUD; moving a category updates children via transaction; deleting a used category requires reassign or blocks.
- `uncategorized` is implicit for null `category_id`; not a stored row.

### 1.4 Categorization Rules
- Fields: `id, pattern (case-insensitive substring or regex-lite), category_id FK, priority (int, lower = higher priority), created_at`.
- Evaluation: on import and on manual trigger, rules sorted by `priority, id` and first matching pattern wins.
- Manual override: editing a transaction's category offers "create rule from this transaction" (pattern = merchant substring suggestion).
- CRUD + reorder (priority update); test coverage mandatory.

### 1.5 Budgets
- Fields: `id, category_id FK, month (YYYY-MM), amount (int minor units), created_at`. Unique on `(category_id, month)`.
- CRUD; list by month range.
- Summary endpoint computes `spent` per category/month vs `budget`, with over-budget flag.

### 1.6 CSV Import
- Endpoint: `POST /v1/transactions/import` (multipart `file`, `account_id`, optional column mapping override).
- Auto-detect: header sniff (case-insensitive) for `date`, `amount`/`debit`/`credit` (handles split columns), `description`/`memo`/`narrative`, `merchant` fallback to description substring.
- Date parse: tries `YYYY-MM-DD`, `DD/MM/YYYY`, `MM/DD/YYYY`, `DD Mon YYYY`; amount parse handles currency symbols, commas, parentheses for negatives.
- Dedupe per 1.2; returns `{imported, skipped, errors[]}`.
- Rules engine auto-categorizes imported rows immediately; counts of auto-categorized exposed in response.
- Importing the same file twice → `imported=0, skipped=N`.

### 1.7 Summaries
- `GET /v1/finance/summary?month=YYYY-MM` → totals, by-account, by-category (tree-rolled), vs budget.
- `GET /v1/finance/spending?group_by=month|category&from=&to=` → time series + breakdown, suitable for Recharts.
- Spot-check invariant: sum of category spend == month total (within rounding).

### 1.8 Dashboard (Finance page)
- Month picker, summary cards (income/outcome/net), category breakdown chart (donut/bar monochrome), transactions table (filter/search/pagination), budget bars with over-budget emphasis.
- No color needed — greys + border weight convey hierarchy.

---

## 2. Planner — Tasks + Habits + Calendar Events (merged)

### 2.1 Tasks
- Fields: `id, title, notes (nullable), status (todo/doing/done), priority (low/med/high), due_date (nullable, DATE), project (nullable string for lightweight grouping), tags[], created_at, updated_at, completed_at nullable`.
- CRUD + `PATCH /tasks/{id}/status`.
- Quick capture: minimal `title` only; full edit later.
- Filters: by status, priority, due (overdue/today/upcoming/undated), project, tags, `q`.
- Sorting: `due_date nulls last, priority desc, created_at desc`.

### 2.2 Habits
- Fields: `id, name, description nullable, cadence (daily/weekly — daily in v1, weekly stretch), target_per_week (int, for weekly), color nullable, created_at, archived_at nullable`.
- Checkoffs: `habit_checkoffs(id, habit_id FK, date DATE, created_at)` unique on `(habit_id, date)`.
- Endpoints: toggle checkoff for date, list checkoffs by range.
- Streak: consecutive days (or weeks) with checkoff up to today; computed server-side.
- Weekly target satisfied flag per week.

### 2.3 Events (Calendar)
- Fields: `id, title, description nullable, starts_at (RFC3339), ends_at (RFC3339, nullable; defaults to starts+1h UI), location nullable, recurrence_rule (RRULE-lite string nullable), tags[], created_at, updated_at`.
- Recurrence: v1 supports `FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=date` subset. Expansion is read-time (next 90 days window) rather than materialized instances.
- No external calendar sync in v1 (ICS import is a stretch ADR).
- Endpoints: CRUD + `GET /planner/events?from=&to=` (expands recurring instances), `GET /planner/today`, `GET /planner/upcoming`.
- Dashboard: month grid + agenda list + today column; habit streak heatmap embedded.

### 2.4 Planner Unified Queries
- `GET /planner/overview?date=YYYY-MM-DD` → tasks due today + overdue, habit checkoffs today, events today. The "daily brief" without LLM.

---

## 3. Knowledge — Notes + Bookmarks + Reading List (merged)

Shared traits: `id, title, body/notes, tags[], created_at, updated_at`, FTS over `title+body`, owner implicit (single user).

### 3.1 Notes
- `body` is Markdown text; rendered in UI.
- Pin/archived flags.

### 3.2 Bookmarks
- `url (unique-ish, normalized), title (fetched or user-provided), description, tags[], created_at`.
- `url` normalized (lowercase host, strip tracking params `utm_*`); duplicate URL on create returns existing (idempotent by URL).

### 3.3 Reading List
- Fields: `id, title, author nullable, url nullable, status (to-read/reading/done), rating (1–5 nullable), notes nullable, tags[], created_at, finished_at nullable`.
- Optional Google Books lookup by title/ISBN (when enabled): fetches cover + canonical author. Results cached per query; failure never blocks create.

### 3.4 Knowledge Features
- Global FTS: `GET /knowledge/search?q=&type=&tag=&limit=` searches notes/bookmarks/reading in one call.
- Tag autocomplete: distinct tags with counts.
- Link graph: `knowledge_links` or reuse universal `item_links` after knowledge rows are projected as `items` views — v1 uses direct `item_links` via universal layer for simplicity (knowledge rows mirror to `items` on write — see data-model).
- Attachments: stretch — file upload tied to note.

---

## 4. Health — Food + Fitness/Body (merged)

### 4.1 Meals
- Fields: `id, eaten_at (RFC3339), title, notes nullable, items JSON (array of {name, qty, unit}) nullable, calories nullable, tags[], created_at`.
- Log model is journal-style; no barcode DB in v1. Calories are optional manual entry; later derived from recipes.
- CRUD + list by date range, `q` filter.

### 4.2 Recipes
- Fields: `id, title, ingredients JSON (array of {name, qty, unit}), instructions (markdown), servings nullable, calories_per_serving nullable, tags[], created_at`.
- Using a recipe from the meal log copies its ingredients into the meal's `items`.

### 4.3 Grocery List
- Derived from selected recipes + manual entries. Entity: `grocery_items(id, name, qty, unit nullable, checked bool, recipe_id FK nullable, created_at)`.
- Check/uncheck, clear checked.
- Stretch: auto-aggregate duplicate ingredient names.

### 4.4 Workouts
- Fields: `id, performed_at (RFC3339), title nullable, notes nullable, duration_minutes nullable, exercises JSON (array of {name, sets, reps, weight, distance, duration})`, `tags[], created_at`.
- CRUD + list by range.

### 4.5 Body Metrics
- Fields: `id, measured_at (RFC3339), weight_kg nullable, body_fat_pct nullable, notes nullable, created_at`. Unique-ish on `date(measured_at)` — upsert on same day replaces.
- Trend chart data endpoint returns daily bucketed series.

### 4.6 Health Summaries
- `GET /health/summary?from=&to=` → workout count/duration, meal count, weight delta, grocery pending count.
- Dashboard: weight trend line, workout frequency bars, recent meals/workouts tables, grocery checklist widget.

---

## Cross-cutting product rules

- **Every create is idempotent where a natural key exists** (transaction hash, bookmark URL, body-metric date, habit checkoff date).
- **Soft vs hard delete:** hard delete in v1 for simplicity; deleted ids never reused. Trash/undo is a stretch.
- **Timestamps:** stored UTC, returned RFC3339. `created_at` set server-side, not client-supplied.
- **Validation:** 400 with field-level error shape `{field, message}` on bad input; never 500 for user error.
- **Tech debt ceiling:** no feature lands without its tests + OpenAPI addition + dashboard affordance if user-facing.
