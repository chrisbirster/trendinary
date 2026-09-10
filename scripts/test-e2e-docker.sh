#!/usr/bin/env bash
set -euo pipefail

IMAGE="${TRENDINARY_E2E_IMAGE:-trendinary:e2e}"
APP_CONTAINER="trendinary-e2e-app-$$"
LIBSQL_CONTAINER="trendinary-e2e-libsql-$$"
NETWORK="trendinary-e2e-net-$$"
LIBSQL_HOST="libsql.test"
PORT="${TRENDINARY_E2E_PORT:-18080}"
LIBSQL_PORT="${TRENDINARY_E2E_LIBSQL_PORT:-18084}"
TOKEN="local-test-token"

cleanup() {
  docker rm -f "$APP_CONTAINER" "$LIBSQL_CONTAINER" >/dev/null 2>&1 || true
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

TRENDINARY_ATLAS_DOCKER_NETWORK="$NETWORK" \
TRENDINARY_ATLAS_LIBSQL_TARGET="${LIBSQL_HOST}:8080" \
  bash scripts/atlas-apply-local-libsql.sh "$LIBSQL_PORT" "$TOKEN"

docker build -t "$IMAGE" .
docker run -d --name "$APP_CONTAINER" \
  --network "$NETWORK" \
  -p "127.0.0.1:${PORT}:8080" \
  -e TURSO_DATABASE_URL="ws://${LIBSQL_HOST}:8080" \
  -e TURSO_AUTH_TOKEN="$TOKEN" \
  -e TRENDINARY_REQUIRE_TURSO=1 \
  -e TRENDINARY_SCANNER_DISABLED=1 \
  -e TRENDINARY_JETSTREAM_DISABLED=1 \
  -e TRENDINARY_GITHUB_DISABLED=1 \
  -e TRENDINARY_RSS_DISABLED=1 \
  -e TRENDINARY_WIKIPEDIA_DISABLED=1 \
  -e TRENDINARY_GDELT_DISABLED=1 \
  -e TRENDINARY_YOUTUBE_DISABLED=1 \
  -e TRENDINARY_NEWSDATA_DISABLED=1 \
  "$IMAGE" >/dev/null

for attempt in $(seq 1 60); do
  if curl -fsS --max-time 2 "http://127.0.0.1:${PORT}/api/v1/healthz" >/dev/null; then
    if PLAYWRIGHT_BASE_URL="http://127.0.0.1:${PORT}" npx playwright test --reporter=line; then
      exit 0
    fi
    echo "Playwright production-image E2E failed; Trendinary container logs follow:" >&2
    docker logs "$APP_CONTAINER" >&2 || true
    exit 1
  fi
  if ! docker inspect -f '{{.State.Running}}' "$APP_CONTAINER" 2>/dev/null | grep -q true; then
    echo "Trendinary production image exited before becoming healthy" >&2
    docker logs "$APP_CONTAINER" >&2 || true
    exit 1
  fi
  sleep 0.25
done

echo "Trendinary production image did not become healthy" >&2
docker logs "$APP_CONTAINER" >&2 || true
exit 1
