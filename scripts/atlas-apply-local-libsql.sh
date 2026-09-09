#!/usr/bin/env bash
set -euo pipefail

PORT="${1:?usage: atlas-apply-local-libsql.sh <port> [token]}"
TOKEN="${2:-local-test-token}"
ATLAS_IMAGE="${TRENDINARY_ATLAS_IMAGE:-arigaio/atlas:latest}"
TARGET="${TRENDINARY_ATLAS_LIBSQL_TARGET:-host.docker.internal:${PORT}}"
NETWORK="${TRENDINARY_ATLAS_DOCKER_NETWORK:-}"

docker_args=(--rm)
if [ -n "$NETWORK" ]; then
  docker_args+=(--network "$NETWORK")
else
  docker_args+=(--add-host host.docker.internal:host-gateway)
fi

# Atlas runs in its own container during local/CI topology tests. Prefer a
# caller-provided Docker network so Atlas talks directly to the libSQL
# container while the host-only published port remains private. The host
# gateway fallback keeps the helper usable outside the topology tests.
docker run \
  "${docker_args[@]}" \
  -v "$PWD:/workspace:ro" \
  -w /workspace \
  "$ATLAS_IMAGE" schema apply \
    --url "libsql+ws://${TARGET}?authToken=${TOKEN}" \
    --to "file://schema/trendinary.sql" \
    --dev-url "sqlite://atlas-dev?mode=memory&_fk=1" \
    --exclude "_litestream*" \
    --auto-approve
