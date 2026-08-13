-- Generated from Better Auth 1.6.27 table metadata for the configuration in
-- src/auth.ts. Better Auth owns every object in the auth schema.
CREATE SCHEMA IF NOT EXISTS auth;
SET search_path TO auth;

CREATE TABLE users (
  id text PRIMARY KEY NOT NULL,
  name text NOT NULL,
  email text NOT NULL UNIQUE,
  email_verified boolean NOT NULL,
  image text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  two_factor_enabled boolean,
  account_state text NOT NULL,
  security_version integer NOT NULL,
  deletion_requested_at timestamptz,
  deletion_recovery_deadline timestamptz,
  CONSTRAINT users_account_state_check
    CHECK (account_state IN ('active', 'suspended', 'deletion_pending', 'deleted')),
  CONSTRAINT users_security_version_check CHECK (security_version > 0),
  CONSTRAINT users_deletion_dates_check CHECK (
    (account_state = 'deletion_pending' AND deletion_requested_at IS NOT NULL AND deletion_recovery_deadline IS NOT NULL)
    OR (account_state <> 'deletion_pending')
  )
);

CREATE TABLE sessions (
  id text PRIMARY KEY NOT NULL,
  expires_at timestamptz NOT NULL,
  token text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  ip_address text,
  user_agent text,
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  mfa_verified_at timestamptz
);

CREATE TABLE accounts (
  id text PRIMARY KEY NOT NULL,
  account_id text NOT NULL,
  provider_id text NOT NULL,
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  access_token text,
  refresh_token text,
  id_token text,
  access_token_expires_at timestamptz,
  refresh_token_expires_at timestamptz,
  scope text,
  password text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT accounts_provider_identity_unique UNIQUE (provider_id, account_id),
  CONSTRAINT accounts_user_provider_unique UNIQUE (user_id, provider_id)
);

CREATE TABLE verifications (
  id text PRIMARY KEY NOT NULL,
  identifier text NOT NULL,
  value text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE TABLE two_factors (
  id text PRIMARY KEY NOT NULL,
  secret text NOT NULL,
  backup_codes text NOT NULL,
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  verified boolean,
  failed_verification_count integer,
  locked_until timestamptz,
  CONSTRAINT two_factors_user_unique UNIQUE (user_id),
  CONSTRAINT two_factors_failed_count_check CHECK (failed_verification_count >= 0)
);

CREATE TABLE jwks (
  id text PRIMARY KEY NOT NULL,
  public_key text NOT NULL,
  private_key text NOT NULL,
  created_at timestamptz NOT NULL,
  expires_at timestamptz
);

CREATE TABLE rate_limits (
  id text PRIMARY KEY NOT NULL,
  key text NOT NULL UNIQUE,
  count integer NOT NULL,
  last_request bigint NOT NULL,
  CONSTRAINT rate_limits_count_check CHECK (count >= 0)
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX accounts_user_id_idx ON accounts(user_id);
CREATE INDEX verifications_identifier_idx ON verifications(identifier);
CREATE INDEX two_factors_secret_idx ON two_factors(secret);
CREATE INDEX two_factors_user_id_idx ON two_factors(user_id);
