# Vision — Personal OS

## Thesis

Personal OS is a **local-first, agent-native life platform**: one SQLite backbone, one Go API, one dashboard, and an MCP toolbelt so *any* external agent can read/write your real data on *your* machine. It merges the best of standalone personal apps (finance, planner, knowledge, health) into a single coherent system where everything is linkable, searchable, and automatable.

### Why this, why now

You run many coding/general AI agents. Today each agent is stateless, blind to your life data, and unobserved. Connecting them to personal context requires rebuilding glue every time. Personal OS fixes three problems at once:

1. **Your data is fragmented** — bank CSVs in Downloads, tasks in chat scrollback, notes in 3 apps.
2. **Agents cannot act on your life** — they lack tools to query or mutate real data.
3. **No shared memory** — every session re-explains context; random life data has nowhere to land.

Personal OS becomes the **data + observability layer your agents plug into**: one auth token, one API, one dashboard.

## Principles

1. **Local-first, you own the file.** Single-user, bearer-token auth now. One `./data/personal-os.db` you can `.backup`, copy, or move to Postgres later. No credential scraping (CSV import only for banks).
2. **Go owns the database.** TypeScript apps never touch SQLite directly. Typed queries (`sqlc`), versioned migrations (`goose`), WAL + foreign keys. All guarantees live in Go.
3. **Agent-native from day one.** Every mutation is an API endpoint *and* an MCP tool. Generic verbs (`save_item`/`search_items`) cover 100% of data immediately; pillar power-tools add precision where analytics demand it.
4. **Perfect beats shipped-fast.** Each phase ends usable and is shippable to a portfolio screenshot: tested domain logic, lint-clean, monochrome-visual, docs-matching-code.
5. **Coverage without sprawl.** Four strong pillars + a universal capture core absorb "everything possible" without inventing 30 tables on day one. Random data is first-class; promotion to a typed record is one call.
6. **Cloud-migration path stays open.** Env-driven config, Postgres-portable SQL, `deploy/` compose. No SQLite-only tricks that block moving later.

## Target audiences

- **Primary:** You — an AI developer who lives in agents and wants a single ledger for money, time, ideas, and body.
- **Secondary:** Portfolio reviewers — the codebase demonstrates mature choices: `sqlc` type safety, `goose` migrations, generated SDK, MCP correctness, tested dedupe/rules engines, clean monochrome UI.
- **Tertiary:** Any future local AI agent you build or adopt — one config block gives it full personal context.

## Non-goals (explicitly out of scope)

- Multi-user / team features, roles, sharing.
- Real bank syncing / credential scraping. CSVs only.
- Full calendar sync (Google/Outlook) — events are native; ICS import is a later stretch.
- Nutrition database licensing / barcode scanning — meals are freeform log + recipes.
- Social / publishing features.

## What success looks like

**For your life:** you naturally capture in Personal OS instead of scattered apps; asking any agent "what did I spend / what is due today / log this workout" works the first time; weekly reviews take minutes because charts are ready.

**For your portfolio:** a reviewer opens the repo and sees — in 60 seconds — a rooted vision doc, a typed Go API with migrated SQLite, a monochrome dashboard that actually renders data, MCP wiring docs that validate in a real client, and a test suite that proves dedupe + rules logic — not a todo-app skeleton.

## Narrative arc

```
Phase 0  Documentation — decisions recorded, not guessed later
Phase 1  Foundation    — skeleton that already answers /healthz + /openapi.json
Phase 2  Finance ⭐    — the hard domain (CSV, dedupe, rules) done right
Phase 3  Planner       — time, tasks, habits, events — the daily loop
Phase 4  Knowledge     — capture + FTS, the long-term memory
Phase 5  Health        — food + fitness, the body ledger
Phase 6  MCP           — one server exposes everything; agents connect in 5 lines
Phase 7  Polish/Deploy — monochrome perfection, docker-compose, backup, README that sells it
```

No observatory — dropped per direction. Intel pipeline (HN/arxiv briefs) also dropped. Focus stays on personal data you can see/modify.

## Constraints kept from the original handoff

- Tech stack locked: Go 1.22+ / chi / sqlc / goose / zerolog; Next.js App Router / Tailwind / shadcn / Recharts; TS MCP via `@modelcontextprotocol/sdk`.
- Monorepo layout exactly as handoff: `services/api`, `services/scheduler` (kept minimal for Phase 6 stretch), `apps/web`, `apps/mcp`, `packages/sdk`, `deploy`.
- Conventional commits, tests for domain logic, `go vet` + `tsc --noEmit` + `lint` gates, Postgres-portable SQL.

---

*This document is the source of truth. Code that contradicts it is a bug in the code.*
