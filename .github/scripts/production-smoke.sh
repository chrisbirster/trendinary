#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${TRENDINARY_BASE_URL:-https://trendinary.com}"
CURL=(curl --fail --show-error --silent --retry 8 --retry-all-errors --retry-delay 5 --connect-timeout 10 --max-time 30)

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

check_json() {
  local path="$1"
  local filter="$2"
  local output="$workdir/$(printf '%s' "$path" | tr '/?' '__').json"

  echo "smoke: GET ${BASE_URL}${path}"
  "${CURL[@]}" "${BASE_URL}${path}" >"$output"
  if ! jq -e "$filter" "$output" >/dev/null; then
    echo "::error::Unexpected response from ${BASE_URL}${path}"
    jq . "$output" || cat "$output"
    return 1
  fi
  jq -c . "$output"
}

check_json "/api/v1/healthz" '.ok == true and .service == "trendinary" and .version == "v1" and .following == true and .web_push == true'
check_json "/api/v1/health/streams" '.data | type == "object" and has("jetstream") and has("scanner") and has("window")'
check_json "/api/v1/trends" '.data | type == "array"'
check_json "/api/v1/following/push/public-key" '.data.public_key | type == "string" and length > 40'

echo "smoke: production API healthy"
