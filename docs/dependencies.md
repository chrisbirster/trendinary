# Dependency and supply-chain policy

Trendinary release builds must be reproducible from a Git commit. JavaScript dependencies are therefore resolved by the committed `package-lock.json`, Go dependencies by `go.mod`/`go.sum`, and external GitHub Actions by immutable commit SHA.

## JavaScript

npm is the canonical package manager.

Fresh checkout / CI / Docker / release:

```bash
npm ci
```

Do not use `npm install` in release-critical workflows or Docker builds. `npm ci` fails when `package.json` and `package-lock.json` disagree and installs exactly the committed dependency graph.

To intentionally update dependencies:

```bash
npm install <package>@<version>
# or, for a full intentional refresh:
npm update
npm run security:node
npm run verify
```

Commit `package.json` and `package-lock.json` together. Review lockfile changes as code: unexpected new packages, install scripts, registries, or large transitive changes need explanation.

The release gate runs:

```bash
npm audit --omit=dev --audit-level=high
```

A reachable high/critical production dependency finding blocks the release. If upstream has no safe version, document the affected package/advisory, why Trendinary is or is not exposed, the compensating control, and an expiry/review date before allowing an exception.

## Go

`go.mod` and `go.sum` are committed and CI runs `go mod tidy` plus a clean-diff check.

The vulnerability gate is pinned to:

```bash
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

Govulncheck is pinned so the scanner implementation itself does not silently change between builds. Its vulnerability database is intentionally current at execution time.

Intentional Go updates should use normal Go module tooling, then run:

```bash
go mod tidy
npm run security:go
npm run verify
```

Commit `go.mod` and `go.sum` together.

## GitHub Actions

External Actions used by repository workflows must be pinned to a full 40-character commit SHA. A nearby comment records the human-readable release/tag used when the SHA was selected, for example:

```yaml
- uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4
```

To update an Action:

1. Resolve the desired upstream release/tag to its current commit SHA from the official repository.
2. Review the upstream release/change history.
3. Replace the SHA and update the comment.
4. Run the full PR CI/release topology gate.

Never replace an immutable pin with `@main`, `@master`, `@latest`, or a mutable major/version tag.

## Enforcement

`scripts/verify-supply-chain.sh` is the executable policy. CI and the release/deploy workflows require:

- `package-lock.json` exists;
- release-critical JavaScript installs use `npm ci`;
- every external GitHub Action is pinned to an immutable commit SHA;
- npm production dependency audit passes;
- govulncheck passes.

The Docker web build copies both `package.json` and `package-lock.json` before `npm ci`, so CI and the production image consume the same dependency graph.
