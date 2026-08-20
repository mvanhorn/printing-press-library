#!/usr/bin/env bash
# shipcheck.sh — exercise every read endpoint against the live tenant and
# record per-endpoint pass/fail to shipcheck-results.json.
#
# Tenant-agnostic. Expects GORGIAS_USERNAME, GORGIAS_API_KEY, and
# GORGIAS_BASE_URL to already be in the environment — how they got there
# (shell profile, secrets manager, CI secret store) is not this script's
# concern. Run it however your workflow injects those vars.
#
# Pure read. Never writes. Idempotent. Safe to run against production.
#
# Usage:
#   GORGIAS_USERNAME=... GORGIAS_API_KEY=... GORGIAS_BASE_URL=... \
#     bash scripts/shipcheck.sh
#
# Output:
#   stdout — human-readable summary line per endpoint + final verdict
#   shipcheck-results.json — machine-readable artifact at repo root

set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO}"

if [ -z "${GORGIAS_USERNAME:-}" ] || [ -z "${GORGIAS_API_KEY:-}" ] || [ -z "${GORGIAS_BASE_URL:-}" ]; then
  echo "error: GORGIAS_USERNAME, GORGIAS_API_KEY, GORGIAS_BASE_URL must be set in the environment" >&2
  exit 64
fi

BINARY="${REPO}/gorgias-pp-cli"
if [ ! -x "${BINARY}" ]; then
  echo "==> building gorgias-pp-cli"
  go build -o gorgias-pp-cli ./cmd/gorgias-pp-cli
fi

OUT="${REPO}/shipcheck-results.json"
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

# Probe each LIST endpoint with --limit 1, capture exit + first-row id.
# Resource families derived from internal/cli/*_list.go.
RESOURCES=(
  custom-fields
  customers
  events
  gorgias-jobs
  integrations
  macros
  rules
  satisfaction-surveys
  tags
  teams
  tickets
  users
  views
  widgets
)

# Endpoints that need a required filter flag. Provide a sensible default per
# resource so we cover them; values are appended after `list`.
extra_flags_for() {
  case "$1" in
    "custom-fields") echo "--object-type=Ticket" ;;
    *) echo "" ;;
  esac
}

# `events` requires --object-type AND --object-id (a specific resource id),
# which we can't synthesize generically. Skip and report `not_probed`.
SKIP_LIST=(events)

# A few endpoints have no list path (e.g. account, phone subcommands); we
# probe them via direct get calls below.
EXTRA_GETS=(
  "account get"
)

PASS=0; FAIL=0; SKIP=0
RESULTS_JSON="${TMP}/results.json"
echo "[" > "${RESULTS_JSON}"
FIRST=1

emit_result() {
  local kind="$1" endpoint="$2" verdict="$3" detail="$4"
  if [ "${FIRST}" -eq 0 ]; then echo "," >> "${RESULTS_JSON}"; fi
  FIRST=0
  printf '  {"kind": "%s", "endpoint": "%s", "verdict": "%s", "detail": %s}' \
    "${kind}" "${endpoint}" "${verdict}" \
    "$(printf '%s' "${detail}" | /usr/bin/python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')" \
    >> "${RESULTS_JSON}"
  printf '  %-8s %-30s %-7s %s\n' "${kind}" "${endpoint}" "${verdict}" "${detail}"
}

is_skipped() {
  local needle="$1"
  for s in "${SKIP_LIST[@]}"; do
    if [ "$s" = "$needle" ]; then return 0; fi
  done
  return 1
}

echo "==> shipcheck: probing $(( ${#RESOURCES[@]} )) LIST endpoints + $(( ${#RESOURCES[@]} + ${#EXTRA_GETS[@]} )) GET-by-id endpoints"
echo "  kind     endpoint                       verdict detail"

for r in "${RESOURCES[@]}"; do
  if is_skipped "$r"; then
    emit_result "list" "${r} list" "skip" "requires endpoint-specific params"
    SKIP=$((SKIP + 1))
    continue
  fi

  LIST_OUT="${TMP}/${r}-list.json"
  EXTRA=$(extra_flags_for "${r}")
  set +e
  # shellcheck disable=SC2086
  "${BINARY}" "${r}" list --limit 1 --agent ${EXTRA} > "${LIST_OUT}" 2>&1
  RC=$?
  set -e

  if [ "${RC}" -eq 0 ]; then
    # Extract first row's id (envelope-aware: results.data[0].id or results[0].id)
    ID=$(/usr/bin/python3 -c "
import json, sys
try:
    d = json.load(open('${LIST_OUT}'))
    items = d.get('results', d)
    if isinstance(items, dict):
        items = items.get('data', [])
    if items:
        print(items[0].get('id', ''))
except Exception:
    pass
")
    if [ -n "${ID}" ]; then
      emit_result "list" "${r} list" "pass" "first id captured"
      PASS=$((PASS + 1))

      # Try GET-by-id with that id
      GET_OUT="${TMP}/${r}-get.json"
      set +e
      "${BINARY}" "${r}" get "${ID}" --agent > "${GET_OUT}" 2>&1
      GET_RC=$?
      set -e
      if [ "${GET_RC}" -eq 0 ]; then
        emit_result "get" "${r} get" "pass" "round-trip ok"
        PASS=$((PASS + 1))
      else
        emit_result "get" "${r} get" "fail" "exit=${GET_RC}; $(head -c 200 "${GET_OUT}" | tr '\n' ' ')"
        FAIL=$((FAIL + 1))
      fi
    else
      emit_result "list" "${r} list" "pass" "empty list (no id to GET)"
      PASS=$((PASS + 1))
      emit_result "get" "${r} get" "skip" "list returned empty"
      SKIP=$((SKIP + 1))
    fi
  else
    emit_result "list" "${r} list" "fail" "exit=${RC}; $(head -c 200 "${LIST_OUT}" | tr '\n' ' ')"
    FAIL=$((FAIL + 1))
  fi
done

# Local-mirror commands (no live API call). Safe regardless of credentials.
# Each must return a clean exit and a parseable JSON envelope.
LOCAL_CMDS=(
  "analytics"
  "orphans"
  "stale"
  "load"
  "workflow status"
)

for cmd in "${LOCAL_CMDS[@]}"; do
  OUT_F="${TMP}/local-$(echo "$cmd" | tr ' /' '__').json"
  set +e
  # shellcheck disable=SC2086
  "${BINARY}" ${cmd} --agent > "${OUT_F}" 2>&1
  RC=$?
  set -e
  if [ "${RC}" -eq 0 ] && /usr/bin/python3 -c "import json,sys;json.load(open('${OUT_F}'))" 2>/dev/null; then
    emit_result "local" "${cmd}" "pass" "json envelope ok"
    PASS=$((PASS + 1))
  else
    emit_result "local" "${cmd}" "fail" "exit=${RC}; $(head -c 200 "${OUT_F}" | tr '\n' ' ')"
    FAIL=$((FAIL + 1))
  fi
done

# Extra GET-only endpoints (no list path).
for cmd in "${EXTRA_GETS[@]}"; do
  OUT_F="${TMP}/extra-$(echo "$cmd" | tr ' /' '__').json"
  set +e
  # shellcheck disable=SC2086
  "${BINARY}" ${cmd} --agent > "${OUT_F}" 2>&1
  RC=$?
  set -e
  if [ "${RC}" -eq 0 ]; then
    emit_result "get" "${cmd}" "pass" "ok"
    PASS=$((PASS + 1))
  else
    emit_result "get" "${cmd}" "fail" "exit=${RC}; $(head -c 200 "${OUT_F}" | tr '\n' ' ')"
    FAIL=$((FAIL + 1))
  fi
done

echo "" >> "${RESULTS_JSON}"
echo "]" >> "${RESULTS_JSON}"

# Verdict: PASS if every non-skipped endpoint passed, WARN if any failed,
# FAIL if more than half failed (config mismatch, not endpoint flakiness).
TOTAL=$((PASS + FAIL))
VERDICT="PASS"
if [ "${FAIL}" -gt 0 ]; then
  VERDICT="WARN"
fi
if [ "${TOTAL}" -gt 0 ] && [ "$((FAIL * 2))" -gt "${TOTAL}" ]; then
  VERDICT="FAIL"
fi

BINARY_VERSION=$("${BINARY}" version 2>/dev/null | head -1 || echo "unknown")
# Redact the tenant identity from the committed artifact; the API surface
# is the same for every Gorgias tenant, so the specific host adds no signal
# and is tenant PII when published.
TENANT_HOST="<tenant>.gorgias.com"

/usr/bin/python3 - <<PY
import json
results = json.load(open("${RESULTS_JSON}"))
out = {
  "schema_version": 1,
  "verdict": "${VERDICT}",
  "executed_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "tenant_host": "${TENANT_HOST}",
  "binary_version": "${BINARY_VERSION}",
  "totals": {
    "pass": ${PASS},
    "fail": ${FAIL},
    "skip": ${SKIP},
    "endpoints_probed": ${TOTAL},
  },
  "results": results,
}
json.dump(out, open("${OUT}", "w"), indent=2)
print()
print(f"==> verdict: {out['verdict']}  pass={out['totals']['pass']}  fail={out['totals']['fail']}  skip={out['totals']['skip']}")
print(f"==> artifact: ${OUT}")
PY
