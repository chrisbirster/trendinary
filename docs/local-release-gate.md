# Local release gate

Trendinary must prove the production topology locally before any release is deployed.

## Database reset

Database destruction is never part of application startup. The only supported reset command is:

```bash
trendinary db reset
```

The command drops application-owned tables and views, recreates the current history schema, and exits. Stop production application processes before intentionally resetting a shared production database.

`TRENDINARY_RESET_DATABASE_ID` is legacy and intentionally ignored. CI fails if it appears in `fly.toml`.

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
4. Process-level integration tests using the compiled Trendinary binary.
5. Production web/server builds.
6. A real local libSQL server test using `ghcr.io/tursodatabase/libsql-server:latest`.
7. `trendinary db reset` against libSQL over the same remote protocol family used by Turso.
8. Two production Docker containers booting concurrently against one shared libSQL database.
9. Playwright browser tests against the actual production Docker image.

The process-level tests cover blank database boot, twenty restart cycles, two processes sharing one database, explicit reset followed by boot, and a regression proving the old startup reset environment variable cannot delete data.

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

After the release gate is green, run the production image yourself:

```bash
docker build -t trendinary:manual .
docker run --rm -p 8080:8080 \
  -e TRENDINARY_DB_PATH=/tmp/trendinary.db \
  -e TRENDINARY_SCANNER_DISABLED=1 \
  -e TRENDINARY_JETSTREAM_DISABLED=1 \
  trendinary:manual
```

Then inspect:

```bash
curl -fsS http://127.0.0.1:8080/api/v1/healthz | jq
curl -fsS http://127.0.0.1:8080/api/v1/trends | jq
open http://127.0.0.1:8080
```

For live-source inspection, omit the scanner/Jetstream disable variables and provide only the credentials you intentionally want to exercise.

## Release rule

Do not deploy if any local release gate fails. An empty leaderboard is valid. A fake, contaminated, or unresponsive leaderboard is not.
