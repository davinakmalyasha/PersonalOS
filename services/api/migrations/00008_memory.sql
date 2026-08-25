-- +goose Up
-- Phase 10b: changelog (agent activity feed), saved searches, pin/archive.

CREATE TABLE changelog (
    id        TEXT PRIMARY KEY,
    entity    TEXT NOT NULL, -- transaction|task|habit|event|note|bookmark|reading|item|meal|workout|goal|…
    entity_id TEXT NOT NULL,
    action    TEXT NOT NULL CHECK(action IN ('create','update','delete','complete')),
    title     TEXT NOT NULL DEFAULT '',
    at        TEXT NOT NULL
);
CREATE INDEX idx_changelog_at ON changelog(at DESC);

CREATE TABLE saved_searches (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    query      TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(query) AND json_type(query)='object'),
    created_at TEXT NOT NULL
);

ALTER TABLE items ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_items_pinned ON items(pinned);

-- +goose Down
DROP INDEX IF EXISTS idx_items_pinned;
ALTER TABLE items DROP COLUMN archived;
ALTER TABLE items DROP COLUMN pinned;
DROP TABLE IF EXISTS saved_searches;
DROP TABLE IF EXISTS changelog;
