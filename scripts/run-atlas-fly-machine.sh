#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}"
case "$MODE" in
  plan|apply) ;;
  *)
    echo "usage: $0 <plan|apply>" >&2
    exit 2
    ;;
esac

: "${FLY_API_TOKEN:?FLY_API_TOKEN is required}"

APP="${TRENDINARY_FLY_APP:-trendinary}"
REGION="${TRENDINARY_FLY_REGION:-iad}"
TIMEOUT_SECONDS="${TRENDINARY_ATLAS_TIMEOUT_SECONDS:-300}"
NAME="trendinary-atlas-${MODE}-${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-1}-$$"
MACHINE_ID=""
LOG_FILE="$(mktemp -t trendinary-atlas-${MODE}.XXXXXX)"
LOG_PID=""

case "$FLY_API_TOKEN" in
  "FlyV1 "*) FLY_AUTHORIZATION="$FLY_API_TOKEN" ;;
  fm2_*) FLY_AUTHORIZATION="FlyV1 $FLY_API_TOKEN" ;;
  *) FLY_AUTHORIZATION="Bearer $FLY_API_TOKEN" ;;
esac

cleanup() {
  if [ -n "$LOG_PID" ]; then
    kill "$LOG_PID" >/dev/null 2>&1 || true
    wait "$LOG_PID" >/dev/null 2>&1 || true
  fi
  if [ -n "$MACHINE_ID" ]; then
    flyctl machine destroy "$MACHINE_ID" --app "$APP" --force >/dev/null 2>&1 || true
  fi
  rm -f "$LOG_FILE"
}
trap cleanup EXIT

print_captured_logs() {
  if [ -s "$LOG_FILE" ]; then
    cat "$LOG_FILE"
  else
    # Best-effort fallback for very short-lived Machines where streaming logs
    # attached too late to capture process output.
    flyctl logs --app "$APP" --machine "$MACHINE_ID" --no-tail || true
  fi
}

# `fly machine run` waits for a Machine to start, not for its process to finish.
# Keep the job Machine after exit, poll its exit event, then destroy it ourselves
# so the schema gate cannot report success before Atlas has actually completed.
set +e
flyctl machine run . \
  --app "$APP" \
  --region "$REGION" \
  --dockerfile Dockerfile.migrate \
  --restart no \
  --name "$NAME" \
  --env "ATLAS_MODE=$MODE"
RUN_STATUS=$?
set -e

for _ in $(seq 1 30); do
  MACHINE_ID="$(flyctl machines list --app "$APP" --json | jq -r --arg name "$NAME" '.[] | select(.name == $name) | .id' | head -n 1)"
  if [ -n "$MACHINE_ID" ]; then
    break
  fi
  sleep 1
done

if [ -z "$MACHINE_ID" ]; then
  echo "Atlas Fly Machine was not created (fly machine run exit=$RUN_STATUS)" >&2
  if [ "$RUN_STATUS" -eq 0 ]; then
    exit 1
  fi
  exit "$RUN_STATUS"
fi

# Attach log capture while the one-off Machine is still alive so Atlas plan
# output is not lost when the short-lived process exits before a later log read.
flyctl logs --app "$APP" --machine "$MACHINE_ID" >"$LOG_FILE" 2>&1 &
LOG_PID=$!

DEADLINE=$(( $(date +%s) + TIMEOUT_SECONDS ))
while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  STATUS_JSON="$(curl -fsS \
    -H "Authorization: $FLY_AUTHORIZATION" \
    -H 'Content-Type: application/json' \
    "https://api.machines.dev/v1/apps/${APP}/machines/${MACHINE_ID}")"

  EXIT_CODE="$(printf '%s' "$STATUS_JSON" | jq -r '[.events[]? | select(.type == "exit")][0].request.exit_event.exit_code // empty')"
  if [ -n "$EXIT_CODE" ]; then
    sleep 1
    if [ -n "$LOG_PID" ]; then
      kill "$LOG_PID" >/dev/null 2>&1 || true
      wait "$LOG_PID" >/dev/null 2>&1 || true
      LOG_PID=""
    fi

    if [ "$EXIT_CODE" -eq 0 ]; then
      if [ "$MODE" = "plan" ]; then
        echo "Atlas plan output from Fly Machine ${MACHINE_ID}:"
        print_captured_logs
      fi
      echo "Atlas ${MODE} completed successfully in Fly Machine ${MACHINE_ID}."
      exit 0
    fi
    echo "Atlas ${MODE} failed in Fly Machine ${MACHINE_ID} with exit code ${EXIT_CODE}." >&2
    print_captured_logs >&2
    exit "$EXIT_CODE"
  fi

  sleep 2
done

echo "Timed out after ${TIMEOUT_SECONDS}s waiting for Atlas ${MODE} Machine ${MACHINE_ID}." >&2
print_captured_logs >&2
exit 124
