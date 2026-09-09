# Local release gate

Trendinary must prove the production topology locally before any release is deployed.

## Database ownership

Application processes do not migrate the database. Atlas is the only schema owner.

The production order is:

```text
Atlas schema apply -> Fly deploy -> production smoke
```

Normal `trendinary` startup opens the existing database, performs a read-only schema compatibility check, and starts serving. If Atlas has not prepared the schema, startup fails with an explicit `database schema is not migrated` error instead of creating tables itself.

The desired schema lives in `schema/trendinary.sql`. Atlas configuration lives in `atlas.hcl`.

## Database reset

Database destruction is never part of application startup. The only supported reset command is:

```bash
trendinary db reset
```

The command drops application-owned tables and views and exits. It deliberately does **not** recreate schema. After a reset, run Atlas before starting Trendinary:

```bash
atlas schema apply --env production --auto-approve
```

So a clean-slate recovery is always:

```text
trendinary db reset
Atlas schema apply
trendinary
```

`TRENDINARY_RESET_DATABASE_ID` is legacy and intentionally ignored. CI fails if it appears in `fly.toml`.

## Schema changes

Edit `schema/trendinary.sql` to describe the desired database. Atlas compares the current Turso schema with that desired state and generates the required SQLite/libSQL changes.

For example, changing a table from:

```sql
CREATE TABLE users (
  id TEXT PRIMARY KEY
);
```

to:

```sql
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  this_should_boolean INTEGER NOT NULL DEFAULT 0
);
```

causes Atlas to plan the compatible schema change. Review a production plan with:

```bash
npm run db:schema:plan
```

Apply it with:

```bash
npm run db:schema:apply
```

Both commands require `TURSO_DATABASE_URL` and `TURSO_AUTH_TOKEN`. The production GitHub environment carries the same two credentials so the deploy workflow can migrate Turso before Fly starts the new release.

## Fast development verification

```bash
npm install
npm run verify
```

This performs TypeScript type checking, the normal Go suite, and production builds.

## Local release verification

Docker must be running. Then execute:

```bash
npm run verify:release
```

The release gate performs:

1. TypeScript type checking.
2. The Go suite ten times to expose intermittent concurrency failures.
3. The Go race detector.
4. Process-level integration tests proving a blank DB is rejected and an externally prepared DB starts cleanly.
5. Twenty restart cycles proving normal startup does not mutate schema.
6. Production web/server builds.
7. A real local libSQL server using `ghcr.io/tursodatabase/libsql-server:latest`.
8. Atlas applying `schema/trendinary.sql` to that real libSQL server.
9. `trendinary db reset` dropping the schema without recreating it.
10. A negative startup test proving the app refuses the reset/unmigrated database.
11. Atlas applying the schema again.
12. Two production Docker containers booting against that one Atlas-prepared shared database.
13. Playwright browser tests against the actual production Docker image after Atlas migration.

The browser tests cover the empty production leaderboard, absence of prototype/demo trends, SPA deep links, and coherent trend-detail source rendering.

## Individual gates

```bash
npm run test:unit
npm run test:race
npm run test:integration
npm run test:libsql
npm run test:e2e:install
npm run test:e2e:docker
```

## Manual localhost inspection

The easiest production-like local test is the real-libSQL gate:

```bash
npm run test:libsql
```

For interactive inspection, start a local libSQL server, apply the Atlas schema, and run the production image against it. `scripts/test-e2e-docker.sh` is the executable reference for that topology.

Then inspect the running app with:

```bash
curl -fsS http://127.0.0.1:8080/api/v1/healthz | jq
curl -fsS http://127.0.0.1:8080/api/v1/trends | jq
open http://127.0.0.1:8080
```

For live-source inspection, enable the scanner/Jetstream and provide only the credentials you intentionally want to exercise.

## Release rule

Do not deploy if any local release gate fails. An empty leaderboard is valid immediately after a clean reset. Once evidence arrives, every public trend must continue to satisfy the independent-publisher/community-actor quality gates; fake, contaminated, single-source, or unresponsive output is not acceptable.
