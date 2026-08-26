-- +goose Up
-- Phase 12b: planner depth — task time-of-day, recurring series lineage,
-- measurable habit entries.

ALTER TABLE tasks ADD COLUMN due_time TEXT; -- 'HH:MM' or NULL
ALTER TABLE tasks ADD COLUMN series_id TEXT; -- links spawned instances of one recurring series
CREATE INDEX idx_tasks_series ON tasks(series_id);

ALTER TABLE habit_checkoffs ADD COLUMN value REAL;    -- measurable habits (e.g. 8 glasses)
ALTER TABLE habit_checkoffs ADD COLUMN note  TEXT NOT NULL DEFAULT '';

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_series;
ALTER TABLE tasks DROP COLUMN series_id;
ALTER TABLE tasks DROP COLUMN due_time;
ALTER TABLE habit_checkoffs DROP COLUMN note;
ALTER TABLE habit_checkoffs DROP COLUMN value;
