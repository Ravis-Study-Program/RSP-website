# Auth schema migrations

`better-auth.sql` is the reviewed Better Auth 1.6.27 baseline. The Better Auth
baseline assumes the privileged deployment provisioner has already created the
`auth` schema with `rsp_auth` as its owner; the runtime role is never granted
database-wide `CREATE`. The Better Auth
CLI introspects PostgreSQL while generating, so regeneration requires an empty,
disposable database rather than running offline:

```sh
DATABASE_URL='postgresql://.../disposable?options=-c%20search_path%3Dauth' \
  pnpm --filter @rsp/auth generate-schema
```

Review generated SQL and constraints before updating the pinned SHA-256 in
`deploy/postgres/migrate-auth.sql` and `test/migration-drift.test.ts`.
Application-owned additions use numbered, repeatable files such as
`0002-lifecycle-outbox.sql`; they are applied and checksum-verified by the
deployment runner, including when upgrading an existing auth schema.
