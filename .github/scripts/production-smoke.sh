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

check_runtime_health() {
  local path="/api/v1/health/streams"
  local output="$workdir/runtime-health.json"
  local attempts="${TRENDINARY_SMOKE_RUNTIME_ATTEMPTS:-30}"
  local delay="${TRENDINARY_SMOKE_RUNTIME_DELAY_SECONDS:-5}"
  local attempt
  local filter
  filter="$(cat <<'JQ'
def epoch: sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601;
.data as $d |
($d | type == "object" and has("jetstream") and has("scanner") and has("window")) and
($d.jetstream.enabled != true or $d.jetstream.connected == true) and
($d.scanner.enabled != true or (
  ($d.scanner.last_success_at? | type == "string") and
  (($d.scanner.last_success_at | epoch) >= (now - 900)) and
  ($d.scanner.running != true or (
    ($d.scanner.last_started_at? | type == "string") and
    (($d.scanner.last_started_at | epoch) >= (now - 180))
  ))
))
JQ
)"

  for attempt in $(seq 1 "$attempts"); do
    echo "smoke: GET ${BASE_URL}${path} (runtime attempt ${attempt}/${attempts})"
    if "${CURL[@]}" "${BASE_URL}${path}" >"$output" && jq -e "$filter" "$output" >/dev/null; then
      jq -c . "$output"
      return 0
    fi
    if [ "$attempt" -lt "$attempts" ]; then
      sleep "$delay"
    fi
  done

  echo "::error::Trendinary runtime is degraded: Jetstream must be connected and scanner success must be <15m old (a running scan must have started <3m ago)."
  jq . "$output" || cat "$output"
  return 1
}

check_json "/api/v1/healthz" '.ok == true and .service == "trendinary" and .version == "v1" and .following == true and .web_push == true'
check_runtime_health
check_json "/api/v1/trends" '
  (.data | type == "array") and
  all(.data[];
    ((.slug // "") as $slug |
      ($slug != "at-protocol" and
       $slug != "midnight-sun" and
       $slug != "aster-1" and
       $slug != "that-blue-chair" and
       $slug != "orbit-cup" and
       $slug != "quiet-quitting-2")) and
    ((.timeline // []) | length) >= 2 and
    ((.aliases // []) | length) <= 12 and
    (([.sources[]?.domain] | unique) as $domains |
      (($domains | length) != 1 or $domains[0] != "wikipedia.org"))
  )
'
check_json "/api/v1/following/push/public-key" '.data.public_key | type == "string" and length > 40'

echo "smoke: production API healthy; any published leaderboard entries satisfy quality invariants"
