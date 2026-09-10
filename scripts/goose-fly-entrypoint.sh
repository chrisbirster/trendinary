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

case "$TURSO_DATABASE_URL" in
  *\?*) DBSTRING="${TURSO_DATABASE_URL}&authToken=${TURSO_AUTH_TOKEN}" ;;
  *) DBSTRING="${TURSO_DATABASE_URL}?authToken=${TURSO_AUTH_TOKEN}" ;;
esac

export GOOSE_DRIVER=turso
export GOOSE_DBSTRING="$DBSTRING"
export GOOSE_MIGRATION_DIR=/workspace/migrations
export NO_COLOR=1

if [ "$MODE" = "plan" ]; then
  goose validate
  goose status
  echo
  echo "Pending migration SQL:"
  current="$(goose version 2>&1 | sed -n 's/^goose: version //p' | tail -n 1)"
  case "$current" in
    ''|*[!0-9]*) current=0 ;;
  esac
  for file in /workspace/migrations/*.sql; do
    [ -f "$file" ] || continue
    base="${file##*/}"
    version="${base%%_*}"
    case "$version" in
      ''|*[!0-9]*) continue ;;
    esac
    if [ "$version" -gt "$current" ]; then
      echo
      echo "--- ${base} ---"
      awk '
        /^--[[:space:]]+\+goose[[:space:]]+Down/ { exit }
        /^--[[:space:]]+\+goose[[:space:]]+Up/ { in_up=1; next }
        in_up { print }
      ' "$file"
    fi
  done
  exit 0
fi

exec goose up
