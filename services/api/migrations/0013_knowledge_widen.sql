-- +goose Up
-- Phase 12d: knowledge depth — first-class highlights with spaced-repetition
-- review scheduling.

CREATE TABLE highlights (
    id             TEXT PRIMARY KEY,
    reading_id     TEXT NOT NULL REFERENCES reading_list(id) ON DELETE CASCADE,
    quote          TEXT NOT NULL,
    note           TEXT NOT NULL DEFAULT '',
    location       TEXT NOT NULL DEFAULT '',
    review_count   INTEGER NOT NULL DEFAULT 0,
    interval_days  INTEGER NOT NULL DEFAULT 0, -- 0 = due now
    last_reviewed  TEXT,
    created_at     TEXT NOT NULL
);
CREATE INDEX idx_highlights_reading ON highlights(reading_id);

-- Spaced-repetition pointer (SM-2-lite): NULL = never scheduled = due now.
ALTER TABLE highlights ADD COLUMN next_review_at TEXT;
CREATE INDEX idx_highlights_next ON highlights(next_review_at);

-- +goose Down
DROP TABLE IF EXISTS highlights;
