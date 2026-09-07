# Production clean-slate reset — 2026-09-07

Trendinary is pre-user and the pre-v0.5.3 detector persisted contaminated trend identities, memberships, snapshots, propagation, Radar state, editorial state, and other derived data while clustering/admission defects were present.

For v0.5.4, production intentionally performs one full Turso application-data reset identified by:

```text
2026-09-07-v0.5.4-clean-slate
```

`TRENDINARY_RESET_DATABASE_ID` is temporarily set in `fly.toml`. On startup Trendinary:

1. opens the configured Turso database;
2. creates/checks the reset ledger;
3. drops every non-SQLite user table and view except the reset ledger;
4. records the reset ID;
5. closes and reopens the Turso store so current migrations recreate the base history schema;
6. lets editorial, Following/Radar, Web Push, source intelligence, and other domain stores recreate their schemas normally.

The reset is idempotent for the same reset ID. It intentionally removes all application data because there are no production users to preserve yet.

After the production deploy and quality-aware smoke pass, remove `TRENDINARY_RESET_DATABASE_ID` from `fly.toml` in the next cleanup release. Do not reuse the reset ID for another destructive operation.
