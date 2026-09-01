# Turso production database

Trendinary production uses Turso/libSQL as its durable relational database. Fly.io runs the Go origin as a stateless compute host; no Fly volume is required.

## Why

The scanner, Jetstream cursor, historical score baselines, stable trend identities, NewsData quota state, and private editorial workspace all need durable SQL state. Keeping that state in Turso means a Fly machine can restart or be replaced without moving a local database file.

Local development and tests still default to SQLite so contributors do not need a remote database.

## One-time setup

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

Store both values as Fly application secrets:

```bash
fly secrets set \
  TURSO_DATABASE_URL='libsql://YOUR-DATABASE.turso.io' \
  TURSO_AUTH_TOKEN='YOUR-DATABASE-TOKEN' \
  -a trendinary
```

Do not put either credential in `fly.toml`, Git history, GitHub Actions logs, or client-side JavaScript.

## Production safety

`fly.toml` sets:

```text
TRENDINARY_REQUIRE_TURSO=1
```

At startup the Go process therefore requires `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`. If they are missing, startup fails. Production never falls back to a local database file.

The release deploy workflow also checks that both Fly secret names exist before it deploys a release tag.

## Local development

With no Turso environment variables, Trendinary uses local SQLite:

```bash
npm run dev:api
```

To test against Turso intentionally:

```bash
export TURSO_DATABASE_URL='libsql://YOUR-DATABASE.turso.io'
export TURSO_AUTH_TOKEN='YOUR-DATABASE-TOKEN'
npm run dev:api
```

## Schema ownership

The Go process owns application schema creation through idempotent `CREATE TABLE IF NOT EXISTS` migrations. Public trend/history tables and private editorial tables share the same Turso database but remain separate application domains.

No D1 database participates in the Go request/scanner path.

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
