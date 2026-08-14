# Legacy data migration runbook

`rsp-migrate` imports the legacy PostgreSQL domain into clean `app` tables and
records evidence in `migration`. It does not move Auth0 credentials; identity
cutover is a separate prerequisite in `auth-cutover.md`.

Never run an apply first. A checksum-approved dry run and restore rehearsal are
mandatory.

## Safety model

- Source reads use `READ ONLY, REPEATABLE READ` so counts, IDs and checksums
  describe one snapshot.
- Target apply holds a dedicated PostgreSQL advisory lock and one transaction.
- A manifest binds source schema fingerprint, EF history, row counts, ID sets,
  canonical per-table SHA-256 checksums, mappings, anomalies and auto-fixes.
- A resolution file is accepted only when its checksum binds it to the exact
  source/manifest. Changed source data invalidates approval.
- Original IDs, timestamps, null/deletion/test state and available history are
  retained. Every represented source row has provenance.
- Rollback selects one import `run-id`; it is not a general database reset.

## Before rehearsal

1. Restore a production backup into an isolated legacy database.
2. Create an empty PostgreSQL 17.11 target and run `just migrate-up`.
3. Record source/target server versions, backup checksum and operator names.
4. Set `LEGACY_DATABASE_URL` read-only and `DATABASE_URL` for the isolated
   target. Do not place either value in Git or shell history on a shared host.
5. Confirm the target contains no real auth credentials and outbound email is
   disabled/captured.

## Packaged controlled shell and DSNs

`backend/Dockerfile` has an `ops` target containing the non-root `rsp-migrate`
and `rspctl` binaries plus runtime certificates and timezone data:

```sh
docker build --file backend/Dockerfile --target ops --tag rsp-ops:cutover .
```

Keep credentials in a mode-0600 secret-store materialized env file, not in the
repository or command history. The two migration DSNs have deliberately
different authorities:

```dotenv
# /secure/rsp-migrate.env
# Provision this account on the isolated legacy restore with
# CONNECT/USAGE/SELECT only. Production validates its TLS hostname and CA.
LEGACY_DATABASE_URL=postgresql://legacy_migration_reader:<source-password>@<legacy-host>:5432/<legacy-database>?sslmode=verify-full

# rsp_migration owns app/migration schema objects and is the only role allowed
# to reverse an import. Do not add an app-only search_path to this DSN.
DATABASE_URL=postgresql://rsp_migration:<migration-password>@postgres:5432/<target-database>?sslmode=require
```

For the repository's local Compose-only rehearsal, `postgres` has no TLS and
the exact target suffix is instead `?sslmode=disable`; source host, CA and
database names remain properties of the isolated legacy restore. Confirm the
effective roles with `SELECT current_user` before apply.

Mount only the reviewed artifacts directory and attach the disposable tool to
the target data network. Because `rsp-migrate` reads both DSNs from its
environment, no secret needs to appear in the command arguments:

```sh
RSP_MIGRATION_ARTIFACTS=/secure/rsp-migration-artifacts
RSP_DATA_NETWORK="${COMPOSE_PROJECT_NAME:-rsp-website}_data"

docker run --rm --read-only --tmpfs /tmp \
  --user "$(id -u):$(id -g)" \
  --network "$RSP_DATA_NETWORK" \
  --env-file /secure/rsp-migrate.env \
  --mount type=bind,src="$RSP_MIGRATION_ARTIFACTS",dst=/artifacts \
  rsp-ops:cutover rsp-migrate legacy dry-run \
  --manifest /artifacts/rehearsal-manifest.json
```

The native `go run` commands below are development equivalents. Use the pinned
operations image for a controlled rehearsal or cutover.

## Dry run

Against PostgreSQL:

```sh
go run ./backend/cmd/rsp-migrate legacy dry-run \
  --source-dsn "$LEGACY_DATABASE_URL" \
  --manifest rehearsal-manifest.json
```

Against the checked-in non-PII fixture:

```sh
go run ./backend/cmd/rsp-migrate legacy dry-run \
  --source-fixture backend/internal/migration/testdata/valid_snapshot.json \
  --manifest fixture-manifest.json
```

Review the manifest rather than only the exit code. Reconcile all 18 source
domain tables, EF migration history, ID sets, deleted/test/null histograms,
enum/timestamp histograms, transformed checksums and every auto-fix.

The following stop the import and need an explicit checksum-bound resolution:

- Orphans or cross-season relationships.
- Ambiguous duplicate people or conflicting slugs/emails.
- Invalid scores, ranges or positive-duration constraints.
- Multiple incompatible subtypes.
- Any transform that changes meaning rather than representation.

Permitted automatic fixes are restricted to documented deterministic and
lossless spelling/enum mappings, normalized lookup shadows, known derivable
timestamps, provenance-preserving duplicate pure-join collapse, approved
ended-season closure and derived alumni reconciliation.

After resolving a blocking anomaly, rerun dry-run with the approved file:

```sh
go run ./backend/cmd/rsp-migrate legacy dry-run \
  --source-dsn "$LEGACY_DATABASE_URL" \
  --resolution approved-resolutions.json \
  --manifest approved-manifest.json
```

## Apply and verify

```sh
go run ./backend/cmd/rsp-migrate legacy apply \
  --source-dsn "$LEGACY_DATABASE_URL" \
  --target-dsn "$DATABASE_URL" \
  --resolution approved-resolutions.json \
  --manifest approved-manifest.json

go run ./backend/cmd/rsp-migrate legacy verify \
  --target-dsn "$DATABASE_URL" \
  --manifest approved-manifest.json
```

Verification is acceptable only with zero unexplained rows, identical expected
ID sets and transformed checksums, zero invalid foreign keys, matching
deleted/test/null/enum/timestamp histograms, and a recorded explanation for
every fix. Import already-ended seasons as closed at snapshot time and complete
only enrollments active at that close. The old graduate flag is reconciliation
evidence; alumni is derived from completed student enrollment.

Run application smoke tests against the isolated target, then prove rollback:

```sh
go run ./backend/cmd/rsp-migrate legacy rollback \
  --target-dsn "$DATABASE_URL" \
  --run-id RUN_ID_FROM_MANIFEST
```

Verify that only the selected run's imported application rows were removed and
append-only migration evidence records the rollback. Restore the isolated
target again and repeat apply/verify to prove reproducibility.

The packaged equivalents use the same env file and reviewed artifact mount:

```sh
docker run --rm --read-only --tmpfs /tmp \
  --user "$(id -u):$(id -g)" --network "$RSP_DATA_NETWORK" \
  --env-file /secure/rsp-migrate.env \
  --mount type=bind,src="$RSP_MIGRATION_ARTIFACTS",dst=/artifacts \
  rsp-ops:cutover rsp-migrate legacy apply \
  --manifest /artifacts/approved-manifest.json \
  --resolution /artifacts/approved-resolutions.json

docker run --rm --read-only --tmpfs /tmp \
  --user "$(id -u):$(id -g)" --network "$RSP_DATA_NETWORK" \
  --env-file /secure/rsp-migrate.env \
  --mount type=bind,src="$RSP_MIGRATION_ARTIFACTS",dst=/artifacts \
  rsp-ops:cutover rsp-migrate legacy verify \
  --manifest /artifacts/approved-manifest.json

docker run --rm --read-only --tmpfs /tmp \
  --user "$(id -u):$(id -g)" --network "$RSP_DATA_NETWORK" \
  --env-file /secure/rsp-migrate.env \
  rsp-ops:cutover rsp-migrate legacy rollback \
  --run-id RUN_ID_FROM_MANIFEST
```

## Cutover window

1. Announce maintenance and disable legacy writes.
2. Take and verify final database and Auth0 exports.
3. Run dry-run against the final read-only snapshot. Its checksum must match the
   manifest approved for apply, or approval restarts.
4. Apply, verify and run permission/PII smoke tests.
5. Complete identity import/link reconciliation before allowing sign-in.
6. Switch traffic only after named product, data and security approvers sign
   off. Preserve legacy infrastructure and backups through the rollback window.

If any invariant or count fails, keep the legacy system authoritative. Do not
edit target rows manually to make verification pass.
