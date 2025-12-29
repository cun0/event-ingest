-- migrations/001_init.sql

CREATE TABLE IF NOT EXISTS events (
  id          BIGSERIAL PRIMARY KEY,
  dedup_key   TEXT        NOT NULL,
  event_name  TEXT        NOT NULL,
  channel     TEXT        NOT NULL,
  campaign_id TEXT        NULL,
  user_id     TEXT        NOT NULL,
  ts          TIMESTAMPTZ NOT NULL,
  tags        TEXT[]      NOT NULL DEFAULT '{}',
  metadata    JSONB       NOT NULL DEFAULT '{}'::jsonb,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotency
CREATE UNIQUE INDEX IF NOT EXISTS events_dedup_key_uq
  ON events (dedup_key);

CREATE INDEX IF NOT EXISTS events_event_name_ts_idx
  ON events (event_name, ts);
