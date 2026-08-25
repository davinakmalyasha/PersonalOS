-- +goose Up
-- Finance pillar: accounts, hierarchical categories, transactions (dedupe key),
-- categorization rules, monthly budgets.

CREATE TABLE accounts (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    type       TEXT NOT NULL CHECK(type IN ('checking','savings','cash','card','wallet')),
    currency   TEXT NOT NULL DEFAULT 'IDR',
    created_at TEXT NOT NULL
);

CREATE TABLE categories (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    parent_id  TEXT REFERENCES categories(id) ON DELETE RESTRICT,
    color      TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_categories_parent ON categories(parent_id);

CREATE TABLE transactions (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount          INTEGER NOT NULL, -- minor units; spend negative, income positive
    currency        TEXT NOT NULL DEFAULT 'IDR',
    date            TEXT NOT NULL,    -- YYYY-MM-DD
    merchant        TEXT NOT NULL DEFAULT '',
    raw_description TEXT NOT NULL DEFAULT '',
    category_id     TEXT REFERENCES categories(id) ON DELETE SET NULL,
    hash            TEXT NOT NULL,    -- sha256(normalized raw_description)
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_tx_dedupe ON transactions(date, amount, hash);
CREATE INDEX idx_tx_account_date ON transactions(account_id, date DESC);
CREATE INDEX idx_tx_category ON transactions(category_id);

CREATE TABLE categorization_rules (
    id          TEXT PRIMARY KEY,
    pattern     TEXT NOT NULL,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    priority    INTEGER NOT NULL DEFAULT 100, -- lower wins
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_rules_priority ON categorization_rules(priority, id);

CREATE TABLE budgets (
    id          TEXT PRIMARY KEY,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    month       TEXT NOT NULL, -- YYYY-MM
    amount      INTEGER NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE(category_id, month)
);
CREATE INDEX idx_budgets_month ON budgets(month);

-- +goose Down
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS categorization_rules;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS accounts;
