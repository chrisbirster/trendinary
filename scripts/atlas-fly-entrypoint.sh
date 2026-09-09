#!/bin/sh
set -eu

: "${TURSO_DATABASE_URL:?TURSO_DATABASE_URL is required}"
: "${TURSO_AUTH_TOKEN:?TURSO_AUTH_TOKEN is required}"

case "${ATLAS_MODE:-apply}" in
  plan)
    exec atlas schema apply --env production --dry-run
    ;;
  apply)
    exec atlas schema apply --env production --auto-approve
    ;;
  *)
    echo "unknown ATLAS_MODE=${ATLAS_MODE}; expected plan or apply" >&2
    exit 2
    ;;
esac
