# Decisions — ADRs

Living list. Numbered, timestamped, one decision per entry. If a decision is reversed, append a new ADR that supersedes it — don't edit history.

---

## ADR-0001 — SQLite driver: `mattn/go-sqlite3` (cgo)

- **Date:** 2026-08-25
- **Decision:** use `mattn/go-sqlite3` rather than `modernc.org/sqlite`.
- **Why:** user preference; `mattn` is battle-tested and widely documented. Cost is a C toolchain (MinGW-w64 via winget `BrechtSanders.WinLibs.POSIX.UCRT`, `CGO_ENABLED=1`). On this Windows box we installed it to `.../WinGet/Packages/.../mingw64/bin/gcc.exe` and added to User PATH.
- **Consequence:** contributors on Windows must install MinGW (one `winget install`); CI must provision gcc or cache cgo build. Pure-Go fallback remains an ADR-level revert if cgo becomes a blocker.

## ADR-0002 — Single root Go module

- **Date:** 2026-08-25
- **Decision:** one `go.mod` at repo root with `module github.com/davinakmalyasha/PersonalOS` covering `services/*`.
- **Why:** `go test ./...` at root covers all services, import paths stay simple (`github.com/davinakmalyasha/PersonalOS/services/api/internal/...`), and module overhead stays minimal for a local-first monorepo.
- **Consequence:** versioning is repo-wide. Split into per-service modules only if cross-service boundaries harden.

## ADR-0003 — OpenAPI served from embedded JSON

- **Date:** 2026-08-25
- **Decision:** hand-maintained `services/api/internal/server/openapi.json` embedded via `go:embed` and served at `GET /openapi.json`; always kept in sync with handlers. Generation from code annotations is a later ADR.
- **Why:** smallest moving part that still gives a single source for `packages/sdk` generation and MCP wiring. Avoids annotation tooling before shapes stabilize.
- **Consequence:** PR checklist must include "did you update openapi.json?" until generation is adopted.

## ADR-0004 — `godotenv` for `.env` + env config

- **Date:** 2026-08-25
- **Decision:** env loading via `joho/godotenv` (`.env` if present, then real env overrides).
- **Why:** matches handoff ("env + `.env` file"), trivial, keeps `internal/config` testable (precedence test).
- **Consequence:** no complex config hierarchy; cloud secrets stay via real env, not file.

## ADR-0005 — Monorepo layout verbatim from handoff

- **Date:** 2026-08-25
- **Decision:** keep `services/api`, `services/scheduler`, `apps/web`, `apps/mcp`, `packages/sdk`, `deploy` exactly as specified.
- **Why:** matches handoff expectations, clear bounded contexts, and portfolio clarity.

## ADR-0006 — Object core + pillar overlays (not pure modules, not pure freeform)

- **Date:** 2026-08-25
- **Decision:** a universal `items` table (type, title, body, data JSON, tags, links) + FTS absorbs any random personal data. Pillars (Finance, Planner, Knowledge, Health) are typed overlays with real columns for analytics — mirrored to `items` where it keeps search unified. Promotion from item → typed record is supported.
- **Why:** resolves "personal data is so many and random" without sacrificing money math / streaks / budgets which demand typed columns. Generic MCP verbs (`save_item`/`search_items`) cover 100% of future data on day one.
- **Consequence:** write paths for knowledge/health mirror to `items` transactionally. FTS lives on `items_fts`, not per-table.

## ADR-0007 — Merged pillar boundaries

- **Date:** 2026-08-25
- **Decision:** merge Food+Fitness→Health, Notes+Bookmarks→Knowledge (incl. Reading), Planner→Tasks+Habits+Events. Finance stays standalone.
- **Why:** per user direction; reduces 7 thin pages to 4 strong ones with better chart density and fewer contexts.
- **Consequence:** APIs grouped accordingly (`/health`, `/knowledge`, `/planner`) but underlying tables remain granular for typed guarantees.

## ADR-0008 — Dropped observatory + intel pipeline

- **Date:** 2026-08-25
- **Decision:** remove the "Agent Observatory" (token/cost ingestion) and the HN/arxiv brief scheduler. `services/scheduler` remains as a stub for a future stretch; no observatory routes.
- **Why:** user does not value token/cost tracking; focus is personal-data coverage + agent data access.
- **Consequence:** token/cost analytics are out of scope unless re-introduced via ADR.

## ADR-0009 — Monochrome theme, dark/light

- **Date:** 2026-08-25
- **Decision:** app-wide monochrome (zinc/gray scale, border-driven hierarchy) with light/dark via CSS variables + `next-themes` class toggle. Only permitted color is a muted destructive red for over-budget/delete. Recharts palette is a 5-step grey ramp.
- **Why:** per user request; editorial feel, portfolio-distinctive, avoids rainbow dashboards.
- **Consequence:** component work must use `hsl(var(--...))` tokens, never hardcoded `#fff`/`#000` or Recharts defaults.

## ADR-0010 — Package manager: npm workspaces

- **Date:** 2026-08-25
- **Decision:** `npm` workspaces (`apps/*`, `packages/*`) rather than pnpm.
- **Why:** matches verification command in handoff (`npm run lint && npx tsc --noEmit`), zero extra toolchain, sufficient for this scale.

## ADR-0011 — Docs-first phase

- **Date:** 2026-08-25
- **Decision:** produce the full docs suite (`vision`/`spec`/`data-model`/`architecture`/`api-outline`/`mcp-tools`/`design-system`/`roadmap`/`decisions`) and commit it before any feature code beyond the foundation skeleton, because the project is "big, impactful, perfect".
- **Why:** user explicitly requested documents first; premature code without agreed scope drifts.
- **Consequence:** Phase 1 skeleton must conform to these docs; divergence is a bug filed against code, not docs.

## ADR-0012 — ID strategy: ULID

- **Date:** 2026-08-25
- **Decision:** ULIDs (`oklog/ulid`) for all primary keys, app-generated.
- **Why:** lexicographically sortable, no AUTOINCREMENT portability quirks, works as `TEXT PK` in SQLite and `text` in Postgres without sequence gymnastics.
- **Consequence:** app must generate IDs; tests use deterministic entropy.

## ADR-0013 — Finance store: hand-written typed queries, sqlc deferred

- **Date:** 2026-08-25
- **Decision:** Phase 2 ships a hand-written `internal/store` package (parameterized SQL, struct scans, sentinel errors `ErrNotFound/ErrConflict/ErrInvalid`) instead of generating code with sqlc. Revisit sqlc when the query surface stabilizes after Planner/Knowledge land.
- **Why:** the finance query surface is small and stable enough to hand-write safely with full test coverage; adding a codegen toolchain step mid-phase risked momentum without changing observable behavior. All SQL remains Postgres-portable.
- **Consequence:** `sqlc.yaml` is not present yet; ADR-0002's module layout unaffected. When adopted, generated queries replace store methods one pillar at a time behind the same handler contracts.

## ADR-0014 — Per-service Go modules (supersedes ADR-0002)

- **Date:** 2026-08-25
- **Decision:** move `go.mod` into `services/api` (`module github.com/davinakmalyasha/PersonalOS/services/api`). Future Go services (e.g., scheduler) get their own module. JS apps are self-contained installs (no npm workspaces); root `package.json` keeps only convenience scripts.
- **Why:** with a single root module, the Go toolchain traversed `apps/web/node_modules` (a dependency ships a stray Go package, `flatted/golang/...`), polluting `go build ./...`. Per-service modules give each language a clean tree; import paths of all existing packages are unchanged relative to their module.
- **Consequence:** run Go commands from `services/api` (README updated); CI sets `working-directory: services/api`. ADR-0002 is superseded, not deleted.

## ADR-0015 — Recurrence expanded at read time, RRULE-lite semantics

- **Date:** 2026-08-25
- **Decision:** events store one row with an optional `recurrence_rule` (`FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=YYYYMMDD`, unknown tokens → 400). `GET /v1/events?from=&to=` expands occurrences in Go at read time — no materialized instance rows. Semantics: each occurrence derives from the ORIGINAL start (monthly clamping never compounds: Jan 31 → Feb 28 → Mar 31); COUNT includes the original occurrence; UNTIL is inclusive by calendar day; a safety cap of ~5y daily bounds UNTIL-less/COUNT-less rules. All day math is UTC.
- **Why:** personal-scale data makes read-time expansion cheap; avoids sync drift between series definition and instances; editing = whole-series edit (per-occurrence overrides are out of scope v1).
- **Consequence:** per-occurrence edits/cancellations need a future `event_overrides` table + ADR; expansion cost is bounded by COUNT/UNTIL/window.

## ADR-0016 — Habit streak semantics

- **Date:** 2026-08-25
- **Decision:** daily habits: current streak = consecutive checked days ending today, with yesterday-grace (today unchecked still counts the run ending yesterday). Weekly habits: current streak = consecutive target-meeting ISO weeks (Mon–Sun), same grace for the in-progress week; `week_done` tracks progress vs `target_per_week`. Longest streak uses identical qualification rules over full history.
- **Why:** grace prevents the demotivating "streak reset every morning" artifact while staying honest; weekly targets map to real habit science (X times per week).
- **Consequence:** streak math lives pure in `domain/planner` (unit-tested); store attaches it on habit reads.

---

Template for the next ADR:

```
## ADR-00NN — Title

- **Date:** YYYY-MM-DD
- **Decision:** ...
- **Why:** ...
- **Consequence:** ...
```
