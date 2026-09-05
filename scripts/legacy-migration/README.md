# Legacy migration scripts

This directory contains the one-time import tool for the previous RSP system.
It is not part of the running API or the Goose application migrations.

Apply `schema.sql` to an isolated target before the first import. The schema
stores import evidence only; manifests and resolutions remain reviewed files.

Run the tool with:

```sh
go run ./scripts/legacy-migration/cmd legacy dry-run \
  --source-dsn "$LEGACY_DATABASE_URL" \
  --manifest rehearsal-manifest.json
```

Use `docs/data-migration.md` for the complete rehearsal and cutover procedure.
