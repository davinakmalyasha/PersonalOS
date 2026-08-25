# API Outline — Endpoint Inventory

Base URL: `http://localhost:8080`. Public routes: `GET /healthz`, `GET /openapi.json`. All `/v1/*` require `Authorization: Bearer <API_TOKEN>` when `API_TOKEN` is set.

Response shape: JSON; paginated lists return `{items, total, page, page_size}`; errors return `{error, code, details[]}`.

---

## System

| Method | Path | Description |
|---|---|---|
| GET | /healthz | liveness + DB ping → `{status:"ok", db:"ok"}` or 503 |
| GET | /openapi.json | canonical OpenAPI spec |

## Universal Capture (`/v1/items`)

| Method | Path | Description |
|---|---|---|
| POST | /v1/items | create item (type, title, body, data JSON, tags) |
| GET | /v1/items | list with `?type=&tag=&q=&page=&page_size=` |
| GET | /v1/items/{id} | get by id |
| PATCH | /v1/items/{id} | partial update |
| DELETE | /v1/items/{id} | hard delete |
| POST | /v1/items/{id}/links | create link `{to_id, kind}` |
| GET | /v1/items/{id}/links | list links for item |
| DELETE | /v1/items/{id}/links/{toId}/{kind} | remove link |
| POST | /v1/items/{id}/promote | promote to typed record `{target: "transaction"|"task"|…}` |
| GET | /v1/search | global FTS `?q=&type=&tag=&limit=` across items + mirrored knowledge |
| GET | /v1/tags | distinct tags with counts `?prefix=&limit=` |

## Finance (`/v1/finance` + `/v1/transactions` alias)

| Method | Path | Description |
|---|---|---|
| POST | /v1/accounts | create account |
| GET | /v1/accounts | list accounts with balance rollup |
| GET | /v1/accounts/{id} | get account |
| PATCH | /v1/accounts/{id} | update account |
| DELETE | /v1/accounts/{id} | delete (blocks if transactions exist) |
| POST | /v1/transactions | create transaction |
| GET | /v1/transactions | list `?account_id=&category_id=&from=&to=&min=&max=&q=&page=&page_size=` |
| GET | /v1/transactions/{id} | get transaction |
| PATCH | /v1/transactions/{id} | update (category override) |
| DELETE | /v1/transactions/{id} | delete |
| POST | /v1/transactions/import | multipart CSV import `?account_id=` → `{imported, skipped, errors}` |
| GET | /v1/categories | list categories (tree) |
| POST | /v1/categories | create category |
| PATCH | /v1/categories/{id} | update / reparent |
| DELETE | /v1/categories/{id} | delete (reassign or block) |
| GET | /v1/rules | list categorization rules ordered by priority |
| POST | /v1/rules | create rule |
| PATCH | /v1/rules/{id} | update rule / reorder priority |
| DELETE | /v1/rules/{id} | delete rule |
| POST | /v1/rules/evaluate | trigger re-evaluation for `?from=&to=` |
| GET | /v1/budgets | list budgets `?month=&from=&to=` |
| POST | /v1/budgets | upsert budget `{category_id, month, amount}` |
| DELETE | /v1/budgets/{id} | delete budget |
| GET | /v1/finance/summary | `?month=YYYY-MM` → totals, by-account, by-category, vs budget |
| GET | /v1/finance/spending | `?group_by=month|category&from=&to=` → series |

## Planner (`/v1/planner`, `/v1/tasks`, `/v1/habits`, `/v1/events`)

| Method | Path | Description |
|---|---|---|
| POST | /v1/tasks | create task |
| GET | /v1/tasks | list `?status=&priority=&due=&project=&tag=&q=&page=&page_size=` |
| GET | /v1/tasks/{id} | get task |
| PATCH | /v1/tasks/{id} | update task / status |
| DELETE | /v1/tasks/{id} | delete |
| POST | /v1/habits | create habit |
| GET | /v1/habits | list habits `?archived=` |
| GET | /v1/habits/{id} | get habit with streak |
| PATCH | /v1/habits/{id} | update / archive |
| DELETE | /v1/habits/{id} | delete (cascades checkoffs) |
| POST | /v1/habits/{id}/checkoffs | toggle checkoff `{date}` |
| GET | /v1/habits/{id}/checkoffs | list checkoffs `?from=&to=` |
| POST | /v1/events | create event |
| GET | /v1/events | list `?from=&to=` (expands recurrences) |
| GET | /v1/events/{id} | get event |
| PATCH | /v1/events/{id} | update event |
| DELETE | /v1/events/{id} | delete |
| GET | /v1/planner/today | tasks due today + overdue + habits today + events today |
| GET | /v1/planner/upcoming | next N days agenda `?days=7` |
| GET | /v1/planner/overview | `?date=YYYY-MM-DD` unified daily bundle |

## Knowledge (`/v1/knowledge`, `/v1/notes`, `/v1/bookmarks`, `/v1/reading`)

| Method | Path | Description |
|---|---|---|
| POST | /v1/notes | create note |
| GET | /v1/notes | list `?tag=&q=&pinned=&archived=&page=&page_size=` |
| GET | /v1/notes/{id} | get note |
| PATCH | /v1/notes/{id} | update note |
| DELETE | /v1/notes/{id} | delete |
| POST | /v1/bookmarks | create bookmark `{url, title, description, tags}` |
| GET | /v1/bookmarks | list `?tag=&q=&page=&page_size=` |
| GET | /v1/bookmarks/{id} | get bookmark |
| PATCH | /v1/bookmarks/{id} | update |
| DELETE | /v1/bookmarks/{id} | delete |
| POST | /v1/reading | create reading entry |
| GET | /v1/reading | list `?status=&tag=&q=&page=&page_size=` |
| GET | /v1/reading/{id} | get reading entry |
| PATCH | /v1/reading/{id} | update (status/rating/notes) |
| DELETE | /v1/reading/{id} | delete |
| GET | /v1/knowledge/search | cross-type FTS `?q=&type=&tag=&limit=` |
| GET | /v1/knowledge/tags | distinct tags with counts |

## Health (`/v1/health`, `/v1/meals`, `/v1/recipes`, `/v1/workouts`)

| Method | Path | Description |
|---|---|---|
| POST | /v1/meals | create meal |
| GET | /v1/meals | list `?from=&to=&q=&page=&page_size=` |
| GET | /v1/meals/{id} | get meal |
| PATCH | /v1/meals/{id} | update |
| DELETE | /v1/meals/{id} | delete |
| POST | /v1/recipes | create recipe |
| GET | /v1/recipes | list `?tag=&q=&page=&page_size=` |
| GET | /v1/recipes/{id} | get recipe |
| PATCH | /v1/recipes/{id} | update |
| DELETE | /v1/recipes/{id} | delete |
| POST | /v1/recipes/{id}/use | create meal from recipe `{eaten_at}` |
| GET | /v1/grocery | list grocery items `?checked=` |
| POST | /v1/grocery | add grocery item |
| PATCH | /v1/grocery/{id} | update / toggle checked |
| DELETE | /v1/grocery/{id} | delete |
| POST | /v1/grocery/clear-checked | bulk clear checked |
| POST | /v1/workouts | create workout |
| GET | /v1/workouts | list `?from=&to=&q=&page=&page_size=` |
| GET | /v1/workouts/{id} | get workout |
| PATCH | /v1/workouts/{id} | update |
| DELETE | /v1/workouts/{id} | delete |
| POST | /v1/body-metrics | upsert metric `{measured_at, weight_kg, body_fat_pct}` |
| GET | /v1/body-metrics | list `?from=&to=&page=&page_size=` |
| GET | /v1/body-metrics/{id} | get metric |
| DELETE | /v1/body-metrics/{id} | delete |
| GET | /v1/health/summary | `?from=&to=` → workout/meal/weight/grocery rollup |
| GET | /v1/health/weight-series | `?from=&to=&bucket=day` → chart series |

## Conventions

- **Pagination:** `page` (1-indexed) + `page_size` (default 20, max 100). Always returns `total`.
- **Timestamps:** request/response use RFC3339 UTC; date-only fields are `YYYY-MM-DD`.
- **Idempotency:** duplicate natural keys (transaction hash, bookmark URL, body-metric date, habit checkoff date) return `200` with existing id or `409` per endpoint docs; imports are idempotent via hash.
- **Validation errors:** `400 {error:"validation_error", details:[{field, message}]}`.
- **Auth:** when `API_TOKEN` set, all `/v1/*` require `Authorization: Bearer $API_TOKEN`. `/healthz` and `/openapi.json` are public.
