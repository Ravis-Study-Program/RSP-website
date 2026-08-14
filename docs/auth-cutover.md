# Better Auth and Auth0 cutover

Production identity migration is not performed by normal application startup.
It requires an authorized Auth0 export and a planned maintenance window. Old
Auth0 sessions cannot migrate; every member signs in again.

## Target identity contract

- Google and email/password providers are supported.
- Email verification is required before protected application access.
- A database-backed Secure, HttpOnly, SameSite=Lax cookie has a rolling
  seven-day session lifetime.
- The browser exchanges the session for a five-minute access JWT and stores it
  only in memory.
- Go validates issuer, audience, expiry, signature and JWKS key ID, then maps
  JWT `sub` through one active `app.user_auth_links` record.
- Matching email is never enough to merge accounts. Adding another provider is
  an explicit, freshly reauthenticated “Connect sign-in method” operation.
- Coordinator, Director and System Admin access requires recent application MFA
  even after OAuth. Privileged assignments remain pending until TOTP is ready.

## Pre-cutover evidence

Obtain an Auth0 export through an authorized channel and record its encrypted
artifact checksum, tenant/export time and custodian. The export must include,
where available:

- Stable Auth0 user ID and provider account links.
- Normalized and original email plus verification state.
- Password hash and algorithm metadata.
- Disabled/blocked/deleted state and MFA enrollment metadata.

Do not commit the export, derived PII fixtures, access tokens or provider
secrets. Tests use synthetic fixtures only.

Reconcile each identity to exactly one imported opaque app user. Ambiguous
emails, duplicate providers, conflicting verified emails and missing app users
go to a manual resolution list; approval is bound to the resulting plan
checksum. There is no “best guess” match.

## Executable synthetic preflight

The repository implements a planning and reconciliation preflight, not a live
Auth0 or Better Auth importer. Build the non-root operations image and run the
checked-in non-PII fixtures from an isolated output directory:

```sh
docker build --file backend/Dockerfile --target ops --tag rsp-ops:cutover .

RSP_AUTH0_REHEARSAL_DIR=/tmp/rsp-auth0-rehearsal
mkdir -p "$RSP_AUTH0_REHEARSAL_DIR"
docker run --rm --read-only --tmpfs /tmp \
  --user "$(id -u):$(id -g)" \
  --mount type=bind,src="$PWD/backend/internal/migration/testdata",dst=/fixtures,readonly \
  --mount type=bind,src="$RSP_AUTH0_REHEARSAL_DIR",dst=/artifacts \
  rsp-ops:cutover rsp-migrate auth0 plan \
  --auth0-users /fixtures/auth0_users.json \
  --app-candidates /fixtures/auth0_app_candidates.json \
  --resolutions /fixtures/auth0_resolutions.json \
  --plan /artifacts/auth0-import-plan.json
```

The input is the documented normalized JSON shape, not an assertion that every
Auth0 tenant export uses that shape. The plan rejects missing or duplicate
provider/account keys and stale resolutions. Verified matching email is only
an advisory candidate and never creates a mapping. Plan statuses use the same
vocabulary as `migration.auth0_identity_imports`:

- `matched`: an explicit app-user resolution with no password reset needed;
- `requires_resolution`: no explicit mapping, including ambiguous email;
- `password_reset_required`: explicitly mapped database identity whose hash is
  not proven compatible;
- `imported`: reserved for a future authorized apply and always zero in this
  preflight.

Review the plan's status counts, sorted provider/account set, unresolved Auth0
IDs and checksum. The preflight does not write `auth`, `app.user_auth_links` or
`migration.auth0_identity_imports`.

## Password and provider handling

- Import a password hash only if Auth0 supplies an algorithm/parameters Better
  Auth can verify without weakening it. Otherwise mark password sign-in for a
  verified reset flow.
- Preserve distinct Google/password provider records linked to the resolved app
  identity. Do not silently join records because emails match.
- Existing TOTP seeds are imported only when format and custody are explicitly
  supported and security-approved; otherwise privileged users reenroll.
- Backup codes are never migrated in plaintext. New codes are generated after
  reauthentication, hashed, and single-use.

## Rehearsal

1. Restore synthetic domain and Auth0 fixtures into an isolated environment.
2. Apply the pinned `apps/auth/migrations/better-auth.sql` to schema `auth`.
3. Run the identity planning preflight and reconcile counts/provider ID sets.
4. Only after a live importer is separately implemented and approved, apply it
   and prove each auth subject has exactly one active app link.
5. Test verification, password reset, Google callback, explicit linking,
   session rotation/revocation, TOTP step-up, backup-code use, JWT/JWKS rotation,
   suspension and deletion recovery.
6. Confirm unaffiliated, unverified, kicked-only, suspended and deleted users
   cannot obtain usable protected access.
7. Destroy the rehearsal environment and retain only redacted reports.

Steps 4–7 are acceptance criteria for the future live importer. The synthetic
planner alone cannot satisfy them.

## Production sequence

1. Put the legacy product into maintenance/read-only mode and take final
   database/Auth0 exports.
2. Deploy Better Auth with production origin, Secure cookies, SES, Google
   credentials and high-entropy secrets from the platform secret store.
3. Apply the pinned auth schema, import identities and verify every reconciliation
   report before linking traffic.
4. Bootstrap the first System Admin once from the packaged operations image.
   `rspctl` must use the runtime app role and `app` search path, never the
   migration-owner DSN:

   ```sh
   # /secure/rspctl.env is mode 0600 and populated by the secret store:
   # DATABASE_URL=postgresql://rsp_app:<app-password>@postgres:5432/<database>?sslmode=require&options=-c%20search_path%3Dapp
   RSP_DATA_NETWORK="${COMPOSE_PROJECT_NAME:-rsp-website}_data"
   docker run --rm --read-only --tmpfs /tmp \
     --network "$RSP_DATA_NETWORK" \
     --env-file /secure/rspctl.env \
     rsp-ops:cutover rspctl bootstrap-admin \
     --auth-subject 'BETTER_AUTH_USER_ID' \
     --email 'verified-admin@example.org'
   ```

   There is no public bootstrap endpoint. Preserve the command result and the
   resulting audit event without recording the password or DSN. The local
   Compose-only database has no TLS, so its DSN uses `sslmode=disable`; a real
   deployment uses its platform-issued TLS settings.

5. Revoke any sessions created during rehearsal or smoke tests, rotate JWKS if
   rehearsal keys crossed the boundary, and enable the production callback.
6. Smoke test a normal password user, Google user, privileged MFA user,
   unverified user, suspended user and deletion recovery.
7. Enable user traffic and monitor auth response classes, origin rejections,
   verification mail, resets, link conflicts and API invalid-token errors.

## Switch-back

Stop new sign-ins first. Keep the new database and audit logs intact for
forensics; do not delete or reverse individual identity links manually. If the
legacy system must resume, restore its routing and session authority under the
documented incident owner. Accounts created only after cutover need a separate
reconciliation before a later retry.

Immediately rotate Better Auth, identity-service, Google, SES and database
credentials if confidentiality—not application behavior—caused switch-back.

## Prerequisites not supplied by this repository

A real tenant import remains blocked until the cutover owner has all of the
following: an authorized tenant export and its format/version, encrypted
artifact custody and checksum, a reviewed normalizer into the preflight shape,
a Better Auth version-pinned provider/password import contract, a security
decision for each password-hash algorithm and MFA format, final manual
resolutions, production Google/mail/callback secrets, verified domain and Auth0
database backups, a maintenance window, named data/security/product approvers,
post-cutover account fixtures, monitoring ownership and a switch-back owner.
No synthetic-plan success substitutes for those prerequisites.
