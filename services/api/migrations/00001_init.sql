-- +goose Up
-- Foundation bootstrap migration.
-- Creates a minimal meta table so goose has something to migrate.
-- Real pillar tables land in Phase 2+.

CREATE TABLE IF NOT EXISTS app_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO app_meta (key, value, updated_at)
VALUES ('schema_version', '00001', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

-- +goose Down
DROP TABLE IF EXISTS app_meta;
