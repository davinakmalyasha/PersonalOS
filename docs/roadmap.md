# Roadmap — Phases (each ends usable)

> One phase per session. Stop after acceptance criteria pass. Conventional commits, tests for domain logic, `go vet` + `npm run lint && tsc --noEmit` gates.

---

## Phase 0 — Documentation & Design ✅ DONE

**Ships:** `docs/vision.md`, `spec.md`, `data-model.md`, `architecture.md`, `api-outline.md`, `mcp-tools.md`, `design-system.md`, `roadmap.md` (this file), `decisions.md` plus `.env.example` and README `Quick start`.

**Accepts:** all 9 docs exist, reference each other consistently, and reviewer can describe the four pillars + universal core without reading code.

---

## Phase 1 — Foundation ✅ DONE

**Scaffold:** exact monorepo layout (`services/api`, `services/scheduler` stub, `apps/web`, `apps/mcp` stub, `packages/sdk` stub, `deploy/`), root `go.mod` (`github.com/davinakmalyasha/PersonalOS`), root `package.json` workspaces, `.gitignore`, `data/` gitignored.

**Go API skeleton:** chi router, `GET /healthz` (DB ping) + `GET /openapi.json` (embedded spec), config via `.env` + env (`godotenv`), zerolog, SQLite (`mattn/go-sqlite3`, MinGW on Windows) with WAL+FK pragmas, goose embedded migrations (bootstrap only), typed Tests (config precedence, healthz handler, migrations up/down).

**Dashboard skeleton:** Next.js App Router + Tailwind + shadcn init + monochrome tokens (light/dark toggle, `next-themes`), Recharts installed but not yet charting. Minimal landing page.

**CI:** GitHub Actions running `go vet`, `go test`, `npm run lint && npx tsc --noEmit` on push.

**Accepts:** `go test ./...` + `go vet ./...` green; `curl /healthz` → `{"status":"ok"}`; `curl /openapi.json` valid JSON; `apps/web` builds and page renders with dark/light toggle.

---

## Phase 2 — Finance ⭐ ✅ DONE

**Schema:** `accounts`, `transactions` (amount minor units, date, merchant, raw_description, hash), `categories` (hierarchical), `categorization_rules` (pattern→category, priority), `budgets` (category+month).

**Features:** CSV import (multipart `POST /transactions/import`, column auto-detect, amount/date parsing, dedupe by `(date, amount, hash)`, import report `imported/skipped/errors`), rules engine (auto-categorize on import + manual override → "create rule", priority-ordered), CRUD + filtered list for all entities, summaries (`/finance/summary`, `/finance/spending`).

**Dashboard page:** month picker, summary cards (income/outcome/net), category breakdown chart, budget bars, transactions table (filter/search/pagination).

**Tests mandatory:** dedupe hash, pattern matching + priority, CSV sniff/parse, import idempotence.

**Accepts:** importing a real bank CSV twice → zero duplicates; manual spot-check sums match API responses; `go test ./...` covers dedupe + rules logic.

---

## Phase 3 — Planner (Tasks + Habits + Events) ✅ DONE

**Schema:** `tasks`, `habits` + `habit_checkoffs`, `events` (RRULE-lite).

**Features:** tasks CRUD + quick capture + filters + `PATCH status`; habits CRUD + toggle checkoff + streak calc; events CRUD + `GET /events?from=&to=` with recurrence expansion + `planner/today` + `planner/upcoming` + `planner/overview` bundle.

**Dashboard page:** month calendar + agenda + today column, tasks table, habit grid with streak/heatmap.

**Tests:** recurrence expansion (daily/weekly/monthly + COUNT/UNTIL), streak calc, habit toggle idempotence.

**Accepts:** weekly repeating event appears correctly for N weeks; habit streak counts match manual calc; `today` bundle aggregates three sources.

---

## Phase 4 — Knowledge (Notes + Bookmarks + Reading) ✅ DONE

**Schema:** `notes`, `bookmarks` (URL normalized, deduped), `reading_list`; mirrored rows to `items` for unified FTS.

**Features:** CRUD for each; `knowledge/search` + `knowledge/tags` cross-type; global `GET /search` across universal FTS; optional Google Books lookup on reading create.

**Dashboard page:** search-first knowledge page, tag filter, link affordance, reading kanban/status filter.

**Tests:** URL normalization, FTS tokenization, duplicate-URL idempotence.

**Accepts:** FTS search returns relevant notes/bookmarks/reading <100ms on 10k seeded rows; searching "personal OS" ranks the right note first.

---

## Phase 5 — Health (Food + Fitness/Body) ✅ DONE

**Schema:** `meals` (+ `items` JSON), `recipes` + `grocery_items`, `workouts` (+ `exercises` JSON), `body_metrics`.

**Features:** meal/recipe/workout CRUD; `POST /recipes/{id}/use` → meal; grocery list CRUD + `clear-checked`; body metrics upsert-by-day + series; `health/summary` + `health/weight-series`.

**Dashboard page:** weight trend line, workout frequency bars, recent meals/workouts, grocery checklist widget.

**Tests:** body-metric upsert (same day replaces), grocery toggle, recipe→meal copy.

**Accepts:** weight series endpoint returns correctly bucketed daily values; grocery clear-checked empties only `checked=true`.

---

## Phase 6 — MCP Layer ✅ DONE

**Implements:** `apps/mcp` server `personal-os` exposing the full tool catalog (33 tools, `docs/mcp-tools.md`) against the live Go API over stdio. Auth via `PERSONAL_OS_URL` + `PERSONAL_OS_TOKEN`. Published build `apps/mcp/dist/index.js`.

**Docs:** wiring blocks for opencode and Claude Desktop/Code; troubleshooting section (is the API up? is the token set?).

**Accepts:** from a real MCP client, asking natural language ("what's due today? how much did I spend on food this month? remember warranty X expires ...") produces correct API-backed answers — no hallucinated IDs. Verified by `scripts/mcp-smoke.mjs`: initialize → tools/list → planner_today / save_item→search_items / search_knowledge / check_habit all pass against a token-protected live API.

---

## Phase 7 — Polish & Deploy ✅ DONE

**Visual perfection:** every page chart-first on the monochrome token system (`hsl(var(--chart-*))` grey ramp); empty states everywhere; dark/light parity; focus rings via Tailwind defaults. Deferred stretch: page-level skeletons and a global `/`-search shortcut (knowledge search autofocuses instead).

**Deploy:** `services/api/Dockerfile` (multi-stage cgo build with `-tags sqlite_fts5`, slim runtime + sqlite3 CLI), `apps/web/Dockerfile`, `deploy/docker-compose.yml` with persistent named volume; `deploy/README.md` documents backups (`sqlite3 .backup`), restore, and a column-by-column Postgres portability audit.

**Accepts:** `docker compose -f deploy/docker-compose.yml up` boots both services; compose-mounted SQLite persists across restarts; README shows all 4 pillars monochrome.

---

## Versioning

`v0.1` = Phase 1 done. `v0.2` = finance. `v1.0` = Phase 7 done. Tag releases with conventional changelog.

## Risk mitigations

- SQLite cgo risk (Windows) → MinGW via winget, documented; CI uses same toolchain or `CGO_ENABLED` cache.
- Scope creep ("everything possible") → universal `items` core absorbs randomness; pillars stay bounded per this roadmap.
- Portfolio impression → every phase ships a screenshot-worthy page and `go test` proof, not just CRUD stubs.
