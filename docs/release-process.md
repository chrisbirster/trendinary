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

Examples:

```text
feature/news-ingestion ─┐
feature/entity-v2 ──────┼──> dev ──release PR──> main ──tag/release──> production
feature/fomo ───────────┘
```

## Release versioning

Trendinary uses Semantic Versioning:

- `MAJOR`: incompatible product/API changes once a stable 1.x contract exists.
- `MINOR`: new product capability or substantial feature train.
- `PATCH`: backward-compatible fixes and small improvements.
- Pre-release versions may use `-alpha.N`, `-beta.N`, or `-rc.N` when appropriate.

The repository tracks the intended release version in:

- `VERSION`
- `package.json`
- a dated matching section in `CHANGELOG.md`

CI fails if the version files disagree or the changelog section is missing.

Git tags and GitHub Releases use a `v` prefix, for example `v0.5.7`.

## Database and production prerequisites

Fly is stateless and production persistence is Turso/libSQL. Atlas is the sole schema owner.

The Fly app `trendinary` must have these application runtime secrets:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

GitHub's `production` environment must also have:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
FLY_API_TOKEN
CLOUDFLARE_API_TOKEN
CLOUDFLARE_DEFAULT_ACCOUNT_ID
```

The Turso values in GitHub are used by Atlas before deployment; the copies in Fly are used by the running application for normal database access. `fly.toml` sets `TRENDINARY_REQUIRE_TURSO=1`, so the Go process fails closed rather than using an ephemeral local database.

See `docs/turso.md` for setup and schema ownership and `docs/recovery.md` for the restore contract.

## Preparing a release

1. Make sure `dev` CI is green.
2. Confirm the production Turso database, GitHub `production` environment, and Fly runtime secrets are configured.
3. Create a small `feature/release-X.Y.Z` branch from `dev`.
4. Update `VERSION` and `package.json` to `X.Y.Z`.
5. Move completed entries from `CHANGELOG.md` → `[Unreleased]` into a dated `[X.Y.Z]` section.
6. Merge that release-prep feature back into `dev` after CI passes.
7. Open a PR from `dev` to `main` titled `Release vX.Y.Z`.
8. Merge only after the release PR is green.

Merging the release PR to `main` automatically runs the **Release** workflow.

## Release workflow ordering

The release workflow deliberately checks production schema access and schema safety **before** it creates a version tag:

```text
production environment
        ↓
verify Turso Atlas credentials
        ↓
Atlas production dry-run / plan
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

The preflight prevents a missing Atlas credential, invalid plan, or destructive/contract-style schema change from burning a release version/tag. `scripts/verify-atlas-plan.py` fails ordinary releases on destructive operations such as table/column drops, table/column renames, SQLite table-rebuild patterns, truncation, and new uniqueness constraints on existing tables. A unique constraint created together with a brand-new table is allowed.

The release verification then:

- validates SemVer and `VERSION` / `package.json` / `CHANGELOG.md` agreement;
- verifies Go module lock files;
- runs the repeated Go suite, race detector, process integration tests, and builds;
- runs the real Atlas → libSQL → reset → Atlas → two-process topology gate;
- runs Playwright against the Atlas-prepared production Docker image;
- extracts the matching changelog release notes;
- creates or verifies annotated tag `vX.Y.Z` at the exact `main` commit;
- creates the GitHub Release from that changelog section.

## Production deployment ordering

Deployment accepts only an existing published release tag and is serialized so two production schema/deploy sequences cannot overlap:

```text
verify published release tag
        ↓
verify GitHub Turso credentials
        ↓
verify Fly Turso runtime secret names
        ↓
re-plan live production schema
        ↓
expand-only plan policy
        ↓
Atlas schema apply
        ↓
Fly deploy
        ↓
Cloudflare deploy
        ↓
production smoke + exact release identity
```

The second plan check is deliberate: even if preflight passed earlier, production may have changed before deployment. A changed plan must still be additive before Atlas is allowed to apply it.

Atlas finishes before any new Trendinary Fly Machine is started. Application Machines never perform migrations or resets during startup.

A manual production redeploy is allowed only by supplying an existing published release tag.

## Schema evolution: expand → deploy → contract

All schema evolution starts in `schema/trendinary.sql`, but the desired state must be rolled out in compatibility phases.

### 1. Expand

First ship only changes that both the currently deployed binary and the next binary can tolerate. Typical expand changes are:

- add a nullable column;
- add a column with a safe default;
- create a new table;
- create indexes/constraints for a table that is itself new in the same plan;
- introduce a parallel representation while continuing to read/write the old representation.

Review the production plan with:

```bash
npm run db:schema:plan
```

The ordinary release workflow runs the same Atlas plan against production and machine-enforces the expand-only policy.

### 2. Deploy

Deploy the application that understands the expanded schema. During this phase, rolling/failed deployments remain safe because both the previous release and the new release can run against the expanded database.

For data-shape changes, backfill and verify the new representation while the old representation is still available. Do not remove the old representation in the same ordinary release that introduces its replacement.

### 3. Contract

Only after the compatibility window has closed—old application Machines are gone, background jobs no longer need the old representation, data/backfills are verified, and rollback requirements are understood—may the obsolete representation be removed or renamed.

Contract/destructive schema changes are **not permitted through the ordinary release path**. They use the manual **Production schema maintenance** workflow. That workflow:

1. runs only from `main` in the protected `production` environment;
2. requires the exact confirmation phrase `APPLY DESTRUCTIVE SCHEMA` and a non-empty reason;
3. requires the plan to actually contain a destructive/contract operation—otherwise it refuses the override and directs the operator to the ordinary release path;
4. records the actor, commit, ref, reason, and Atlas plan in the GitHub Actions job summary/log;
5. shares the `trendinary-production` concurrency lock with normal deploys;
6. explicitly runs the destructive-policy override;
7. applies Atlas;
8. re-plans and requires convergence;
9. runs the public production smoke against the still-running application.

The override is therefore explicit and auditable; it is never silently inferred from an ordinary release.

## Rollback after an Atlas expand succeeds

An additive schema change is intentionally forward-compatible. If Atlas succeeds but the subsequent Fly deployment or smoke fails:

1. **Do not automatically reverse the database schema.** Keep the expanded schema in place.
2. Stop/cancel further rollout as appropriate.
3. Redeploy the previously published application tag, or fix forward with a corrected release. The previous application must have been designed to tolerate the expanded schema.
4. Verify `/api/v1/healthz`, `/api/v1/readyz`, runtime health, and the production smoke contract.
5. Investigate/fix the application deployment independently of the already-applied additive schema.
6. Leave any eventual contract cleanup for a later, explicitly approved maintenance window.

Trying to automatically downgrade a shared database during a failed rolling application deployment can destroy data written by the new representation or make surviving Machines incompatible. The expand/deploy/contract discipline exists specifically to avoid that rollback trap.

`trendinary db reset` remains an explicit destructive drop-only administrative command for controlled reset scenarios. It never recreates schema. After a reset, Atlas must prepare the database before ordinary startup can succeed.

## Recovery qualification

CI runs `scripts/test-recovery-drill.sh` against real local libSQL persistence. The drill writes runtime data, restores the database files into a new libSQL server, reconciles the restored database to the current Atlas desired schema, boots the production Docker image, checks exact release identity, and executes the deployment-critical production smoke contract.

That automated drill proves the repository procedure. It does **not** by itself prove that the production Turso account has the required point-in-time recovery/export/restore capability enabled and operational. Confirming the real production account/database recovery path remains a v1 release prerequisite; see `docs/recovery.md`.

## Hotfixes

For a production-critical fix:

1. Branch `hotfix/*` from `main`.
2. Bump to a new PATCH version and update `CHANGELOG.md` on the hotfix branch.
3. Open the hotfix PR to `main` and require the same complete CI gate as a normal release.
4. Merging the hotfix runs the production preflight, then tags/releases/deploys that PATCH version only if the preflight and release gates succeed.
5. Immediately sync the released `main` commit back into `dev` before normal feature work continues.

Hotfixes are the exception to feature → dev → main; they must never leave `dev` missing a production fix.

## Release record

A release is considered complete only when all four records agree:

1. GitHub Release: `vX.Y.Z`
2. Git tag: `vX.Y.Z`
3. `VERSION` / `package.json`: `X.Y.Z`
4. `CHANGELOG.md`: dated `[X.Y.Z]` section

Production is considered healthy only after the tagged release has passed Atlas apply, Fly deploy, Cloudflare deploy, exact runtime identity verification, and the public smoke gate.
