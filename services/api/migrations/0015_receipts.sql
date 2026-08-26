-- +goose Up
-- Phase 13b: receipt attachments on transactions (file lives on disk under
-- the attachments dir; the row records its relative name).

ALTER TABLE transactions ADD COLUMN receipt_file TEXT;
ALTER TABLE transactions ADD COLUMN receipt_name TEXT;

-- +goose Down
ALTER TABLE transactions DROP COLUMN receipt_name;
ALTER TABLE transactions DROP COLUMN receipt_file;
