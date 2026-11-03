-- Better Auth 1.7 records each JWKS key's algorithm and curve. Both columns are
-- nullable, so keys created under the 1.6.27 baseline remain valid. This file
-- is applied by migrate-auth.sql and must remain safe for an existing rewrite
-- database.
SET search_path TO auth;

ALTER TABLE jwks ADD COLUMN IF NOT EXISTS alg text;
ALTER TABLE jwks ADD COLUMN IF NOT EXISTS crv text;
