#!/usr/bin/env bash
set -euo pipefail

IMAGE="${TRENDINARY_E2E_IMAGE:-trendinary:e2e}"
CONTAINER="trendinary-e2e-${$}"
PORT="${TRENDINARY_E2E_PORT:-18080}"

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker build -t "$IMAGE" .
docker run -d --name "$CONTAINER" \
  -p "127.0.0.1:${PORT}:8080" \
  -e TRENDINARY_DB_PATH=/tmp/trendinary.db \
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
    PLAYWRIGHT_BASE_URL="http://127.0.0.1:${PORT}" npx playwright test
    exit 0
  fi
  if ! docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -q true; then
    echo "Trendinary production image exited before becoming healthy" >&2
    docker logs "$CONTAINER" >&2 || true
    exit 1
  fi
  sleep 0.25
done

echo "Trendinary production image did not become healthy" >&2
docker logs "$CONTAINER" >&2 || true
exit 1
