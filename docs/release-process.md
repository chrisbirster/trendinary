# Release process

Trendinary uses a strict feature → dev → main flow so production code always maps to an explicit release.

## Branch roles

### `main`

- Production/release branch.
- Must remain deployable.
- Receives normal changes only through release PRs from `dev`.
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
feature/entity-v2 ──────┼──> dev ──release PR──> main ──tag──> production
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

Git tags and GitHub Releases use a `v` prefix, for example `v0.1.0`.

## Preparing a release

1. Make sure `dev` CI is green.
2. Create a small `feature/release-X.Y.Z` branch from `dev`.
3. Update `VERSION` and `package.json` to `X.Y.Z`.
4. Move completed entries from `CHANGELOG.md` → `[Unreleased]` into a dated `[X.Y.Z]` section.
5. Merge that release-prep feature back into `dev` after CI passes.
6. Open a PR from `dev` to `main` titled `Release vX.Y.Z`.
7. Merge only after the release PR is green.
8. Run the **Release** GitHub Actions workflow from `main` with version `X.Y.Z`.

The Release workflow:

- validates SemVer and repository version metadata;
- verifies a matching changelog section exists;
- reruns the complete application and Docker verification gate;
- creates annotated tag `vX.Y.Z`;
- creates the GitHub Release from the changelog section;
- invokes the production deployment using that exact tag.

After the release, fast-forward/sync `dev` to the new `main` merge commit before accepting the next feature train if needed.

## Production deployment rule

Never deploy an arbitrary branch or untagged commit to production.

The production workflow accepts a release tag and checks out that exact tag before deploying Fly.io and Cloudflare infrastructure. This makes production reproducible and lets us answer:

> What code is live?

with one exact release such as `v0.3.0`.

## Hotfixes

For a production-critical fix:

1. Branch `hotfix/*` from `main`.
2. Open the hotfix PR to `main` and require CI.
3. Release a new PATCH version.
4. Immediately sync the released `main` commit back into `dev` before normal feature work continues.

Hotfixes are the exception to feature → dev → main; they must never leave `dev` missing a production fix.

## Release record

A release is considered complete only when all four records agree:

1. GitHub Release: `vX.Y.Z`
2. Git tag: `vX.Y.Z`
3. `VERSION` / `package.json`: `X.Y.Z`
4. `CHANGELOG.md`: dated `[X.Y.Z]` section

That is Trendinary's source of truth for release history.
