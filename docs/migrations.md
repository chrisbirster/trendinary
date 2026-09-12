# Database migrations

Trendinary uses versioned Goose SQL migrations. Normal application processes never create, alter, or drop schema.

## Ownership

- `migrations/*.sql` is the schema history and source of truth.
- Goose applies migrations outside the application lifecycle.
- `cmd/migrationplan` is the read-only production planner. It reads `sqlite_master` and `goose_db_version` and prints pending migration SQL without creating the Goose ledger or running DDL.
- `trendinary db reset` drops only Trendinary-owned application tables, the legacy reset table, and `goose_db_version`. It does not recreate schema.

## Local

Validate migration files without touching a database:

```bash
npm run db:schema:local:plan
```

Apply pending migrations to `trendinary.db`:

```bash
npm run db:schema:local:apply
```

The real libSQL release topology test is:

```bash
npm run test:libsql
```

It runs Goose against a libSQL server, resets the database, verifies a normal Trendinary process refuses the unmigrated database, reapplies Goose migrations, and starts two non-migrating application processes against the shared database.

## Production

Production Turso credentials remain Fly app secrets. GitHub only needs `FLY_API_TOKEN` to orchestrate the one-off migration Machine.

Read-only plan:

```bash
npm run db:schema:fly:plan
```

Apply pending migrations:

```bash
npm run db:schema:fly:apply
```

Do not apply a production migration until the plan has been reviewed.

## Release branch flow

```text
feature/* -> dev -> vX.Y.Z tag -> release/deploy -> dev -> main
```

A release tag must point to a commit reachable from `dev`. A `dev -> main` pull request is accepted only when a `v*` tag points at the exact `dev` head being promoted.
