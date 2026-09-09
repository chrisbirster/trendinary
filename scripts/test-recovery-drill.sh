#!/usr/bin/env bash
set -euo pipefail

SOURCE_CONTAINER="trendinary-recovery-source-$$"
RESTORED_CONTAINER="trendinary-recovery-restored-$$"
APP_CONTAINER="trendinary-recovery-app-$$"
SOURCE_VOLUME="trendinary-recovery-source-vol-$$"
RESTORED_VOLUME="trendinary-recovery-restored-vol-$$"
NETWORK="trendinary-recovery-net-$$"
SOURCE_HOST="source.libsql"
RESTORED_HOST="libsql.test"
SOURCE_PORT="${TRENDINARY_RECOVERY_SOURCE_PORT:-18110}"
RESTORED_PORT="${TRENDINARY_RECOVERY_RESTORED_PORT:-18111}"
APP_PORT="${TRENDINARY_RECOVERY_APP_PORT:-18112}"
LIBSQL_IMAGE="${TRENDINARY_LIBSQL_IMAGE:-ghcr.io/tursodatabase/libsql-server:latest}"
APP_IMAGE="${TRENDINARY_RECOVERY_IMAGE:-trendinary:recovery-drill}"
TOKEN="local-test-token"
RELEASE="v0.0.0-recovery"
COMMIT="recovery-drill"

cleanup() {
  docker rm -f "$APP_CONTAINER" "$RESTORED_CONTAINER" "$SOURCE_CONTAINER" >/dev/null 2>&1 || true
  docker volume rm -f "$RESTORED_VOLUME" "$SOURCE_VOLUME" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_libsql() {
  local container="$1"
  local port="$2"
  local attempt
  for attempt in $(seq 1 100); do
    if curl -sS --max-time 1 "http://127.0.0.1:${port}/" >/dev/null 2>&1; then
      return 0
    fi
    if ! docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null | grep -q true; then
      echo "libSQL container $container exited" >&2
      docker logs "$container" >&2 || true
      return 1
    fi
    sleep 0.25
  done
  echo "libSQL container $container did not become reachable" >&2
  docker logs "$container" >&2 || true
  return 1
}

atlas_apply_to() {
  local target="$1"
  local port="$2"
  TRENDINARY_ATLAS_DOCKER_NETWORK="$NETWORK" \
  TRENDINARY_ATLAS_LIBSQL_TARGET="${target}:8080" \
    bash scripts/atlas-apply-local-libsql.sh "$port" "$TOKEN"
}

docker network create "$NETWORK" >/dev/null
docker volume create "$SOURCE_VOLUME" >/dev/null
docker volume create "$RESTORED_VOLUME" >/dev/null

docker run -d --name "$SOURCE_CONTAINER" \
  --network "$NETWORK" \
  --network-alias "$SOURCE_HOST" \
  -p "127.0.0.1:${SOURCE_PORT}:8080" \
  -v "$SOURCE_VOLUME:/var/lib/sqld" \
  "$LIBSQL_IMAGE" >/dev/null
wait_libsql "$SOURCE_CONTAINER" "$SOURCE_PORT"

atlas_apply_to "$SOURCE_HOST" "$SOURCE_PORT"
TRENDINARY_TEST_TURSO_URL="ws://127.0.0.1:${SOURCE_PORT}" \
TRENDINARY_TEST_TURSO_TOKEN="$TOKEN" \
  go test -tags=integration ./internal/history -run '^TestTursoRuntimeUsesAtlasPreparedSchemaWithoutDDL$' -count=1

docker stop -t 15 "$SOURCE_CONTAINER" >/dev/null
docker rm "$SOURCE_CONTAINER" >/dev/null
docker run --rm \
  -v "$SOURCE_VOLUME:/from:ro" \
  -v "$RESTORED_VOLUME:/to" \
  alpine:3.22 sh -eu -c 'cp -a /from/. /to/'

docker run -d --name "$RESTORED_CONTAINER" \
  --network "$NETWORK" \
  --network-alias "$RESTORED_HOST" \
  -p "127.0.0.1:${RESTORED_PORT}:8080" \
  -v "$RESTORED_VOLUME:/var/lib/sqld" \
  "$LIBSQL_IMAGE" >/dev/null
wait_libsql "$RESTORED_CONTAINER" "$RESTORED_PORT"

atlas_apply_to "$RESTORED_HOST" "$RESTORED_PORT"
TRENDINARY_TEST_TURSO_URL="ws://127.0.0.1:${RESTORED_PORT}" \
TRENDINARY_TEST_TURSO_TOKEN="$TOKEN" \
  go test -tags=integration ./internal/history -run '^TestTursoRecoverySentinelPresent$' -count=1

docker build \
  --build-arg "TRENDINARY_RELEASE=$RELEASE" \
  --build-arg "TRENDINARY_COMMIT_SHA=$COMMIT" \
  -t "$APP_IMAGE" .

docker run -d --name "$APP_CONTAINER" \
  --network "$NETWORK" \
  -p "127.0.0.1:${APP_PORT}:8080" \
  -e TURSO_DATABASE_URL="ws://${RESTORED_HOST}:8080" \
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
  "$APP_IMAGE" >/dev/null

for attempt in $(seq 1 100); do
  if curl -fsS --max-time 2 "http://127.0.0.1:${APP_PORT}/api/v1/readyz" >/dev/null; then
    TRENDINARY_BASE_URL="http://127.0.0.1:${APP_PORT}" \
    TRENDINARY_EXPECTED_RELEASE="$RELEASE" \
    TRENDINARY_EXPECTED_COMMIT="$COMMIT" \
    TRENDINARY_SMOKE_RECOVERY=1 \
    TRENDINARY_SMOKE_RUNTIME_ATTEMPTS=1 \
      bash .github/scripts/production-smoke.sh
    echo "recovery drill: restored libSQL data -> Atlas reconcile -> release boot -> production smoke passed"
    exit 0
  fi
  if ! docker inspect -f '{{.State.Running}}' "$APP_CONTAINER" 2>/dev/null | grep -q true; then
    echo "Trendinary recovery image exited before readiness" >&2
    docker logs "$APP_CONTAINER" >&2 || true
    exit 1
  fi
  sleep 0.25
done

echo "Trendinary recovery image never became ready" >&2
docker logs "$APP_CONTAINER" >&2 || true
exit 1
