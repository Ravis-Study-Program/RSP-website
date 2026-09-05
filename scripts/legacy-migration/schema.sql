-- Legacy import bookkeeping. Apply this to the target database before running
-- the importer; it is intentionally not a Goose application migration.

CREATE SCHEMA IF NOT EXISTS migration;

CREATE TYPE migration.run_state AS ENUM (
  'planned',
  'applying',
  'applied',
  'verified',
  'failed'
);

CREATE TYPE migration.anomaly_severity AS ENUM ('warning', 'blocking');
CREATE TYPE migration.resolution_action AS ENUM (
  'map_id',
  'use_value',
  'skip_source_row',
  'approve_transform'
);

CREATE TABLE migration.runs (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  manifest_checksum text NOT NULL UNIQUE,
  source_schema_fingerprint text NOT NULL,
  source_snapshot_at timestamptz NOT NULL,
  state migration.run_state NOT NULL DEFAULT 'planned',
  started_at timestamptz,
  finished_at timestamptz,
  error_summary text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE migration.source_tables (
  run_id uuid NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  source_table text NOT NULL,
  source_count bigint NOT NULL CHECK (source_count >= 0),
  imported_count bigint NOT NULL CHECK (imported_count >= 0),
  source_ids_checksum text NOT NULL,
  transformed_checksum text NOT NULL,
  PRIMARY KEY (run_id, source_table)
);

CREATE TABLE migration.row_provenance (
  run_id uuid NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  source_table text NOT NULL,
  source_id text NOT NULL,
  target_table text NOT NULL,
  target_id text NOT NULL,
  source_checksum text NOT NULL,
  transformed_checksum text NOT NULL,
  transformed_data jsonb NOT NULL,
  duplicate_group text,
  PRIMARY KEY (run_id, source_table, source_id, target_table, target_id)
);
CREATE INDEX row_provenance_target
  ON migration.row_provenance (target_table, target_id);

CREATE TABLE migration.anomalies (
  run_id uuid NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  code text NOT NULL,
  source_table text NOT NULL,
  source_id text,
  severity migration.anomaly_severity NOT NULL,
  detail text NOT NULL,
  context jsonb NOT NULL DEFAULT '{}'::jsonb,
  resolution_checksum text,
  PRIMARY KEY (run_id, code, source_table, source_id)
);

CREATE TABLE migration.resolutions (
  run_id uuid NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  anomaly_code text NOT NULL,
  source_table text NOT NULL,
  source_id text,
  action migration.resolution_action NOT NULL,
  value jsonb NOT NULL,
  resolution_file_checksum text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, anomaly_code, source_table, source_id)
);

CREATE TABLE migration.auto_fixes (
  run_id uuid NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  source_table text NOT NULL,
  source_id text,
  code text NOT NULL,
  before_value jsonb,
  after_value jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, source_table, source_id, code)
);

CREATE TABLE migration.auth0_identity_imports (
  run_id uuid NOT NULL REFERENCES migration.runs(id) ON DELETE RESTRICT,
  auth0_user_id text NOT NULL,
  app_user_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  provider text,
  provider_account_id text,
  email_verified boolean,
  password_hash_compatible boolean,
  status text NOT NULL CHECK (status IN ('matched', 'requires_resolution', 'password_reset_required', 'imported')),
  detail text,
  PRIMARY KEY (run_id, auth0_user_id)
);
