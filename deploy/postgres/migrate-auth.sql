-- Versioned, checksum-pinned auth schema runner. Better Auth owns its generated
-- baseline, while RSP companion objects are added through numbered upgrades.
-- Existing rewrite databases that predate this runner adopt the validated
-- baseline and still receive every later migration.
\set ON_ERROR_STOP on

SET search_path TO auth;
SELECT pg_advisory_lock(hashtext('rsp-auth-schema-migrations'));

CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY NOT NULL,
  checksum text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT schema_migrations_checksum_check CHECK (checksum ~ '^[0-9a-f]{64}$')
);

SELECT to_regclass('auth.users') IS NULL AS needs_baseline \gset
\if :needs_baseline
\echo 'Applying pinned Better Auth 1.6.27 baseline'
BEGIN;
\ir ../../apps/auth/migrations/better-auth.sql
INSERT INTO auth.schema_migrations (version, checksum)
VALUES ('0001-better-auth-1.6.27', '17f1d2e1a897e44eca5a01a7204f658e5bb3f54e92d10f6a03e3c30a374662d5');
COMMIT;
\else
-- Databases created before version tracking must match the baseline shape
-- before it is adopted. This prevents marking a partial/foreign schema valid.
DO $validation$
DECLARE
  missing text;
BEGIN
  SELECT string_agg(required.object_name, ', ' ORDER BY required.object_name)
    INTO missing
    FROM (VALUES
      ('auth.users'), ('auth.sessions'), ('auth.accounts'),
      ('auth.verifications'), ('auth.two_factors'), ('auth.jwks'),
      ('auth.rate_limits')
    ) AS required(object_name)
   WHERE to_regclass(required.object_name) IS NULL;
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'existing auth baseline is incomplete; missing: %', missing;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'auth' AND table_name = 'users'
       AND column_name = 'security_version'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'auth' AND table_name = 'sessions'
       AND column_name = 'mfa_verified_at'
  ) THEN
    RAISE EXCEPTION 'existing auth baseline lacks required access-state columns';
  END IF;
END
$validation$;

INSERT INTO auth.schema_migrations (version, checksum)
VALUES ('0001-better-auth-1.6.27', '17f1d2e1a897e44eca5a01a7204f658e5bb3f54e92d10f6a03e3c30a374662d5')
ON CONFLICT (version) DO NOTHING;
\endif

DO $checksum$
BEGIN
  IF (SELECT checksum FROM auth.schema_migrations WHERE version = '0001-better-auth-1.6.27')
       <> '17f1d2e1a897e44eca5a01a7204f658e5bb3f54e92d10f6a03e3c30a374662d5' THEN
    RAISE EXCEPTION 'auth migration checksum mismatch: 0001-better-auth-1.6.27';
  END IF;
END
$checksum$;

SELECT NOT EXISTS (
  SELECT 1 FROM auth.schema_migrations WHERE version = '0002-lifecycle-outbox'
) AS needs_lifecycle_outbox \gset
\if :needs_lifecycle_outbox
\echo 'Applying auth migration 0002-lifecycle-outbox'
BEGIN;
\ir ../../apps/auth/migrations/0002-lifecycle-outbox.sql
INSERT INTO auth.schema_migrations (version, checksum)
VALUES ('0002-lifecycle-outbox', '44a4e5a1b012d49ca6399b078a76787f6a3e10117bdf3f4dce5a234d9fed900a');
COMMIT;
\else
\echo 'Auth migration 0002-lifecycle-outbox already applied'
\endif

DO $checksum$
BEGIN
  IF (SELECT checksum FROM auth.schema_migrations WHERE version = '0002-lifecycle-outbox')
       <> '44a4e5a1b012d49ca6399b078a76787f6a3e10117bdf3f4dce5a234d9fed900a' THEN
    RAISE EXCEPTION 'auth migration checksum mismatch: 0002-lifecycle-outbox';
  END IF;
END
$checksum$;

SELECT pg_advisory_unlock(hashtext('rsp-auth-schema-migrations'));
