-- +goose Up
-- Planner pillar: tasks, habits + checkoffs, events (RRULE-lite, expanded at read time).

CREATE TABLE tasks (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'todo' CHECK(status IN ('todo','doing','done')),
    priority     TEXT NOT NULL DEFAULT 'med' CHECK(priority IN ('low','med','high')),
    due_date     TEXT,             -- YYYY-MM-DD
    project      TEXT,
    tags         TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    completed_at TEXT
);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_due ON tasks(due_date);
CREATE INDEX idx_tasks_priority ON tasks(priority);
CREATE INDEX idx_tasks_project ON tasks(project);

CREATE TABLE habits (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    cadence         TEXT NOT NULL DEFAULT 'daily' CHECK(cadence IN ('daily','weekly')),
    target_per_week INTEGER NOT NULL DEFAULT 7 CHECK(target_per_week BETWEEN 1 AND 7),
    color           TEXT,
    created_at      TEXT NOT NULL,
    archived_at     TEXT
);
CREATE INDEX idx_habits_archived ON habits(archived_at);

CREATE TABLE habit_checkoffs (
    id         TEXT PRIMARY KEY,
    habit_id   TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    date       TEXT NOT NULL,      -- YYYY-MM-DD
    created_at TEXT NOT NULL,
    UNIQUE(habit_id, date)
);
CREATE INDEX idx_checkoffs_habit_date ON habit_checkoffs(habit_id, date DESC);

CREATE TABLE events (
    id              TEXT PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    starts_at       TEXT NOT NULL, -- RFC3339 UTC
    ends_at         TEXT,
    location        TEXT NOT NULL DEFAULT '',
    recurrence_rule TEXT,          -- FREQ=DAILY|WEEKLY|MONTHLY;INTERVAL=n;COUNT=n|UNTIL=YYYYMMDD
    tags            TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX idx_events_starts ON events(starts_at);

-- +goose Down
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS habit_checkoffs;
DROP TABLE IF EXISTS habits;
DROP TABLE IF EXISTS tasks;
