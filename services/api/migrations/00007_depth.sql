-- +goose Up
-- Phase 10a: finance intelligence (aliases, import profiles, rollover) and
-- planner depth (subtasks, dependencies, estimates, event exceptions, pause).

CREATE TABLE merchant_aliases (
    id         TEXT PRIMARY KEY,
    pattern    TEXT NOT NULL UNIQUE, -- substring match, case-insensitive
    canonical  TEXT NOT NULL,
    created_at TEXT NOT NULL
);

ALTER TABLE accounts ADD COLUMN settings TEXT NOT NULL DEFAULT '{}'
    CHECK(json_valid(settings) AND json_type(settings)='object');

ALTER TABLE budgets ADD COLUMN rollover INTEGER NOT NULL DEFAULT 0;

ALTER TABLE tasks ADD COLUMN parent_id TEXT REFERENCES tasks(id) ON DELETE CASCADE;
ALTER TABLE tasks ADD COLUMN blocked_by TEXT REFERENCES tasks(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN estimate_minutes INTEGER
    CHECK(estimate_minutes IS NULL OR estimate_minutes >= 0);
CREATE INDEX idx_tasks_parent ON tasks(parent_id);

CREATE TABLE event_overrides (
    id         TEXT PRIMARY KEY,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    date       TEXT NOT NULL, -- YYYY-MM-DD occurrence being overridden
    action     TEXT NOT NULL CHECK(action IN ('cancel','edit')),
    title      TEXT,
    starts_at  TEXT,
    ends_at    TEXT,
    location   TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(event_id, date)
);

ALTER TABLE habits ADD COLUMN paused_until TEXT; -- YYYY-MM-DD; not due while paused

-- +goose Down
ALTER TABLE habits DROP COLUMN paused_until;
DROP TABLE IF EXISTS event_overrides;
DROP INDEX IF EXISTS idx_tasks_parent;
ALTER TABLE tasks DROP COLUMN estimate_minutes;
ALTER TABLE tasks DROP COLUMN blocked_by;
ALTER TABLE tasks DROP COLUMN parent_id;
ALTER TABLE budgets DROP COLUMN rollover;
ALTER TABLE accounts DROP COLUMN settings;
DROP TABLE IF EXISTS merchant_aliases;
