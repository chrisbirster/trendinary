#!/bin/sh
set -eu

: "${TURSO_DATABASE_URL:?TURSO_DATABASE_URL is required}"
: "${TURSO_AUTH_TOKEN:?TURSO_AUTH_TOKEN is required}"

MODE="${GOOSE_MODE:-apply}"
case "$MODE" in
  plan|apply) ;;
  *)
    echo "unknown GOOSE_MODE=${MODE}; expected plan or apply" >&2
    exit 2
    ;;
esac

export GOOSE_MIGRATION_DIR=/workspace/migrations
export NO_COLOR=1

if [ "$MODE" = "plan" ]; then
  goose validate
  exec migrationplan
fi

case "$TURSO_DATABASE_URL" in
  *\?*) DBSTRING="${TURSO_DATABASE_URL}&authToken=${TURSO_AUTH_TOKEN}" ;;
  *) DBSTRING="${TURSO_DATABASE_URL}?authToken=${TURSO_AUTH_TOKEN}" ;;
esac

export GOOSE_DRIVER=turso
export GOOSE_DBSTRING="$DBSTRING"
exec goose up
