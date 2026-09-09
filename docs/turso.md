# Turso production database

Trendinary production uses Turso/libSQL as its durable relational database. Fly.io runs the Go origin as a stateless compute host; no Fly volume is required.

## Why

The scanner, Jetstream cursor, historical score baselines, stable trend identities, NewsData quota state, Following/Radar state, and private editorial workspace all need durable SQL state. Keeping that state in Turso means a Fly Machine can restart or be replaced without moving a local database file.

Local development and tests still use SQLite/libSQL-compatible databases, but schema ownership is the same everywhere: Atlas prepares the database before Trendinary starts.

## Schema ownership

`schema/trendinary.sql` is the desired application schema and `atlas.hcl` defines how Atlas applies it.

The ownership boundary is strict:

```text
trendinary db reset      # explicit destructive drop of Trendinary-owned schema only
        ↓
Atlas schema apply       # owns CREATE / ALTER / schema evolution
        ↓
trendinary               # runtime opens existing schema; no migrations
```

Normal application startup never creates, alters, drops, or upgrades database schema. It performs a read-only compatibility check and exits with `database schema is not migrated` when the required schema has not been prepared.

The runtime connection also blocks legacy defensive DDL from reaching SQLite/libSQL. This lets older package constructors remain harmless while Atlas stays the only schema writer.

## One-time Turso setup

Install and authenticate the Turso CLI:

```bash
brew install tursodatabase/tap/turso
turso auth login
```

Create the production database:

```bash
turso db create trendinary
```

Read its libSQL URL:

```bash
turso db show trendinary --url
```

Create a database auth token:

```bash
turso db tokens create trendinary --expiration never
```

Store both values as Fly application secrets because the running Go service needs them for normal data access:

```bash
fly secrets set \
  TURSO_DATABASE_URL='libsql://YOUR-DATABASE.turso.io' \
  TURSO_AUTH_TOKEN='YOUR-DATABASE-TOKEN' \
  -a trendinary
```

Store the same two values as secrets in GitHub's `production` environment. GitHub Actions needs them so Atlas can plan/apply the production schema before Fly deployment.

Do not put either credential in `fly.toml`, Git history, GitHub Actions logs, or client-side JavaScript.

## Production safety

`fly.toml` sets:

```text
TRENDINARY_REQUIRE_TURSO=1
```

At startup the Go process therefore requires `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`. If they are missing, startup fails. Production never falls back to a local database file.

The release flow adds two additional guards:

1. Before a release tag is created, GitHub's `production` environment is opened and Atlas performs a read-only production schema plan. Missing credentials or an invalid plan stop the release before version tagging.
2. During deployment, the workflow verifies the Turso Fly runtime secret names, applies the schema with Atlas, deploys Fly, deploys Cloudflare, and only then runs the production smoke gate.

Production schema/deploy runs are serialized so two releases cannot apply/deploy concurrently.

## Local development

The default local database is `trendinary.db`. Prepare it through the Atlas `local` environment:

```bash
npm run db:schema:local:plan
npm run db:schema:local:apply
npm run dev:api
```

A blank database is intentionally not bootstrapped by `npm run dev:api`.

To test against Turso intentionally:

```bash
export TURSO_DATABASE_URL='libsql://YOUR-DATABASE.turso.io'
export TURSO_AUTH_TOKEN='YOUR-DATABASE-TOKEN'
npm run db:schema:plan
npm run db:schema:apply
npm run dev:api
```

Review the plan before applying it to any shared database.

## Explicit reset

Database destruction is never a startup behavior. The only supported destructive path is the explicit command:

```bash
trendinary db reset
```

For a source checkout you can use:

```bash
go run ./cmd/trendinary db reset
```

The command uses a schema-mutating administrative connection, drops only the explicitly declared Trendinary-owned application tables plus the obsolete legacy reset ledger, and exits. It does not discover-and-drop unrelated/provider tables, and it does not recreate anything.

After reset, ordinary Trendinary startup must fail until Atlas reapplies the desired schema.

For the default local database:

```bash
go run ./cmd/trendinary db reset
npm run db:schema:local:apply
npm run dev:api
```

For production/shared Turso, use the production Atlas plan/apply path and the normal release/recovery controls rather than treating reset as an application-start flag.

`TRENDINARY_RESET_DATABASE_ID` is legacy and intentionally inert.

## Recovery

Use Turso point-in-time recovery for production recovery. Turso maintains recovery history according to the account plan and can restore a database from a selected point in time.

For an offline/exported copy, use Turso's database export or shell dump tooling rather than attempting to copy files from Fly.

The local command:

```bash
trendinary backup ./trendinary-backup.db
```

uses SQLite `VACUUM INTO` and is intentionally supported only for local SQLite stores. It returns an error when the application is connected to Turso.

## R2

Cloudflare R2 remains Trendinary's raw/provenance archive for source payloads, replay fixtures, and generated exports. Database durability and recovery are handled by Turso rather than a Fly-volume-to-R2 snapshot pipeline.

See `docs/local-release-gate.md` for the executable Atlas/libSQL release topology and `docs/release-process.md` for the release ordering.
