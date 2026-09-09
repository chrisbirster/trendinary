#!/usr/bin/env bash
set -euo pipefail

VERIFY=(python3 scripts/verify-atlas-plan.py)
FIXTURES="tests/fixtures/atlas-plan"

"${VERIFY[@]}" "$FIXTURES/additive.sql"

for fixture in destructive-rebuild.sql destructive-rename.sql destructive-constraint.sql; do
  if "${VERIFY[@]}" "$FIXTURES/$fixture"; then
    echo "expected destructive fixture to be blocked: $fixture" >&2
    exit 1
  fi
  "${VERIFY[@]}" --allow-destructive "$FIXTURES/$fixture"
done

# Comments that merely discuss destructive SQL must not trip the policy.
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat >"$tmp" <<'EOF'
-- DROP TABLE signals; is forbidden in ordinary releases.
/* ALTER TABLE signals RENAME COLUMN score TO old_score; */
ALTER TABLE signals ADD COLUMN safe_note TEXT;
EOF
"${VERIFY[@]}" "$tmp"

echo "Atlas plan policy fixtures: additive accepted, destructive plans blocked, explicit maintenance override audited"
