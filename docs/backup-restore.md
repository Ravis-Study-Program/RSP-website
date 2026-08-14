# Backup and restore runbook

Backups are an operational control, not a substitute for an import manifest or
append-only audit history. Production schedules, retention, encryption and
off-site replication belong in the deployment platform; these commands define
the portable PostgreSQL procedure.

## Backup

Choose an encrypted directory outside the repository and ensure only the
operator can read it. The example variable is intentionally task-specific:

```sh
RSP_BACKUP_DIR=/secure/rsp-backups/2026-08-13
install -d -m 0700 "$RSP_BACKUP_DIR"
docker compose exec -T postgres pg_dump \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  --format custom \
  --compress 9 \
  --no-owner \
  --file /tmp/rsp.dump
docker compose cp postgres:/tmp/rsp.dump "$RSP_BACKUP_DIR/rsp.dump"
shasum -a 256 "$RSP_BACKUP_DIR/rsp.dump" > "$RSP_BACKUP_DIR/rsp.dump.sha256"
chmod 0600 "$RSP_BACKUP_DIR/rsp.dump" "$RSP_BACKUP_DIR/rsp.dump.sha256"
```

Remove the temporary container file after confirming the external checksum.
Record PostgreSQL version, UTC timestamp, database/schema scope, deployment
revision, backup size, checksum and operator. Never store dumps in Git, CI
artifacts, general chat or an unencrypted personal cloud folder.

The full database backup includes `app`, `auth` and `migration`, preserving the
identity/domain linkage. If policy requires separate custody, take schema-only
or schema-filtered copies in addition—not instead—and document dependencies.

## Verification restore

Restore into a new isolated database, never over the only production copy. The
name below is explicit so a typo cannot target the active database:

```sh
RSP_BACKUP_DIR=/secure/rsp-backups/2026-08-13
shasum -a 256 -c "$RSP_BACKUP_DIR/rsp.dump.sha256"
docker compose exec -T postgres createdb \
  --username "$POSTGRES_USER" rsp_restore_verify
docker compose cp "$RSP_BACKUP_DIR/rsp.dump" postgres:/tmp/rsp-restore.dump
docker compose exec -T postgres pg_restore \
  --username "$POSTGRES_USER" \
  --dbname rsp_restore_verify \
  --no-owner \
  --exit-on-error \
  /tmp/rsp-restore.dump
```

Connect using a read-only verification role and check:

- Schemas/tables/migration versions exist.
- App/auth user-link counts reconcile without exposing PII in the report.
- Foreign keys and append-only triggers are valid.
- Deleted/test/null/state histograms match the source database.
- The application can start with outbound email/provider callbacks disabled.
- A representative permission matrix smoke test passes.

Destroy `rsp_restore_verify` only after evidence is recorded and a second
operator confirms the exact target. Dropping a database is destructive and must
not be copied from this runbook without that check.

## Recovery principles

1. Declare an incident and freeze writes if continuing would make recovery less
   reliable.
2. Identify recovery point/time objectives and the last verified backup/WAL
   position.
3. Restore into isolation, verify integrity, then choose in-place repair versus
   traffic switch with the incident lead and data owner.
4. Rotate credentials if backup confidentiality may be compromised.
5. Preserve logs, manifests and the pre-recovery database for forensics.
6. Document lost/replayed writes and notify affected people under the response
   policy.

Perform a full restore rehearsal before every live migration and at a regular
operational cadence. A successful `pg_dump` without a successful restore test is
not a verified backup.
