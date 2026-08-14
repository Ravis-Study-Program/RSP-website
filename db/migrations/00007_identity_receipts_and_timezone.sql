-- +goose Up

ALTER TABLE app.users
  ADD COLUMN timezone_configured boolean NOT NULL DEFAULT true;

CREATE TABLE app.identity_event_receipts (
  event_id text PRIMARY KEY,
  auth_subject text NOT NULL,
  event_type text NOT NULL,
  security_version bigint NOT NULL CHECK (security_version > 0),
  received_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE app.identity_event_receipts;
ALTER TABLE app.users DROP COLUMN timezone_configured;
