#!/usr/bin/env bash
set -euo pipefail

LIBSQL_CONTAINER="trendinary-libsql-$$"
APP1_CONTAINER="trendinary-libsql-app1-$$"
APP2_CONTAINER="trendinary-libsql-app2-$$"
FAIL_CONTAINER="trendinary-libsql-unmigrated-$$"
NETWORK="trendinary-libsql-net-$$"
LIBSQL_HOST="libsql.test"
LIBSQL_PORT="${TRENDINARY_LIBSQL_PORT:-18081}"
APP1_PORT="${TRENDINARY_LIBSQL_APP1_PORT:-18082}"
APP2_PORT="${TRENDINARY_LIBSQL_APP2_PORT:-18083}"
IMAGE="${TRENDINARY_LIBSQL_IMAGE:-trendinary:libsql-test}"
TOKEN="local-test-token"

cleanup() {
  docker rm -f "$FAIL_CONTAINER" "$APP1_CONTAINER" "$APP2_CONTAINER" "$LIBSQL_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$NETWORK" >/dev/null

docker run -d --name "$LIBSQL_CONTAINER" \
  --network "$NETWORK" \
  --network-alias "$LIBSQL_HOST" \
  -p "127.0.0.1:${LIBSQL_PORT}:8080" \
  ghcr.io/tursodatabase/libsql-server:latest >/dev/null

for attempt in $(seq 1 80); do
  if curl -sS --max-time 1 "http://127.0.0.1:${LIBSQL_PORT}/" >/dev/null 2>&1; then
    break
  fi
  if ! docker inspect -f '{{.State.Running}}' "$LIBSQL_CONTAINER" 2>/dev/null | grep -q true; then
    echo "local libSQL server exited" >&2
    docker logs "$LIBSQL_CONTAINER" >&2 || true
    exit 1
  fi
  if [ "$attempt" -eq 80 ]; then
    echo "local libSQL server did not become reachable" >&2
    docker logs "$LIBSQL_CONTAINER" >&2 || true
    exit 1
  fi
  sleep 0.25
done

atlas_apply() {
  TRENDINARY_ATLAS_DOCKER_NETWORK="$NETWORK" \
  TRENDINARY_ATLAS_LIBSQL_TARGET="${LIBSQL_HOST}:8080" \
    bash scripts/atlas-apply-local-libsql.sh "$LIBSQL_PORT" "$TOKEN"
}

# The migration process runs first. No Trendinary application process is
# allowed to create or alter schema.
atlas_apply

TRENDINARY_TEST_TURSO_URL="ws://127.0.0.1:${LIBSQL_PORT}" \
TRENDINARY_TEST_TURSO_TOKEN="$TOKEN" \
  go test -tags=integration ./internal/history -run 'TestTurso' -count=1

docker build -t "$IMAGE" .

common_args=(
  --network "$NETWORK"
  -e TURSO_DATABASE_URL="ws://${LIBSQL_HOST}:8080"
  -e TURSO_AUTH_TOKEN="$TOKEN"
  -e TRENDINARY_REQUIRE_TURSO=1
  -e TRENDINARY_SCANNER_DISABLED=1
  -e TRENDINARY_JETSTREAM_DISABLED=1
  -e TRENDINARY_GITHUB_DISABLED=1
  -e TRENDINARY_RSS_DISABLED=1
  -e TRENDINARY_WIKIPEDIA_DISABLED=1
  -e TRENDINARY_GDELT_DISABLED=1
  -e TRENDINARY_YOUTUBE_DISABLED=1
  -e TRENDINARY_NEWSDATA_DISABLED=1
)

# Exercise the explicit destructive command. It drops the schema and stops;
# Atlas, not the application, is responsible for recreating it.
docker run --rm \
  --network "$NETWORK" \
  -e TURSO_DATABASE_URL="ws://${LIBSQL_HOST}:8080" \
  -e TURSO_AUTH_TOKEN="$TOKEN" \
  -e TRENDINARY_REQUIRE_TURSO=1 \
  "$IMAGE" db reset

# Prove an ordinary app refuses to start while the database is unmigrated.
docker run -d --name "$FAIL_CONTAINER" "${common_args[@]}" "$IMAGE" >/dev/null
for attempt in $(seq 1 40); do
  if ! docker inspect -f '{{.State.Running}}' "$FAIL_CONTAINER" 2>/dev/null | grep -q true; then
    break
  fi
  sleep 0.1
done
if docker inspect -f '{{.State.Running}}' "$FAIL_CONTAINER" 2>/dev/null | grep -q true; then
  echo "Trendinary started against an unmigrated database" >&2
  docker logs "$FAIL_CONTAINER" >&2 || true
  exit 1
fi
if ! docker logs "$FAIL_CONTAINER" 2>&1 | grep -q "database schema is not migrated"; then
  echo "Trendinary failed against the reset database for an unexpected reason" >&2
  docker logs "$FAIL_CONTAINER" >&2 || true
  exit 1
fi
docker rm "$FAIL_CONTAINER" >/dev/null 2>&1 || true

# Recreate the complete desired schema exactly once, before application start.
atlas_apply

docker run -d --name "$APP1_CONTAINER" -p "127.0.0.1:${APP1_PORT}:8080" "${common_args[@]}" "$IMAGE" >/dev/null
docker run -d --name "$APP2_CONTAINER" -p "127.0.0.1:${APP2_PORT}:8080" "${common_args[@]}" "$IMAGE" >/dev/null

for port in "$APP1_PORT" "$APP2_PORT"; do
  healthy=0
  for attempt in $(seq 1 80); do
    if curl -fsS --max-time 2 "http://127.0.0.1:${port}/api/v1/healthz" >/dev/null; then
      healthy=1
      break
    fi
    sleep 0.25
  done
  if [ "$healthy" -ne 1 ]; then
    echo "Trendinary container on port ${port} did not become healthy against Atlas-prepared shared libSQL" >&2
    docker logs "$APP1_CONTAINER" >&2 || true
    docker logs "$APP2_CONTAINER" >&2 || true
    exit 1
  fi
done

echo "local libSQL gate: db reset -> Atlas apply -> two non-migrating app processes are healthy"
