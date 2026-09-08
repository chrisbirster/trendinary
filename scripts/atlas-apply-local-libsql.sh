#!/usr/bin/env bash
set -euo pipefail

PORT="${1:?usage: atlas-apply-local-libsql.sh <port> [token]}"
TOKEN="${2:-local-test-token}"
ATLAS_IMAGE="${TRENDINARY_ATLAS_IMAGE:-arigaio/atlas:latest}"

# Atlas runs in its own container during local/CI topology tests. The target
# libSQL server is published on the Docker host, so use host.docker.internal on
# both Docker Desktop and Linux (host-gateway supplies the latter mapping).
docker run --rm \
  --add-host host.docker.internal:host-gateway \
  -v "$PWD:/workspace:ro" \
  -w /workspace \
  "$ATLAS_IMAGE" schema apply \
    --url "libsql+ws://host.docker.internal:${PORT}?authToken=${TOKEN}" \
    --to "file://schema/trendinary.sql" \
    --dev-url "sqlite://atlas-dev?mode=memory&_fk=1" \
    --exclude "_litestream*" \
    --auto-approve
