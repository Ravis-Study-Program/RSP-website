-- Auth0 identity import bookkeeping. Apply after schema.sql. Unlike schema.sql
-- this file is additive and safe to re-run, because targets that already hold
-- a verified data migration cannot re-apply schema.sql.

CREATE TABLE IF NOT EXISTS migration.auth0_imports (
  run_id uuid PRIMARY KEY REFERENCES migration.runs(id) ON DELETE RESTRICT,
  plan_checksum text NOT NULL UNIQUE,
  auth_user_count integer NOT NULL CHECK (auth_user_count >= 0),
  google_account_count integer NOT NULL CHECK (google_account_count >= 0),
  password_reset_count integer NOT NULL CHECK (password_reset_count >= 0),
  unresolved_count integer NOT NULL CHECK (unresolved_count >= 0),
  applied_at timestamptz NOT NULL
);

ALTER TABLE migration.auth0_identity_imports
  ADD COLUMN IF NOT EXISTS auth_user_id text,
  ADD COLUMN IF NOT EXISTS identities jsonb;

-- Every imported row names its Better Auth user; nothing else may.
DO $constraint$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'auth0_identity_imports_imported_auth_user'
       AND conrelid = 'migration.auth0_identity_imports'::regclass
  ) THEN
    ALTER TABLE migration.auth0_identity_imports
      ADD CONSTRAINT auth0_identity_imports_imported_auth_user CHECK (
        (status = 'imported') = (auth_user_id IS NOT NULL AND app_user_id IS NOT NULL)
      );
  END IF;
END
$constraint$;
