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

The repository tracks the intended release version in both:

- `VERSION`
- `package.json`

CI fails if they disagree.

Git tags and GitHub Releases use a `v` prefix, for example `v0.2.0`.

## Production prerequisites

Fly is stateless and production persistence is Turso/libSQL. Before a release can deploy, the Fly app `trendinary` must have these application secrets:

```text
TURSO_DATABASE_URL
TURSO_AUTH_TOKEN
```

`fly.toml` sets `TRENDINARY_REQUIRE_TURSO=1`, so the process also fails closed at startup if Turso is not configured. There is no Fly database volume prerequisite.

GitHub's `production` environment still requires the deployment credentials used by the release workflow, including `FLY_API_TOKEN`, `CLOUDFLARE_API_TOKEN`, and `CLOUDFLARE_DEFAULT_ACCOUNT_ID`.

See `docs/turso.md` for the one-time database setup.

## Preparing a release

1. Make sure `dev` CI is green.
2. Confirm the production Turso database and Fly secrets are configured.
3. Create a small `feature/release-X.Y.Z` branch from `dev`.
4. Update `VERSION` and `package.json` to `X.Y.Z`.
5. Move completed entries from `CHANGELOG.md` → `[Unreleased]` into a dated `[X.Y.Z]` section.
6. Merge that release-prep feature back into `dev` after CI passes.
7. Open a PR from `dev` to `main` titled `Release vX.Y.Z`.
8. Merge only after the release PR is green.

Merging the release PR to `main` automatically runs the **Release** workflow.

The Release workflow:

- derives the release version from `VERSION`;
- validates SemVer and that `VERSION` matches `package.json`;
- verifies a matching changelog section exists;
- reruns the complete application and Docker verification gate;
- creates or verifies annotated tag `vX.Y.Z` at the exact `main` commit;
- creates the GitHub Release from the changelog section;
- invokes production deployment using that exact release tag.

Production deployment then verifies that the exact published tag is checked out and that the required Turso Fly secrets exist before deploying the Go origin and Cloudflare edge.

The workflow is idempotent and can be manually re-run from `main` as a recovery path when a tag or release already exists at the same commit.

After the release, fast-forward/sync `dev` to the new `main` merge commit before accepting the next feature train if needed.

## Production deployment rule

Never deploy an arbitrary branch or untagged commit to production.

The production workflow accepts a release tag, verifies that a published GitHub Release exists for it, and checks out that exact tag before deploying Fly.io and Cloudflare infrastructure. This makes production reproducible and lets us answer:

> What code is live?

with one exact release such as `v0.3.0`.

A manual production redeploy is allowed only by supplying an existing published release tag.

## Hotfixes

For a production-critical fix:

1. Branch `hotfix/*` from `main`.
2. Bump to a new PATCH version and update `CHANGELOG.md` on the hotfix branch.
3. Open the hotfix PR to `main` and require CI.
4. Merging the hotfix automatically tags/releases/deploys that PATCH version.
5. Immediately sync the released `main` commit back into `dev` before normal feature work continues.

Hotfixes are the exception to feature → dev → main; they must never leave `dev` missing a production fix.

## Release record

A release is considered complete only when all four records agree:

1. GitHub Release: `vX.Y.Z`
2. Git tag: `vX.Y.Z`
3. `VERSION` / `package.json`: `X.Y.Z`
4. `CHANGELOG.md`: dated `[X.Y.Z]` section

That is Trendinary's source of truth for release history.
