-- +goose Up
-- Phase 12a: finance depth — managed subscriptions, account kinds + opening
-- balances, rule amount conditions, transaction tags.

CREATE TABLE subscriptions (
    id           TEXT PRIMARY KEY,
    merchant     TEXT NOT NULL,
    amount_minor INTEGER NOT NULL,
    cadence      TEXT NOT NULL DEFAULT 'monthly' CHECK(cadence IN ('weekly','monthly','yearly')),
    next_due     TEXT, -- YYYY-MM-DD; refreshed on sync
    status       TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','muted','cancelled')),
    occurrences  INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    UNIQUE(merchant, amount_minor)
);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

ALTER TABLE accounts ADD COLUMN kind TEXT NOT NULL DEFAULT 'asset' CHECK(kind IN ('asset','liability'));
ALTER TABLE accounts ADD COLUMN opening_balance_minor INTEGER NOT NULL DEFAULT 0;

ALTER TABLE categorization_rules ADD COLUMN amount_min INTEGER;
ALTER TABLE categorization_rules ADD COLUMN amount_max INTEGER;

ALTER TABLE transactions ADD COLUMN tags TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array');
CREATE INDEX idx_tx_tags ON transactions(tags);

-- +goose Down
DROP INDEX IF EXISTS idx_tx_tags;
ALTER TABLE transactions DROP COLUMN tags;
ALTER TABLE categorization_rules DROP COLUMN amount_max;
ALTER TABLE categorization_rules DROP COLUMN amount_min;
ALTER TABLE accounts DROP COLUMN opening_balance_minor;
ALTER TABLE accounts DROP COLUMN kind;
DROP TABLE IF EXISTS subscriptions;
