-- +goose Up
-- Agentic depth: goals, recurring tasks, habit schedules, water intake,
-- reading highlights, transfer flagging.

CREATE TABLE goals (
    id           TEXT PRIMARY KEY,
    kind         TEXT NOT NULL CHECK(kind IN ('savings','calorie')),
    name         TEXT NOT NULL,
    target_minor INTEGER, -- savings: target amount in minor units
    saved_minor  INTEGER NOT NULL DEFAULT 0,
    deadline     TEXT,    -- YYYY-MM-DD
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE INDEX idx_goals_kind ON goals(kind);

ALTER TABLE tasks ADD COLUMN recurrence_rule TEXT; -- RRULE-lite; completing spawns next
ALTER TABLE habits ADD COLUMN weekdays TEXT NOT NULL DEFAULT '1111111'
    CHECK(length(weekdays)=7 AND weekdays GLOB '*[01]*'); -- Mon..Sun, 1=scheduled
ALTER TABLE body_metrics ADD COLUMN water_ml INTEGER NOT NULL DEFAULT 0
    CHECK(water_ml >= 0);
ALTER TABLE reading_list ADD COLUMN highlights TEXT NOT NULL DEFAULT '[]'
    CHECK(json_valid(highlights) AND json_type(highlights)='array');
ALTER TABLE transactions ADD COLUMN is_transfer INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE transactions DROP COLUMN is_transfer;
ALTER TABLE reading_list DROP COLUMN highlights;
ALTER TABLE body_metrics DROP COLUMN water_ml;
ALTER TABLE habits DROP COLUMN weekdays;
ALTER TABLE tasks DROP COLUMN recurrence_rule;
DROP TABLE IF EXISTS goals;
