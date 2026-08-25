-- +goose Up
-- Health pillar: meals (journal), recipes + grocery list, workouts, body metrics.
-- JSON columns are TEXT with json_valid checks; Postgres maps them to jsonb.

CREATE TABLE meals (
    id         TEXT PRIMARY KEY,
    eaten_at   TEXT NOT NULL, -- RFC3339 UTC
    title      TEXT NOT NULL,
    notes      TEXT NOT NULL DEFAULT '',
    items      TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(items) AND json_type(items)='array'),
    calories   INTEGER CHECK(calories IS NULL OR calories >= 0),
    tags       TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_meals_eaten ON meals(eaten_at DESC);

CREATE TABLE recipes (
    id                   TEXT PRIMARY KEY,
    title                TEXT NOT NULL,
    ingredients          TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(ingredients) AND json_type(ingredients)='array'),
    instructions         TEXT NOT NULL DEFAULT '',
    servings             INTEGER CHECK(servings IS NULL OR servings > 0),
    calories_per_serving INTEGER CHECK(calories_per_serving IS NULL OR calories_per_serving >= 0),
    tags                 TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

CREATE TABLE grocery_items (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    qty        TEXT NOT NULL DEFAULT '',
    unit       TEXT,
    checked    INTEGER NOT NULL DEFAULT 0,
    recipe_id  TEXT REFERENCES recipes(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_grocery_checked ON grocery_items(checked);

CREATE TABLE workouts (
    id               TEXT PRIMARY KEY,
    performed_at     TEXT NOT NULL, -- RFC3339 UTC
    title            TEXT,
    notes            TEXT NOT NULL DEFAULT '',
    duration_minutes INTEGER CHECK(duration_minutes IS NULL OR duration_minutes >= 0),
    exercises        TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(exercises) AND json_type(exercises)='array'),
    tags             TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);
CREATE INDEX idx_workouts_performed ON workouts(performed_at DESC);

CREATE TABLE body_metrics (
    id           TEXT PRIMARY KEY,
    measured_at  TEXT NOT NULL, -- RFC3339 UTC; one row per calendar day (app-enforced upsert)
    weight_kg    REAL CHECK(weight_kg IS NULL OR weight_kg > 0),
    body_fat_pct REAL CHECK(body_fat_pct IS NULL OR (body_fat_pct > 0 AND body_fat_pct < 100)),
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_body_metrics_day ON body_metrics(substr(measured_at,1,10));

-- +goose Down
DROP TABLE IF EXISTS body_metrics;
DROP TABLE IF EXISTS workouts;
DROP TABLE IF EXISTS grocery_items;
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS meals;
