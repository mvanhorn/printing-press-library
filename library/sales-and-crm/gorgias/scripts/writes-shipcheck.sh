#!/usr/bin/env bash
# writes-shipcheck.sh — dry-run every write endpoint (create/update/delete)
# and assert each produces the expected HTTP method + path.
#
# No live API call. No credentials needed beyond placeholder env vars to
# satisfy config validation. Output is written to writes-shipcheck.json
# at the repo root.
#
# What this proves: every write endpoint's URL routing, method, and ID
# substitution is wired correctly through the cobra surface. It does NOT
# prove that Gorgias accepts the body — that requires a live roundtrip
# (Approach B) or a sandbox tenant (Approach C).
#
# Usage:
#   bash scripts/writes-shipcheck.sh

set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO}"

BINARY="${REPO}/gorgias-pp-cli"
if [ ! -x "${BINARY}" ]; then
  echo "==> building gorgias-pp-cli"
  go build -o gorgias-pp-cli ./cmd/gorgias-pp-cli
fi

# Placeholder env so the client config validates. No call is sent — --dry-run
# short-circuits before the HTTP layer.
export GORGIAS_BASE_URL="https://example.gorgias.com/api"
export GORGIAS_USERNAME="dryrun-username-placeholder"
export GORGIAS_API_KEY="dryrun-key"

# Resource → API path. Most match the CLI subcommand 1:1, but `gorgias-jobs`
# maps to `/jobs` on the server, so we can't derive the path mechanically.
api_path_for() {
  case "$1" in
    "gorgias-jobs") echo "jobs" ;;
    *) echo "$1" ;;
  esac
}

# Per-operation matrix. Each "resource:op" entry is run through `--dry-run`
# and the first stdout line must match `<METHOD> https://.../api/<path>[/<id>]`.
CREATES=(custom-fields customers gorgias-jobs integrations macros rules satisfaction-surveys tags teams tickets users views widgets)
UPDATES=(custom-fields customers gorgias-jobs integrations macros rules satisfaction-surveys tags teams tickets users views widgets)
DELETES=(customers gorgias-jobs integrations macros rules tags teams tickets users views widgets)

PLACEHOLDER_ID=1
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

PROBES_JSON="${TMP}/probes.json"
echo "[" > "${PROBES_JSON}"
FIRST=1
PASS=0; FAIL=0

emit() {
  local op="$1" resource="$2" verdict="$3" detail="$4"
  if [ "${FIRST}" -eq 0 ]; then echo "," >> "${PROBES_JSON}"; fi
  FIRST=0
  printf '  {"op": "%s", "resource": "%s", "verdict": "%s", "detail": %s}' \
    "$op" "$resource" "$verdict" \
    "$(printf '%s' "$detail" | /usr/bin/python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')" \
    >> "${PROBES_JSON}"
  printf '  %-7s %-25s %-7s %s\n' "$op" "$resource" "$verdict" "$detail"
}

probe() {
  local op="$1" resource="$2" method="$3" args="$4" expected_path="$5"
  local out_f="${TMP}/${op}-${resource}.out"
  local expected="${method} ${GORGIAS_BASE_URL}/${expected_path}"

  set +e
  # shellcheck disable=SC2086
  "${BINARY}" "${resource}" "${op}" ${args} --dry-run > "${out_f}" 2>&1
  local rc=$?
  set -e

  if [ "$rc" -ne 0 ]; then
    emit "$op" "$resource" "fail" "exit=$rc; $(head -c 200 "${out_f}" | tr '\n' ' ')"
    FAIL=$((FAIL+1))
    return
  fi

  local first_line
  first_line=$(head -1 "${out_f}")
  if [ "$first_line" = "$expected" ]; then
    emit "$op" "$resource" "pass" "$first_line"
    PASS=$((PASS+1))
  else
    emit "$op" "$resource" "fail" "expected '${expected}', got '${first_line}'"
    FAIL=$((FAIL+1))
  fi
}

echo "==> writes-shipcheck: dry-running 37 write endpoints"
echo "  op      resource                  verdict detail"

# Creates: POST /<path>
for r in "${CREATES[@]}"; do
  path=$(api_path_for "$r")
  probe "create" "$r" "POST" "" "$path"
done

# Updates: PUT /<path>/<id>
for r in "${UPDATES[@]}"; do
  path=$(api_path_for "$r")
  probe "update" "$r" "PUT" "${PLACEHOLDER_ID}" "${path}/${PLACEHOLDER_ID}"
done

# Deletes: DELETE /<path>/<id>
for r in "${DELETES[@]}"; do
  path=$(api_path_for "$r")
  probe "delete" "$r" "DELETE" "${PLACEHOLDER_ID}" "${path}/${PLACEHOLDER_ID}"
done

echo "" >> "${PROBES_JSON}"
echo "]" >> "${PROBES_JSON}"

TOTAL=$((PASS + FAIL))
VERDICT="PASS"
if [ "$FAIL" -gt 0 ]; then VERDICT="WARN"; fi
if [ "$TOTAL" -gt 0 ] && [ "$((FAIL * 2))" -gt "$TOTAL" ]; then VERDICT="FAIL"; fi

BINARY_VERSION=$("${BINARY}" version 2>/dev/null | head -1 || echo "unknown")

/usr/bin/python3 - <<PY
import json
probes = json.load(open("${PROBES_JSON}"))
out = {
  "schema_version": 1,
  "verdict": "${VERDICT}",
  "executed_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "binary_version": "${BINARY_VERSION}",
  "mode": "dry-run",
  "note": "Verifies URL routing, HTTP method, and ID substitution. Does NOT verify that Gorgias accepts the body — that requires a live roundtrip.",
  "totals": {"pass": ${PASS}, "fail": ${FAIL}, "endpoints_probed": ${TOTAL}},
  "probes": probes,
}
json.dump(out, open("${REPO}/writes-shipcheck.json", "w"), indent=2)
print()
print(f"==> verdict: {out['verdict']}  pass={out['totals']['pass']}  fail={out['totals']['fail']}")
print(f"==> artifact: ${REPO}/writes-shipcheck.json")
PY
