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

Export users from Auth0 with a Management API users-exports job in JSON
format. The `identities` and `app_metadata` fields are required: linked
secondary identities only appear in `identities`, and `app_metadata.userId` is
the legacy server's binding to its `UserId`.

```json
{
  "format": "json",
  "fields": [
    { "name": "user_id" },
    { "name": "email" },
    { "name": "email_verified" },
    { "name": "blocked" },
    { "name": "identities" },
    { "name": "app_metadata" }
  ]
}
```

Record the gzip artifact's checksum, tenant, export time and custodian. Do not
commit the export, anything derived from it, access tokens or provider
secrets. Tests use synthetic fixtures only.

Password hashes are not exported. The Auth0 tenant is on the Free plan, which
cannot request a hash export, so every database user sets a new password
through the verified reset flow after cutover. Google users are pre-linked by
their Google `sub` and sign in unchanged.

## Identity import tooling

`rsp-migrate auth0` turns the export into Better Auth users, Google accounts
and `app.user_auth_links` rows. Run every step from the operations image in an
isolated working directory that is destroyed afterwards. `RUN_ID` is the
verified data migration run.

```sh
rsp-migrate auth0 normalize --export auth0_users.json.gz --out auth0-users.json
rsp-migrate auth0 candidates --run-id "$RUN_ID" --out app-candidates.json
rsp-migrate auth0 propose --auth0-users auth0-users.json \
  --app-candidates app-candidates.json \
  --out proposed-resolutions.json --report manual-resolutions.json
```

`normalize` accepts the gzip file as downloaded and rejects providers other
than `auth0` and `google-oauth2`, duplicate users, and identities that do not
include the user's primary identity. `candidates` lists active and suspended
app users with their legacy `UserId` from `migration.row_provenance`; deleted
members are never candidates.

`propose` drafts a resolution only when two independent signals agree: the
Auth0 user's `app_metadata.userId` names a legacy user, and its verified email
equals that member's contact email. Everything else lands in the manual report
with a reason: `blocked`, `email_unverified`, `no_legacy_user_id` (the legacy
server wrote `Unknown User` for members who never signed in again),
`legacy_user_not_found`, `email_mismatch` or `duplicate_legacy_user_candidate`.
Manual entries list advisory email candidates. A draft is not an approval:
review `proposed-resolutions.json`, add manual decisions, and save the result as
the approved `resolutions.json`.

```sh
rsp-migrate auth0 plan --auth0-users auth0-users.json \
  --app-candidates app-candidates.json --resolutions resolutions.json \
  --plan auth0-import-plan.json
```

Approvers sign off on the plan's status counts, unresolved Auth0 IDs and
checksum. Plan statuses:

- `matched`: explicitly resolved, Google only;
- `password_reset_required`: explicitly resolved with a database identity;
- `requires_resolution`: no explicit mapping, ambiguous email or a blocked
  Auth0 user. Blocked users can never be resolved;
- `imported`: written by `apply` into the bookkeeping table only.

`apply` takes the same four inputs as the plan. It recomputes the plan at its
original timestamp and exits with code 3 unless the checksum matches, so an
edited plan or changed input cannot be applied. Unresolved identities also exit
with code 3 unless `--allow-unresolved` is passed; they are then recorded and
left unimported, and those members can sign up again as new users.

For each app user, `apply` writes one Better Auth user with the member's
display name, email, account state and security version; one `google` account
row keyed by the Google `sub` when there is one; and one `better_auth` link
that is active only if the email is verified, matching the backend's own
`createIdentityUser`. Several Auth0 users resolved to the same member merge
into one Better Auth user. No `credential` rows are written. Imported password
users get `INVALID_EMAIL_OR_PASSWORD` until they reset, and Better Auth creates
the credential during the reset.

`apply` refuses to run unless the data migration run is `verified`, and refuses
a second import for the same run or plan. It rejects members who are deleted,
whose contact email differs from the Auth0 email, or who already have an auth
link. Two Google accounts or two different emails on one member are also
rejected. Everything happens in one serializable transaction that re-verifies
the result before committing, so a failure writes nothing.

`verify` repeats those invariant checks after commit and exits with code 3 on
any problem. Run it immediately after `apply`. Later sign-ins legitimately
change verification state and security versions.

## Password and provider handling

- Password hashes are not imported. Members with a database identity reset
  their password through the existing `/reset-password` flow, which proves
  control of the email before a credential exists.
- Preserve distinct Google/password provider records linked to the resolved app
  identity. Do not silently join records because emails match.
- TOTP seeds are not imported. Legacy admins arrive as pending Directors and
  activate after enrolling TOTP.
- Backup codes are never migrated in plaintext. New codes are generated after
  reauthentication, hashed, and single-use.
- Better Auth's reset flow also adds a password to a Google-only account
  without the fresh reauthentication `/connect-password` requires. This is
  stock Better Auth behavior and predates the import; decide separately whether
  to guard it.

## Rehearsal

1. Restore a copy of the verified production data migration into an isolated
   environment.
2. Apply `deploy/postgres/migrate-auth.sql` and
   `scripts/legacy-migration/auth0-import-schema.sql`.
3. Run normalize, candidates, propose and plan on the real export and work
   the manual list until approvers sign the plan.
4. Grant import access as described in the production sequence, then run
   `apply` and `verify` and revoke the grants.
5. Test verification, password reset for an imported database user, Google
   sign-in for a pre-linked user, explicit linking, session
   rotation/revocation, TOTP step-up, backup-code use, JWT/JWKS rotation,
   suspension and deletion recovery.
6. Confirm unaffiliated, unverified, kicked-only, suspended and deleted users
   cannot obtain usable protected access.
7. Destroy the rehearsal environment and retain only redacted reports.

`go test -tags=integration ./scripts/legacy-migration/...` covers the import
against PostgreSQL 18 with the production role layout, and the live Better Auth
suite proves an imported member without a credential can reset and sign in.
Neither substitutes for a rehearsal on the real export.

## Production sequence

1. Put the legacy product into maintenance/read-only mode and take final
   database/Auth0 exports.
2. Deploy Better Auth with production origin, Secure cookies, SES, Google
   credentials and high-entropy secrets from the platform secret store.
3. Apply the pinned auth schema, import identities and verify every reconciliation
   report before linking traffic. `rsp_auth` owns `auth` and `rsp_migration`
   owns `app` and `migration`, so the import role gets narrow access to `auth`
   for the window only. Run the grant and revoke as the database owner:

   ```sql
   GRANT USAGE ON SCHEMA auth TO rsp_migration;
   GRANT SELECT, INSERT ON auth.users, auth.accounts TO rsp_migration;
   ```

   ```sh
   psql "$MIGRATION_DATABASE_URL" --set ON_ERROR_STOP=1 \
     --file scripts/legacy-migration/auth0-import-schema.sql
   rsp-migrate auth0 apply --plan auth0-import-plan.json \
     --auth0-users auth0-users.json --app-candidates app-candidates.json \
     --resolutions resolutions.json --run-id "$RUN_ID" \
     --target-dsn "$MIGRATION_DATABASE_URL"
   rsp-migrate auth0 verify --run-id "$RUN_ID" \
     --target-dsn "$MIGRATION_DATABASE_URL"
   ```

   ```sql
   REVOKE SELECT, INSERT ON auth.users, auth.accounts FROM rsp_migration;
   REVOKE USAGE ON SCHEMA auth FROM rsp_migration;
   ```

   Preserve the apply and verify output with the plan checksum. Before opening
   traffic, send password users a notice that links to `/reset-password`
   rather than bulk-requesting resets: reset links expire after an hour and the
   reset endpoint is rate limited.

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
following: an authorized tenant export, encrypted artifact custody and
checksum, final manual resolutions and a signed plan, production
Google/mail/callback secrets, verified domain and Auth0 database backups, a
maintenance window, named data/security/product approvers, post-cutover
account fixtures, monitoring ownership and a switch-back owner.
No synthetic-plan success substitutes for those prerequisites.
