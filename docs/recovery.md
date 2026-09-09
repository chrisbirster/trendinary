# Database recovery

Trendinary production persistence is Turso/libSQL and Atlas is the sole schema owner. Recovery is a data operation first and a schema-reconciliation operation second.

## Repository recovery drill

`npm run test:recovery` proves the repository-level procedure using persistent local libSQL:

1. start libSQL on a persistent volume;
2. apply `schema/trendinary.sql` with Atlas;
3. write durable sentinel rows through the normal runtime connection;
4. stop libSQL cleanly;
5. copy the persistent database into a fresh volume/server;
6. reconcile the restored database forward with the current Atlas desired state;
7. verify all sentinel data is present;
8. boot the production Docker image with exact release/commit metadata;
9. require `/healthz`, `/readyz`, runtime health, push configuration, and exact identity to pass the production smoke contract.

CI runs this drill on every v1-readiness change.

## Production recovery requirement

The local drill is necessary but not sufficient for v1. Before v1.0.0, the real Turso production database must have a tested recovery path. Record the following evidence in the v1 readiness issue:

- the recovery mechanism available for the production database/account (for example point-in-time restore, backup/export, or provider-supported restore);
- the documented retention/window relevant to Trendinary;
- the exact operator command/UI path used to create a non-production restored copy;
- verification that the restored copy contains known Trendinary data;
- Atlas reconciliation against the restored copy;
- successful Trendinary readiness/smoke against the restored copy;
- the date of the drill and the actor who performed it.

Do not claim production recovery is verified based only on the local libSQL drill.

## Recovery procedure

For an incident where data must be restored:

1. stop or isolate writers before selecting the recovery point;
2. create a restored copy using the Turso-supported recovery mechanism; do not overwrite the only surviving source copy before validation;
3. point an authorized recovery environment at the restored database;
4. run an Atlas plan against the restored copy;
5. review the plan using the same expand/contract policy as production;
6. reconcile the restored copy to the application version that will be deployed;
7. verify known data and application readiness;
8. only then cut production over using an explicit incident/change record;
9. run the full production smoke and verify exact release/commit identity.

## Rollback semantics

An additive Atlas migration is not automatically rolled back when an application deployment fails. Keep the expanded schema and roll back/fix the application. Destructive contract operations require the dedicated audited schema-maintenance workflow and a recovery plan before execution.

## Clean-slate reset

`trendinary db reset` is drop-only and intentionally destructive. It is not a normal recovery mechanism. After a controlled reset, Atlas must recreate the desired schema before any Trendinary application process may start.
