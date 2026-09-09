# Turso production database

Trendinary production uses Turso/libSQL as its durable relational database. Fly.io runs the Go origin as stateless compute; no Fly volume is required.

## Schema ownership

`schema/trendinary.sql` is the desired application schema and `atlas.hcl` defines how Atlas applies it.

```text
trendinary db reset      # explicit destructive drop of Trendinary-owned schema only
        ↓
Atlas schema apply       # sole CREATE / ALTER / schema evolution owner
        ↓
trendinary               # runtime opens existing schema; no migrations
```

Normal application startup never creates, alters, drops, or upgrades database schema. It performs a read-only compatibility check and exits with `database schema is not migrated` when Atlas has not prepared the required schema.

## One-time Turso setup

Install/authenticate the Turso CLI, create the database, read its URL, and create a database token:

```bash
brew install tursodatabase/tap/turso
turso auth login
turso db create trendinary
turso db show trendinary --url
turso db tokens create trendinary --expiration never
```

Store the URL/token only as Fly application secrets:

```bash
fly secrets set \
  TURSO_DATABASE_URL='libsql://YOUR-DATABASE.turso.io' \
  TURSO_AUTH_TOKEN='YOUR-DATABASE-TOKEN' \
  -a trendinary
```

The normal Go application and short-lived Atlas migration Machines in the same Fly app inherit those secrets. Do **not** duplicate the Turso credentials into GitHub Actions. GitHub production workflows need `FLY_API_TOKEN` to create/inspect the migration Machines, but Atlas executes inside Fly.

Never put Turso credentials in `fly.toml`, Git history, Actions logs, or client JavaScript.

## Production migration topology

`Dockerfile.migrate` packages Atlas plus the desired schema. `scripts/run-atlas-fly-machine.sh` launches one short-lived Machine inside the `trendinary` Fly app, waits for the Atlas process exit event, reads the actual exit code, and destroys the Machine afterward.

Release preflight uses:

```bash
npm run db:schema:fly:plan
```

Production deployment uses the same plan again, enforces the expand-only policy, and only then runs:

```bash
npm run db:schema:fly:apply
```

Normal web Machines never participate in migration.

## Production safety

`fly.toml` sets `TRENDINARY_REQUIRE_TURSO=1`, so production never falls back to an ephemeral local database.

Before a release tag can be created:

1. GitHub verifies Fly contains the required Turso secret names.
2. A one-off Atlas Machine performs a production dry-run.
3. `scripts/verify-atlas-plan.py` rejects destructive/backward-incompatible plans.

Deployment repeats the live plan immediately before apply so drift between release preflight and deploy cannot turn an approved additive plan into an unreviewed destructive one.

Production schema/deploy operations share one concurrency lock.

## Local development

Prepare the default local `trendinary.db` through Atlas:

```bash
npm run db:schema:local:plan
npm run db:schema:local:apply
npm run dev:api
```

To work directly against an intentionally configured non-production Turso database from an operator workstation:

```bash
export TURSO_DATABASE_URL='libsql://YOUR-DATABASE.turso.io'
export TURSO_AUTH_TOKEN='YOUR-DATABASE-TOKEN'
npm run db:schema:plan
npm run db:schema:apply
npm run dev:api
```

## Explicit reset

Database destruction is never startup behavior. `trendinary db reset` uses an administrative connection, drops only explicitly declared Trendinary-owned objects, and exits. It never recreates schema.

After a reset, Atlas must prepare the database before Trendinary can start.

Do not use reset as the normal production recovery mechanism.

## Recovery

CI proves the repository restore procedure using persistent real libSQL via `npm run test:recovery`. Production recovery is a separate operational qualification: the actual Turso account/database recovery mechanism, retention window, restored-copy validation, Atlas reconciliation, and application smoke must be exercised before v1.0.0.

See `docs/recovery.md` for the required production recovery evidence. Do not claim the production database is recoverable merely because local libSQL restore tests pass.

## R2

Cloudflare R2 remains Trendinary's raw/provenance archive for source payloads, replay fixtures, and generated exports. Database durability/recovery is handled by Turso rather than copying files from Fly.

See `docs/local-release-gate.md` and `docs/release-process.md` for the executable release topology.
