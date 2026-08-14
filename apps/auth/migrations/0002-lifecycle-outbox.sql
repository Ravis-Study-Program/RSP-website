-- Application-owned auth companion objects introduced after the pinned
-- Better Auth 1.6.27 baseline. This file is applied by migrate-auth.sql and
-- must remain safe for an existing rewrite database.
SET search_path TO auth;

CREATE TABLE IF NOT EXISTS account_deletion_recovery_tokens (
  id text PRIMARY KEY NOT NULL,
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash text NOT NULL UNIQUE,
  requested_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  revoked_at timestamptz,
  CONSTRAINT account_deletion_recovery_window CHECK (expires_at > requested_at),
  CONSTRAINT account_deletion_recovery_hash CHECK (
    length(token_hash) = 64 AND token_hash ~ '^[0-9a-f]{64}$'
  ),
  CONSTRAINT account_deletion_recovery_terminal CHECK (used_at IS NULL OR revoked_at IS NULL)
);

CREATE TABLE IF NOT EXISTS lifecycle_outbox (
  id text PRIMARY KEY NOT NULL,
  target text NOT NULL,
  payload jsonb NOT NULL,
  attempt_count integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL,
  locked_until timestamptz,
  last_error text,
  created_at timestamptz NOT NULL,
  delivered_at timestamptz,
  CONSTRAINT lifecycle_outbox_target_check CHECK (target IN ('identity', 'email')),
  CONSTRAINT lifecycle_outbox_attempt_count_check CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS account_deletion_recovery_tokens_user_id_idx
  ON account_deletion_recovery_tokens(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS account_deletion_recovery_tokens_one_open
  ON account_deletion_recovery_tokens(user_id)
  WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS lifecycle_outbox_pending_idx
  ON lifecycle_outbox(available_at, created_at)
  WHERE delivered_at IS NULL;
