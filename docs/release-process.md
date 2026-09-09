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

See `docs/turso.md` for setup and schema ownership.

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

The release workflow deliberately checks production schema access **before** it creates a version tag:

```text
production environment
        ↓
verify Turso Atlas credentials
        ↓
Atlas schema dry-run / plan
        ↓
full release verification
        ↓
create/verify vX.Y.Z tag
        ↓
create/verify GitHub Release
        ↓
production deployment
```

The preflight prevents a missing Atlas credential or invalid production schema plan from burning a release version/tag.

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
Atlas schema apply
        ↓
Fly deploy
        ↓
Cloudflare deploy
        ↓
production smoke
```

Atlas finishes before any new Trendinary Fly Machine is started. Application Machines never perform migrations or resets during startup.

A manual production redeploy is allowed only by supplying an existing published release tag.

## Schema changes

All schema evolution starts in `schema/trendinary.sql`. Review the production plan with:

```bash
npm run db:schema:plan
```

Apply only through the controlled production deployment/recovery path:

```bash
npm run db:schema:apply
```

For the default local SQLite database use:

```bash
npm run db:schema:local:plan
npm run db:schema:local:apply
```

`trendinary db reset` is an explicit destructive drop-only administrative command. It never recreates schema. After a reset, Atlas must prepare the database before ordinary startup can succeed.

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

Production is considered healthy only after the tagged release has passed Atlas apply, Fly deploy, Cloudflare deploy, and the public smoke gate.
