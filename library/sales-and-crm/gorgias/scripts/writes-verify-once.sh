#!/usr/bin/env bash
# writes-verify-once.sh — ONE-SHOT live verification of write endpoints
# whose Gorgias API has no DELETE (so live-roundtrip would leave debris).
# Run once against a live tenant to confirm create/update work end-to-end;
# do not put in recurring CI.
#
# Endpoints covered:
#   customers      create + update    (the API has DELETE, but a deleted
#                                      customer loses ticket history —
#                                      we treat it as effectively undeletable)
#   custom-fields  create + update    (no DELETE in API)
#
# DEBRIS LEFT BEHIND: 2 rows (1 customer + 1 custom-field) per successful run.
# Each is named with a `verify-once-<timestamp>-<pid>` marker so you can find
# them in the admin UI and retire them on a quarterly basis.
#
# Tenant-agnostic. Expects GORGIAS_USERNAME, GORGIAS_API_KEY, GORGIAS_BASE_URL
# in the environment.
#
# Output: writes-verify-once.json at the repo root.

set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO}"

if [ -z "${GORGIAS_USERNAME:-}" ] || [ -z "${GORGIAS_API_KEY:-}" ] || [ -z "${GORGIAS_BASE_URL:-}" ]; then
  echo "error: GORGIAS_USERNAME, GORGIAS_API_KEY, GORGIAS_BASE_URL must be set" >&2
  exit 64
fi

BINARY="${REPO}/gorgias-pp-cli"
if [ ! -x "${BINARY}" ]; then
  echo "==> building gorgias-pp-cli"
  go build -o gorgias-pp-cli ./cmd/gorgias-pp-cli
fi

if [ -f "${REPO}/writes-verify-once.json" ] && [ "${1:-}" != "--force" ]; then
  echo "error: writes-verify-once.json already exists. This is a one-shot script."
  echo "       If you really want to run it again (will leave another 2 debris rows),"
  echo "       pass --force as the first argument."
  exit 65
fi

MARKER="verify-once-$(date +%Y%m%d-%H%M%S)-$$"
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

cat <<MSG
==> writes-verify-once: 4 endpoints (2 resources × create/update)
==> marker: ${MARKER}

WARNING — this run will leave 2 admin-UI rows on the live tenant:
  - 1 customer record (search by email matching the marker)
  - 1 custom-field record (search by label matching the marker)

Press Ctrl-C in the next 5 seconds to abort.
MSG
sleep 5

PROBES_JSON="${TMP}/probes.json"
echo "[" > "${PROBES_JSON}"
FIRST=1
PASS=0; FAIL=0

emit() {
  local resource="$1" op="$2" verdict="$3" detail="$4"
  if [ "${FIRST}" -eq 0 ]; then echo "," >> "${PROBES_JSON}"; fi
  FIRST=0
  printf '  {"resource": "%s", "op": "%s", "verdict": "%s", "detail": %s}' \
    "$resource" "$op" "$verdict" \
    "$(printf '%s' "$detail" | /usr/bin/python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')" \
    >> "${PROBES_JSON}"
  printf '  %-15s %-7s %-7s %s\n' "$resource" "$op" "$verdict" "$detail"
}

extract_id() {
  /usr/bin/python3 -c "
import json, sys
try:
    d = json.load(open(sys.argv[1]))
    for p in (('data','id'), ('results','id'), ('id',)):
        cur = d; ok = True
        for k in p:
            if isinstance(cur, dict) and k in cur:
                cur = cur[k]
            else:
                ok = False; break
        if ok and cur is not None:
            print(cur); break
except Exception:
    pass
" "$1"
}

# ---- customers --------------------------------------------------------
CUSTOMER_BODY=$(cat <<JSON
{
  "channels": [
    {"type": "email", "address": "${MARKER}@example.com"}
  ],
  "firstname": "Verify",
  "lastname": "Once ${MARKER}"
}
JSON
)
CUST_OUT="${TMP}/customers-create.json"
set +e
echo "${CUSTOMER_BODY}" | "${BINARY}" customers create --stdin --agent > "${CUST_OUT}" 2>&1
RC=$?
set -e
CUST_ID=""
if [ "$RC" -eq 0 ]; then
  CUST_ID=$(extract_id "${CUST_OUT}")
fi
if [ -n "$CUST_ID" ]; then
  emit "customers" "create" "pass" "id=${CUST_ID} (debris)"
  PASS=$((PASS+1))

  # update — Gorgias's customers update body silently accepts unknown
  # fields like firstname/lastname but treats the body as "empty" if no
  # documented field is present. The documented partial-update field
  # surface is: email, external_id, language, name, timezone, channels.
  set +e
  echo "{\"name\":\"Verify Once-renamed ${MARKER}\"}" | \
    "${BINARY}" customers update "${CUST_ID}" --stdin --agent > "${TMP}/customers-update.json" 2>&1
  URC=$?
  set -e
  if [ "$URC" -eq 0 ]; then
    emit "customers" "update" "pass" "id=${CUST_ID} updated"
    PASS=$((PASS+1))
  else
    emit "customers" "update" "fail" "exit=${URC}; $(head -c 200 "${TMP}/customers-update.json" | tr '\n' ' ')"
    FAIL=$((FAIL+1))
  fi
else
  emit "customers" "create" "fail" "exit=$RC; $(head -c 200 "${CUST_OUT}" | tr '\n' ' ')"
  FAIL=$((FAIL+1))
  emit "customers" "update" "skip" "create failed"
fi

# ---- custom-fields ----------------------------------------------------
CF_BODY=$(cat <<JSON
{
  "object_type": "Ticket",
  "label": "${MARKER}",
  "definition": {
    "data_type": "text",
    "input_settings": {"input_type": "input"}
  }
}
JSON
)
CF_OUT="${TMP}/custom-fields-create.json"
set +e
echo "${CF_BODY}" | "${BINARY}" custom-fields create --stdin --agent > "${CF_OUT}" 2>&1
RC=$?
set -e
CF_ID=""
if [ "$RC" -eq 0 ]; then
  CF_ID=$(extract_id "${CF_OUT}")
fi
if [ -n "$CF_ID" ]; then
  emit "custom-fields" "create" "pass" "id=${CF_ID} (debris)"
  PASS=$((PASS+1))

  set +e
  echo "{\"label\":\"${MARKER}-renamed\"}" | \
    "${BINARY}" custom-fields update "${CF_ID}" --stdin --agent > "${TMP}/custom-fields-update.json" 2>&1
  URC=$?
  set -e
  if [ "$URC" -eq 0 ]; then
    emit "custom-fields" "update" "pass" "id=${CF_ID} updated"
    PASS=$((PASS+1))
  else
    emit "custom-fields" "update" "fail" "exit=${URC}; $(head -c 200 "${TMP}/custom-fields-update.json" | tr '\n' ' ')"
    FAIL=$((FAIL+1))
  fi
else
  emit "custom-fields" "create" "fail" "exit=$RC; $(head -c 200 "${CF_OUT}" | tr '\n' ' ')"
  FAIL=$((FAIL+1))
  emit "custom-fields" "update" "skip" "create failed"
fi

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
  "mode": "live one-shot, leaves debris",
  "marker": "${MARKER}",
  "note": "Endpoints here have no DELETE (or DELETE is destructive). Search the admin UI for the marker to find and manually retire the debris rows. Do NOT add these to recurring CI.",
  "totals": {"pass": ${PASS}, "fail": ${FAIL}, "endpoints_probed": ${TOTAL}},
  "debris_left_behind": [
    {"resource": "customers",     "id": "${CUST_ID}", "find_by": "email contains '${MARKER}'"},
    {"resource": "custom-fields", "id": "${CF_ID}",   "find_by": "label = '${MARKER}-renamed' (or '${MARKER}' if update failed)"}
  ],
  "probes": probes,
}
json.dump(out, open("${REPO}/writes-verify-once.json", "w"), indent=2)
print()
print(f"==> verdict: {out['verdict']}  pass={out['totals']['pass']}  fail={out['totals']['fail']}")
print(f"==> artifact: ${REPO}/writes-verify-once.json")
print()
print("DEBRIS LEFT (manually retire when convenient):")
for d in out["debris_left_behind"]:
    print(f"   {d['resource']:15} id={d['id']:>10}  find_by={d['find_by']}")
PY
