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

## Phase 8 — Live Board ✅ DONE

**Concept:** the dashboard is a component package, not a website. `apps/web` is the reference host; `components/*` are the pull-and-embed units.

**Ships:** `GET /v1/activity` (per-pillar latest-change timestamps); `ActivityProvider` polling every 4s (paused on hidden tab); neutral top-bar shell with **◉ live dot**; `/` rewritten as a **bento board** — preview tiles, click → tile grows into a full detail card while siblings dim/scale behind (stacked-cards); `?expand=<tile>` deep-links + `personal-os:focus` CustomEvents let host apps/agents drive expansion; activity writes flash the touched pillar's tile. Landing page deleted.

**Accepts:** activity timestamps move on writes; board tiles re-fetch only when their pillar's version changes; expansion/collapse animates smoothly; zero marketing residue.

---

## Phase 9 — Agentic Depth ✅ DONE

**Features (reference-grounded: Monarch/Rocket Money, Habitify/Loop, MFP/Hevy, Omnivore/Readwise):**

- Finance: `goals` (savings w/ progress + single calorie target), `/finance/recurring` subscription detection, transfer pairing (same-day opposite cross-account → `is_transfer`, excluded from spend summaries)
- Planner: recurring tasks (RRULE-lite on tasks; completing spawns the next instance, UNTIL-respected), habit weekday schedules (`weekdays` Mon-first mask, due-logic respects it), rolling 30-day consistency % alongside streaks
- Health: water intake (daily ml on the body-metrics row + quick log), exercise PRs (heaviest set per exercise), calories-today vs calorie goal in summary
- Knowledge: reading highlights (`[{quote, note?, at}]`)
- Universal: `/items/expiring` scans data-JSON date fields for upcoming warranties/renewals

**MCP:** `manage_goals`, `find_recurring`, `log_water`, `exercise_prs`, `upcoming_expiries` added (38 tools total).

**Accepts:** recurring heuristic detects 3× monthly Netflix; transfer pair nets zero in summary; completing a weekly recurring task spawns due+7d; weekday mask suppresses unscheduled days; PRs reflect heaviest set; expiry scan finds warranty inside horizon.

---

## Phase 10 - Intelligence & Memory (10a+10b+10c DONE, 11 next)

**10a - Finance + Planner depth (DONE):**

- Finance: `/finance/net-worth` (cumulative balances derived from transactions), upcoming bills from recurring detection surfaced in Today + MCP, merchant aliases (pattern->canonical, UNIQUE) applied on create/import, budget rollover (prior-month unused flows forward when flagged), per-account import profiles in `accounts.settings`
- Planner: task subtasks (`parent_id`, tree in list), `blocked_by` (Today flags blocked items), `estimate_minutes` -> day-load minutes in Today bundle, event exceptions (`POST /events/{id}/exceptions` closes the ADR-0015 gap), habit `paused_until` respected by due-logic + consistency
- MCP: `net_worth`, `upcoming_bills`, `manage_merchants`, `set_event_exception`, `review_week`

**Accepts:** net-worth series matches manual cumulative sums; bills strip shows subscription due within 7d; alias rewrites noisy merchant on import; rollover budget carries unused prior-month amount; completing a subtask parent works independently; exception cancels one occurrence of a series.

**10b - Memory & findability (DONE):**

- Migration 00008: `changelog(entity, entity_id, action, title, at)`, `saved_searches`, `items.pinned`, `items.archived_at`
- Changelog written app-level on every pillar mutation -> `GET /activity/feed` (+ entity filter) + MCP `activity_feed`
- Search v2: `GET /v1/search` returns ranked items (pinned first, archived hidden) UNION typed scans of tasks/meals/workouts/transactions - one "find anything" call; `items` array kept for back-compat
- Saved searches CRUD + run (`POST /saved_searches/{id}/run`); daily note get-or-create + append (`/knowledge/daily`); on-this-day resurface (`/knowledge/resurface`); pin/archive on items; `/v1/export` full JSON dump; backlinks data on `/items/{id}/links`
- MCP: `activity_feed`, `manage_saved_searches`, `daily_note`, `resurface_memory`, `backlinks`, `export_data` added (49 tools total)

**Accepts:** archiving an item hides it from lists/search until restored; pinned sorts first; feed logs create/update/complete/delete per entity with filter; saved-search run reproduces its query; second daily-note GET reuses the same note and append adds a bullet; resurface finds last year's same-day capture; export contains every table with rows.

**10c - Health macros + full wiring (DONE):**

- Migration 00009: `meals.protein_g/carbs_g/fat_g`, `body_metrics.measurements` JSON (free-form {key: number}), singleton `health_settings` (macro/water targets + weekly workout target)
- Meal macros + daily targets -> summary rings (`macros` + `settings` on /health/summary); weekly tonnage `/health/volume` (weight x reps per exercise, case-insensitive); measurement trends `/body-metrics/trends` (per-key ascending series); settings GET/PUT+PATCH with merge semantics
- UI wiring: Activity Feed tile on the board (what did my agent just do), bills strip in Money detail, goals/subscriptions/aliases panels on Finance, weekday chips + subtask indent + recurring/blocked badges + day-load minutes on Planner, daily-note + on-this-day resurface cards on Knowledge (backlinks panel already inline in search), macro rings + PR table + water button + tonnage + measurement trends on Health
- MCP: `manage_health_settings`, `workout_volume`, `measurement_trends` + macros on `log_meal` added (52 tools total)

**Accepts:** logging two meals rolls macro totals into the rings against PUT targets; volume aggregates same exercise case-insensitively sorted by tonnage; trends return chest/waist series ascending; settings merge keeps untouched fields; board activity tile updates as agents write.

---
## Phase 11 - Scheduler (DONE)

**Features:**

- `services/scheduler` (own Go module, pure Go): interval loop (`SCHED_INTERVAL_SECONDS`, default 6h) with optional nightly gating (`SCHED_RUN_HOUR` UTC; one pass/day, deduped) and optional immediate pass on boot
- Nightly jobs over HTTP (no direct DB access, same rule as MCP): transfer pairing re-run, subscription recompute (+ bills due <=7d), expiring-items digest <=30d, budget-over check for the current month
- Findings render as a text digest and fan out via Telegram bot API and/or Discord webhook (`TELEGRAM_*` / `DISCORD_WEBHOOK_URL`); quiet passes log only
- Compose entry (`scheduler` service, depends on api) + Dockerfile; env template documents all knobs

**Accepts:** fixture-API tests prove pairing count, Netflix-only bill selection within 7d, over-budget filtering, bearer token forwarding and 5xx surfacing; nightly gate fires exactly once per day at the configured UTC hour; Discord/Telegram payloads captured by httptest servers.

---
## Phase 12a - Finance depth (DONE)

**Features (reference-grounded: Monarch recurring review, Simplifi projected cash flow, PocketGuard safe-to-spend):**

- Managed subscriptions: detection upserts into a `subscriptions` table (UNIQUE merchant+amount) without clobbering lifecycle; confirm/mute/cancel via PATCH; manual bill entry (e.g. rent paid offline); `POST /finance/subscriptions/sync`
- Safe-to-spend (`GET /finance/safe-to-spend`): income MTD - spend MTD - unspent budgets (rollover-aware) - active subs still due this month
- Cash-flow forecast (`GET /finance/forecast?days&alert_below`): projected daily balance from current balances + active subscription charges + trailing-90d avg net flow; lowest point + alert flag
- Scheduler low-balance nudge (`SCHED_LOW_BALANCE_MINOR` / `SCHED_LOW_BALANCE_DAYS`)
- Rules v2: optional amount window (bounds on |amount|) on rules + `POST /rules/{id}/apply` backfill re-categorizes matching history
- Transaction tags (create/update/filter); accounts gain `kind` asset|liability + `opening_balance_minor` -> net worth now assets-vs-liabilities seeded from openings
- `GET /transactions/export.csv` portability export
- MCP: `manage_subscriptions`, `safe_to_spend`, `cashflow_forecast`, `recategorize_history` added (56 tools total)

**Accepts:** three Netflix charges -> sync creates one active sub, second sync is idempotent; muting hides it from status=active; forecast projects the rent charge landing on its due date and alerts below threshold; safe-to-spend math matches components; backfill moves only rows inside the rule's amount window; liability account subtracts from net.

---
## Phase 12b - Planner depth (DONE)

**Features (reference-grounded: Todoist natural-language dates, TickTick calendar, Habitify measurable habits):**

- Server-side natural-language date parser (`domain/planner/nldate.go`): today/tomorrow/next week/next month, weekday names, `in N days|weeks|months`, `27 aug`/`aug 27`, day-first slash dates, optional `@17:00` / `at 7pm` times -> `GET /planner/parse-date`
- Tasks gain `due_time` (HH:MM, validated, clearable) for time-of-day scheduling
- Recurring lineage: new instances carry `series_id` (seeded on first create); `POST /tasks/{id}/skip` advances a recurring task without completing it
- Measurable habits: checkoffs accept `{value, note}` (upsert semantics); presence-based streaks unchanged; toggle still available
- `GET /planner/calendar.ics?days=90` read-only iCalendar feed of events + dated open tasks
- MCP: `parse_date`, `skip_task`, `delete_task` added; `create_task`/`update_task` expose recurrence/subtask/blocked/estimate/due_time; `check_habit` takes value/note (59 tools total)
- Web: NL dates in task quick-add (type "fri" as the date), skip button + due-time chips in the tasks table, .ics feed link

**Accepts:** "tomorrow at 7pm" parses to date+19:00+iso; invalid HH:MM rejected on create and patch; completing a recurring task spawns a same-series instance; skip leaves exactly one open instance in the series; measurable checkoff marks done without breaking the day list; ICS feed contains VEVENTs.

---
## Phase 12c - Health depth (DONE)

**Features (reference-grounded: Hevy exercise library/routines/1RM, MyFitnessPal food DB, Cronometer):**

- Seeded **exercise library** (`exercises`: name/muscle_group/equipment; 33 movements) with search + taxonomy filters — logged sets can now be normalized against it
- **Routines** (workout templates): CRUD with ordered `routine_exercises` (sets + target reps); `POST /routines/{id}/start` copies the template into a fresh workout
- **Personal food database**: upsert-by-name with merge semantics (0/empty keeps stored values); `POST /foods/{id}/log {servings}` creates a meal with scaled calories/macros
- Meals gain a **slot** (breakfast|lunch|dinner|snack); recipes now carry macros into meals correctly via the widened CreateMeal path
- PRs gain **estimated 1RM** (Epley) tracked alongside max weight; summary computes **weekly workout-target progress** (`week_workouts_done/target/pct`, Mon-based) and carries **goal_weight_kg**
- `GET /health/macros-series` per-day macro history for ring trends
- MCP: `exercise_library`, `manage_routines`, `manage_foods` added (62 tools total)
- Web: inline Targets & goals editor on the health page

**Accepts:** squat search returns 3 seeded variants with taxonomy; starting Push A materializes 6 sets in one workout; logging 1.5 servings of Nasi Ayam yields 975 kcal / 63g protein / slot=lunch; partial food upserts do not wipe macros; weekly progress shows 1/4 = 25% after one workout.

---
## Versioning

`v0.1` = Phase 1 done. `v0.2` = finance. `v1.0` = Phase 7 done. `v1.1` = live board + agentic depth (Phases 8–9).

## Risk mitigations

- SQLite cgo risk (Windows) → MinGW via winget, documented; CI uses same toolchain or `CGO_ENABLED` cache.
- Scope creep ("everything possible") → universal `items` core absorbs randomness; pillars stay bounded per this roadmap.
- Portfolio impression → every phase ships a screenshot-worthy page and `go test` proof, not just CRUD stubs.
