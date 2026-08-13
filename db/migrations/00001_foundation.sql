-- +goose Up
CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS migration;
CREATE TYPE app.account_state AS ENUM ('active','suspended','deletion_pending','deleted');
CREATE TYPE app.season_status AS ENUM ('open','closed');
CREATE TYPE app.season_role AS ENUM ('student','mentor','coordinator');
CREATE TYPE app.enrollment_state AS ENUM ('active','completed','kicked','withdrawn');

CREATE TABLE app.users (
  id text PRIMARY KEY,
  slug text NOT NULL UNIQUE,
  display_name text NOT NULL,
  email text,
  account_state app.account_state NOT NULL DEFAULT 'active',
  timezone text NOT NULL DEFAULT 'Australia/Adelaide',
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE app.user_auth_links (
  auth_subject text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE app.seasons (
  id text PRIMARY KEY,
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  status app.season_status NOT NULL DEFAULT 'open',
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE app.enrollments (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  role app.season_role NOT NULL,
  state app.enrollment_state NOT NULL DEFAULT 'active',
  completed_by_close_id text,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  UNIQUE(user_id, season_id)
);
CREATE TABLE app.season_close_events (
  id text PRIMARY KEY,
  season_id text NOT NULL REFERENCES app.seasons(id) ON DELETE RESTRICT,
  actor_id text NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  reason text NOT NULL CHECK (length(trim(reason)) > 0),
  closed_at timestamptz NOT NULL,
  reopened_at timestamptz
);
ALTER TABLE app.enrollments ADD CONSTRAINT enrollment_close_fk FOREIGN KEY (completed_by_close_id) REFERENCES app.season_close_events(id) ON DELETE RESTRICT;
CREATE TABLE app.audit_events (
  id text PRIMARY KEY,
  actor_id text REFERENCES app.users(id) ON DELETE RESTRICT,
  action text NOT NULL,
  subject_type text NOT NULL,
  subject_id text NOT NULL,
  data jsonb NOT NULL DEFAULT '{}',
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION app.prevent_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit events are append-only'; END $$;
CREATE TRIGGER audit_events_append_only BEFORE UPDATE OR DELETE ON app.audit_events FOR EACH ROW EXECUTE FUNCTION app.prevent_audit_mutation();

-- +goose Down
DROP SCHEMA IF EXISTS migration CASCADE;
DROP SCHEMA IF EXISTS app CASCADE;

