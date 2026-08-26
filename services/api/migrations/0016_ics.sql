-- +goose Up
-- Phase 13c: ICS import — dedupe key for events imported from calendars.

ALTER TABLE events ADD COLUMN external_uid TEXT;
CREATE UNIQUE INDEX idx_events_external_uid ON events(external_uid) WHERE external_uid IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_events_external_uid;
ALTER TABLE events DROP COLUMN external_uid;
