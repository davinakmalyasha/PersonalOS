-- +goose Up
-- Universal capture core (items + links + FTS5) and Knowledge pillar
-- (notes, bookmarks, reading_list). Knowledge rows mirror to items in the
-- same write transaction for unified search.

CREATE TABLE items (
    id             TEXT PRIMARY KEY,
    type           TEXT NOT NULL,
    title          TEXT NOT NULL,
    body           TEXT NOT NULL DEFAULT '',
    data           TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(data) AND json_type(data)='object'),
    tags           TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    source         TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','api','mcp','import','promotion')),
    source_item_id TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);
CREATE INDEX idx_items_type ON items(type);
CREATE INDEX idx_items_created ON items(created_at DESC);
CREATE INDEX idx_items_mirror ON items(type, source_item_id);

CREATE TABLE item_links (
    from_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    to_id      TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL DEFAULT 'related',
    created_at TEXT NOT NULL,
    PRIMARY KEY (from_id, to_id, kind)
);
CREATE INDEX idx_links_to ON item_links(to_id);

-- External-content FTS5 kept in sync by triggers.
CREATE VIRTUAL TABLE items_fts USING fts5(
    title, body, tags,
    content='items', content_rowid='rowid',
    tokenize='porter unicode61'
);

-- +goose StatementBegin
CREATE TRIGGER items_fts_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, title, body, tags)
    VALUES (new.rowid, new.title, new.body, new.tags);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_fts_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, title, body, tags)
    VALUES ('delete', old.rowid, old.title, old.body, old.tags);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_fts_au AFTER UPDATE OF title, body, tags ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, title, body, tags)
    VALUES ('delete', old.rowid, old.title, old.body, old.tags);
    INSERT INTO items_fts(rowid, title, body, tags)
    VALUES (new.rowid, new.title, new.body, new.tags);
END;
-- +goose StatementEnd

CREATE TABLE notes (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    pinned      INTEGER NOT NULL DEFAULT 0,
    archived_at TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE INDEX idx_notes_pinned ON notes(pinned);

CREATE TABLE bookmarks (
    id          TEXT PRIMARY KEY,
    url         TEXT NOT NULL UNIQUE, -- normalized canonical URL
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE reading_list (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    author      TEXT,
    url         TEXT,
    status      TEXT NOT NULL DEFAULT 'to-read' CHECK(status IN ('to-read','reading','done')),
    rating      INTEGER CHECK(rating IS NULL OR rating BETWEEN 1 AND 5),
    notes       TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    finished_at TEXT
);
CREATE INDEX idx_reading_status ON reading_list(status);

-- +goose Down
DROP TABLE IF EXISTS reading_list;
DROP TABLE IF EXISTS bookmarks;
DROP TABLE IF EXISTS notes;
DROP TRIGGER IF EXISTS items_fts_ai;
DROP TRIGGER IF EXISTS items_fts_ad;
DROP TRIGGER IF EXISTS items_fts_au;
DROP TABLE IF EXISTS items_fts;
DROP TABLE IF EXISTS item_links;
DROP TABLE IF EXISTS items;
