-- +goose Up
-- Phase 13a: multi-currency — exchange rates relative to a configurable base
-- currency (default IDR). Missing rate for a currency = treated as 1:1.

CREATE TABLE exchange_rates (
    code         TEXT PRIMARY KEY, -- ISO-ish code matching accounts.currency / transactions.currency
    rate_to_base REAL NOT NULL CHECK(rate_to_base > 0), -- 1 unit of code = N base units
    updated_at   TEXT NOT NULL
);

INSERT OR IGNORE INTO app_meta (key, value, updated_at)
VALUES ('fx_base', 'IDR', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

-- +goose Down
DELETE FROM app_meta WHERE key='fx_base';
DROP TABLE IF EXISTS exchange_rates;
