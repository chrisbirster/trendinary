#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "::error::$*"
  exit 1
}

[[ -f package-lock.json ]] || fail "package-lock.json is required"

# Release-critical JavaScript installs must consume the exact committed graph.
for path in Dockerfile .github/workflows/ci.yml .github/workflows/release.yml .github/workflows/deploy.yml; do
  if grep -En '(^|[[:space:]])npm install([[:space:]]|$)' "$path" >/dev/null; then
    grep -En '(^|[[:space:]])npm install([[:space:]]|$)' "$path" || true
    fail "$path contains mutable 'npm install'; use npm ci"
  fi
done

# Repository-local reusable workflows use ./ paths and are trusted by the same
# commit. Every external action must be pinned to a full immutable commit SHA.
bad_actions="$(grep -RhE '^[[:space:]]*-[[:space:]]+uses:[[:space:]]+[^.]' .github/workflows \
  | grep -Ev '@[0-9a-f]{40}([[:space:]]*(#.*)?)$' || true)"
if [[ -n "$bad_actions" ]]; then
  printf '%s\n' "$bad_actions"
  fail "external GitHub Actions must be pinned to 40-character commit SHAs"
fi

echo "supply-chain policy: lockfile present, frozen installs enforced, external Actions immutable"
