# Release process

Trendinary uses a strict feature → dev → main flow so production code always maps to an explicit release.

## Branch roles

### `main`

- Production/release branch.
- Must remain deployable.
- Receives normal changes only through release PRs from `dev`.
- Every merge to `main` must represent a new release version.
- Every production deployment must correspond to a Git tag and GitHub Release.

### `dev`

- Always-green integration branch.
- All feature work starts from `dev` and returns to `dev` through a pull request.
- CI runs on every pull request and every push to `dev`.

### `feature/*`

- Short-lived work branches created from the current `dev` head.
- Pull requests target `dev`, never `main`.
- Delete after merge when practical.

## Release versioning

Trendinary uses Semantic Versioning. The intended release version must agree across `VERSION`, `package.json`, and a matching dated section in `CHANGELOG.md`. Tags and GitHub Releases use a `v` prefix.

## Database and production prerequisites

Fly is stateless and production persistence is Turso/libSQL. Atlas is the sole schema owner.

The Fly app `trendinary` owns the production database credentials:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

Those credentials are not duplicated into GitHub Actions. Short-lived Atlas Machines are created inside the existing Fly app, inherit the app secrets, run one plan/apply operation, and are destroyed after their process exits.

GitHub's protected `production` environment needs only the orchestration/deployment secrets:

```text
FLY_API_TOKEN
CLOUDFLARE_API_TOKEN
CLOUDFLARE_DEFAULT_ACCOUNT_ID
```

`fly.toml` sets `TRENDINARY_REQUIRE_TURSO=1`, so the Go process fails closed rather than using an ephemeral local database.

See `docs/turso.md` for database setup and `docs/recovery.md` for recovery qualification.

## Preparing a release

1. Make sure `dev` CI is green.
2. Confirm the Fly Turso secrets and GitHub production deployment credentials are configured.
3. Create `feature/release-X.Y.Z` from `dev`.
4. Update `VERSION`, `package.json`, and `CHANGELOG.md`.
5. Merge release prep to `dev` after CI passes.
6. Open `dev` → `main` as `Release vX.Y.Z`.
7. Merge only after the release PR is green.

Merging to `main` automatically runs the **Release** workflow.

## Release workflow ordering

The release workflow checks schema access and safety before creating a version tag:

```text
production environment
        ↓
verify Fly Turso secret names
        ↓
Atlas dry-run in one-off Fly Machine
        ↓
expand-only plan policy
        ↓
full release verification
        ↓
create/verify vX.Y.Z tag
        ↓
create/verify GitHub Release
        ↓
production deployment
```

`scripts/verify-atlas-plan.py` rejects ordinary releases containing destructive or backward-incompatible operations such as table/column drops, table/column renames, SQLite table-rebuild patterns, truncation, and new uniqueness constraints on existing tables. A unique constraint created together with a brand-new table is allowed.

The release verification also runs dependency/security gates, repeated Go tests, race/integration tests, real Atlas→libSQL topology tests, the backup/restore recovery drill, and production Docker/Playwright verification.

## Production deployment ordering

Production deployment is serialized with `cancel-in-progress: false`:

```text
verify published release tag
        ↓
verify Fly Turso secret names
        ↓
re-plan live production schema in Fly
        ↓
expand-only plan policy
        ↓
Atlas apply in one-off Fly Machine
        ↓
Fly application deploy
        ↓
Cloudflare deploy
        ↓
public smoke + exact release/commit identity
```

The second plan check is deliberate: production may have changed between release preflight and deploy. A changed live plan must still be additive before Atlas can apply it.

Normal Trendinary application Machines never create, alter, drop, or reset schema during startup.

## Schema evolution: expand → deploy → contract

All schema evolution starts in `schema/trendinary.sql`, but desired-state changes must be rolled out in compatibility phases.

### Expand

Ship only changes both the old and new application releases can tolerate. Examples include nullable columns, columns with safe defaults, new tables, indexes/constraints for a table created in the same plan, and parallel representations that leave the old representation usable.

### Deploy

Deploy the application that understands the expanded schema. If Atlas succeeds but application deployment fails, keep the additive schema and roll back/fix the application. Do not automatically downgrade a shared database.

### Contract

Remove or rename old representations only after old Machines/jobs are gone, backfills are verified, and rollback requirements are understood.

Contract/destructive changes cannot use the normal release path. They must use the manual **Production schema maintenance** workflow. That workflow:

1. runs only from `main` in the protected `production` environment;
2. requires the exact confirmation phrase `APPLY DESTRUCTIVE SCHEMA` and a non-empty reason;
3. requires the live Atlas plan to actually contain destructive/contract operations;
4. records actor, commit, ref, reason, and plan in the Actions audit summary;
5. shares the same `trendinary-production` concurrency lock as normal deploys;
6. applies the explicitly approved plan through a one-off Fly Atlas Machine;
7. re-plans and requires convergence;
8. runs production smoke afterward.

## Rollback after an additive Atlas apply

If Atlas succeeds but Fly/Cloudflare/smoke fails:

1. do not reverse the database automatically;
2. stop the rollout;
3. redeploy the previously published app tag or fix forward;
4. verify `/api/v1/healthz`, `/api/v1/readyz`, runtime health, and production smoke;
5. leave eventual contract cleanup for a later maintenance window.

## Recovery qualification

CI runs `scripts/test-recovery-drill.sh` against persistent local libSQL. It writes runtime data, restores the database into a fresh server, reconciles it to the current Atlas desired state, verifies sentinel data, boots the production image, verifies exact runtime identity, and executes the deployment-critical smoke contract.

That proves the repository procedure. It does not prove the production Turso account has point-in-time recovery/export/restore configured and tested. The real production recovery capability must be verified before v1.0.0; see `docs/recovery.md`.

## Hotfixes

For a production-critical fix:

1. branch `hotfix/*` from `main`;
2. bump PATCH version/changelog when a prior tag exists;
3. open the hotfix PR directly to `main`;
4. require the same CI/release gates;
5. merge only when exact-head checks are green;
6. immediately sync the released `main` commit back into `dev`.

## Release record

A release is complete only when GitHub Release, Git tag, `VERSION`/`package.json`, and `CHANGELOG.md` agree. Production is healthy only after Atlas apply, Fly deploy, Cloudflare deploy, exact runtime identity verification, and public smoke all pass.
