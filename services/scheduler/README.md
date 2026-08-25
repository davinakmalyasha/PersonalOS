# Scheduler

Placeholder for future background jobs (stretch). Planned jobs:

- Daily digest (planner overview + health rollup) — webhook optional via `TELEGRAM_BOT_TOKEN` / `DISCORD_WEBHOOK_URL` from env.
- `items` FTS reindex verification.

Kept minimal in Phase 1 to stay focused on the API + dashboard. See `docs/roadmap.md` Phase 7.

Run as a separate binary `services/scheduler/cmd/scheduler` when introduced — shares the same SQLite file via `DB_PATH`, never concurrent writes without WAL.
