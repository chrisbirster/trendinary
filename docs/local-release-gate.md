# Local release gate

Trendinary must prove the production topology locally before any release is deployed.

## Database ownership

Application processes do not migrate the database. Atlas is the sole schema owner.

Production uses short-lived Atlas Machines inside the existing `trendinary` Fly app:

```text
Fly Atlas dry-run Machine
        ↓
expand-only plan policy
        ↓
publish release
        ↓
Fly Atlas re-plan Machine
        ↓
expand-only plan policy
        ↓
Fly Atlas apply Machine
        ↓
Fly app deploy
        ↓
Cloudflare deploy
        ↓
production smoke
```

The migration Machines inherit the Fly app's Turso secrets. GitHub orchestrates them using `FLY_API_TOKEN`; the Turso URL/token do not need to be copied into GitHub Actions.

Normal `trendinary` startup opens the existing database, performs a read-only compatibility check, and starts serving. If Atlas has not prepared the schema, startup fails with `database schema is not migrated` instead of creating tables itself.

## Database reset

`trendinary db reset` is explicit, destructive, and drop-only. It drops only the declared Trendinary-owned application objects and exits. It never discovers unrelated/provider objects and never recreates schema.

After a local reset:

```bash
npm run db:schema:local:apply
```

Production reset is not a routine deployment or recovery operation. Use the documented recovery/maintenance controls.

## Schema changes

Edit `schema/trendinary.sql` to describe desired state. The ordinary release path is expand-only. Before a production tag can be created, Atlas plans against the live database inside Fly and `scripts/verify-atlas-plan.py` rejects destructive/backward-incompatible operations. Deployment repeats that live plan check immediately before apply.

Operator commands for the production Fly path are:

```bash
npm run db:schema:fly:plan
npm run db:schema:fly:apply
```

Direct `db:schema:plan/apply` commands remain available for intentionally configured non-production Turso databases from an operator workstation.

Destructive contract work must use the audited **Production schema maintenance** workflow; it cannot be smuggled through an ordinary release.

## Fast development verification

```bash
npm ci
npm run verify
```

## Local release verification

Docker must be running. Execute:

```bash
npm run verify:release
```

The release gate performs:

1. TypeScript type checking.
2. The Go suite ten times.
3. The Go race detector.
4. Process-level integration tests.
5. Restart/no-runtime-DDL verification.
6. Production web/server builds.
7. Atlas plan-policy fixture tests.
8. Real local libSQL on an isolated Docker network.
9. Atlas desired-state apply to real libSQL.
10. Runtime DDL interception plus normal/multi-statement DML.
11. Explicit reset followed by negative unmigrated-startup verification.
12. Atlas reapply and two concurrent non-migrating app processes.
13. Persistent libSQL backup/restore into a fresh server.
14. Atlas reconciliation of the restored database.
15. Sentinel-data verification after restore.
16. Production image boot against the restored database with exact release/commit identity.
17. Deployment-critical production smoke in recovery mode.
18. Playwright against the Atlas-prepared production Docker image.
19. Build verification for `Dockerfile.migrate`.

The automated recovery drill proves the repository recovery procedure. It does not substitute for testing the actual production Turso account/database recovery mechanism; that remains a v1 release prerequisite documented in `docs/recovery.md`.

## Individual gates

```bash
npm run test:atlas-policy
npm run test:unit
npm run test:race
npm run test:integration
npm run test:libsql
npm run test:recovery
npm run test:e2e:install
npm run test:e2e:docker
```

## Manual localhost inspection

```bash
npm run db:schema:local:apply
npm run dev:api
curl -fsS http://127.0.0.1:8080/api/v1/healthz | jq
curl -fsS http://127.0.0.1:8080/api/v1/readyz | jq
curl -fsS http://127.0.0.1:8080/api/v1/trends | jq
```

## Release rule

Do not deploy if any release gate fails. Do not treat a GitHub Actions run that failed before runner allocation as a passing gate. Production schema changes must remain additive in ordinary releases, and production is not considered healthy until exact release identity plus the public smoke contract pass.
