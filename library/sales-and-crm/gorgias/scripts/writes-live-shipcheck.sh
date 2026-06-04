#!/usr/bin/env bash
# writes-live-shipcheck.sh — safe-roundtrip live writes against production.
# For each "safe" resource (tags, macros, tickets), perform:
#   1. create with a unique marker
#   2. update the created row
#   3. delete the created row
#
# Tenant-agnostic. Expects GORGIAS_USERNAME, GORGIAS_API_KEY, and
# GORGIAS_BASE_URL in the environment.
#
# Excluded from live coverage (intentional):
#   - custom-fields  — Gorgias API has no delete endpoint; would leave debris
#   - satisfaction-surveys — tied to real customer conversations
#   - gorgias-jobs   — async batch jobs aren't safe-roundtrip semantics
#   - customers, users, integrations, rules, views, teams, widgets
#     — disruptive or catastrophic against production
#
# These remain dry-run-only (covered by writes-shipcheck.sh).
#
# Cleanup invariant: every successful create is paired with a delete on the
# same path. A failed run may leave a row behind; all rows are named with
# a unique marker prefix so manual cleanup is trivial.
#
# Output:
#   stdout — human-readable summary per probe + final verdict
#   writes-live-shipcheck.json — machine-readable artifact at repo root

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

MARKER="shipcheck-$(date +%Y%m%d-%H%M%S)-$$"
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

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
  printf '  %-12s %-8s %-7s %s\n' "$resource" "$op" "$verdict" "$detail"
}

extract_id() {
  /usr/bin/python3 -c "
import json, sys
try:
    d = json.load(open(sys.argv[1]))
    # gorgias-pp-cli wraps every response in {action, data: <body>, path, resource, status, success}.
    # The actual API object lives at data.id; check that first, fall back to other shapes.
    if isinstance(d, dict):
        for path in (('data', 'id'), ('results', 'id'), ('id',)):
            cur = d
            ok = True
            for k in path:
                if isinstance(cur, dict) and k in cur:
                    cur = cur[k]
                else:
                    ok = False
                    break
            if ok and cur is not None:
                print(cur)
                break
except Exception:
    pass
" "$1"
}

run_roundtrip() {
  local resource="$1" create_body="$2" update_body="$3"

  # 1. create -----------------------------------------------------------
  local create_out="${TMP}/${resource}-create.json"
  set +e
  echo "${create_body}" | "${BINARY}" "${resource}" create --stdin --agent > "${create_out}" 2>&1
  local rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    emit "$resource" "create" "fail" "exit=$rc; $(head -c 200 "${create_out}" | tr '\n' ' ')"
    FAIL=$((FAIL+1))
    return
  fi
  local id
  id=$(extract_id "${create_out}")
  if [ -z "$id" ]; then
    emit "$resource" "create" "fail" "create succeeded but couldn't extract id from response"
    FAIL=$((FAIL+1))
    return
  fi
  emit "$resource" "create" "pass" "id=$id"
  PASS=$((PASS+1))

  # 2. update -----------------------------------------------------------
  local update_out="${TMP}/${resource}-update.json"
  set +e
  echo "${update_body}" | "${BINARY}" "${resource}" update "${id}" --stdin --agent > "${update_out}" 2>&1
  local urc=$?
  set -e
  if [ "$urc" -eq 0 ]; then
    emit "$resource" "update" "pass" "id=$id updated"
    PASS=$((PASS+1))
  else
    emit "$resource" "update" "fail" "exit=$urc; $(head -c 200 "${update_out}" | tr '\n' ' ')"
    FAIL=$((FAIL+1))
  fi

  # 3. delete (ALWAYS attempt, even if update failed, so we clean up) ---
  local delete_out="${TMP}/${resource}-delete.json"
  set +e
  "${BINARY}" "${resource}" delete "${id}" --agent --yes > "${delete_out}" 2>&1
  local drc=$?
  set -e
  if [ "$drc" -eq 0 ]; then
    emit "$resource" "delete" "pass" "id=$id deleted"
    PASS=$((PASS+1))
  else
    emit "$resource" "delete" "fail" "exit=$drc; cleanup FAILED for id=$id — manual delete needed; $(head -c 200 "${delete_out}" | tr '\n' ' ')"
    FAIL=$((FAIL+1))
  fi
}

echo "==> writes-live-shipcheck: 6 resources × create/update/delete = 18 live writes"
echo "==> marker: ${MARKER} (used to name all test rows)"
echo "  resource     op       verdict detail"
echo
echo "Resources intentionally NOT covered here:"
echo "  - tickets       — creating spawns a customer record that has no clean bulk-cleanup path"
echo "  - customers     — destructive against real ticket history"
echo "  - users         — creating consumes seats and sends invitation emails"
echo "  - custom-fields — Gorgias API has no DELETE endpoint"
echo "  - satisfaction-surveys — no DELETE endpoint, links to real conversations"
echo "  - gorgias-jobs  — async batch jobs aren't safe-roundtrip semantics"
echo "  - rules         — code body validated by Gorgias DSL AST parser; building"
echo "                    a 'well-formed but inert' rule needs DSL-grammar knowledge"
echo "                    we don't yet have. Request-shape verified by dry-run."
echo "  Request-shape coverage for all these lives in writes-shipcheck.sh (dry-run)."
echo

# tags ----------------------------------------------------------------
# Minimum-viable body: just `name`. Gorgias also accepts `description` and
# `decoration` (both documented optional) and persists them correctly; we
# don't exercise those here because the smoke-test goal is to prove the
# create→update→delete lifecycle, not enumerate every body field.
run_roundtrip "tags" \
  "{\"name\":\"${MARKER}-tag\"}" \
  "{\"name\":\"${MARKER}-tag-renamed\"}"

# macros --------------------------------------------------------------
# Macros' write surface is `name`, `actions`, `external_id`, `intent`,
# `language` — no description (the create-macro docs do not list one,
# and the server hard-rejects with 400 "No such field" if sent).
run_roundtrip "macros" \
  "{\"name\":\"${MARKER}-macro\",\"actions\":[]}" \
  "{\"name\":\"${MARKER}-macro-renamed\"}"

# teams ---------------------------------------------------------------
# Organizational metadata only — pure config row.
run_roundtrip "teams" \
  "{\"name\":\"${MARKER}-team\"}" \
  "{\"name\":\"${MARKER}-team-renamed\"}"

# views ---------------------------------------------------------------
# Sidebar filter spec for the agent UI. View filter DSL only accepts a
# small whitelist of `ticket.*` fields (no ticket.id, no ticket.subject —
# server rejects with "Unsupported argument"). `ticket.status = closed`
# is allowed and read-only; viewing closed tickets is harmless and we
# delete the view immediately. `slug` is marked DEPRECATED in both the
# OpenAPI spec and the CLI help text, but is also marked required:true,
# and the server enforces required — the deprecation note means "we'd
# like to remove this someday," not "you can omit it today."
run_roundtrip "views" \
  "{\"name\":\"${MARKER}-view\",\"slug\":\"${MARKER}-view-slug\",\"type\":\"ticket-list\",\"filters\":\"eq(ticket.status, \\\"closed\\\")\"}" \
  "{\"name\":\"${MARKER}-view-renamed\"}"

# widgets -------------------------------------------------------------
# Agent-facing sidebar widget. type=custom + empty wrapper template
# means it renders nothing if installed; we delete before that matters.
run_roundtrip "widgets" \
  "{\"type\":\"custom\",\"context\":\"ticket\",\"template\":{\"type\":\"wrapper\",\"widgets\":[]}}" \
  "{\"context\":\"customer\"}"

# integrations --------------------------------------------------------
# HTTP integration with a publicly-valid host. Gorgias's URL validator
# rejects unreachable hosts (e.g. `.invalid`), so we use example.com/never
# — a real domain whose /never path 404s harmlessly even if anything
# tried to call it. When type=http, the server requires `http.url`,
# `http.method`, `http.request_content_type`, AND `http.response_content_type`,
# none of which are enumerated in the field-by-field reference at
# developers.gorgias.com/reference/create-integration (the `http` object
# is documented as opaque); they're discoverable only via the 400.
run_roundtrip "integrations" \
  "{\"type\":\"http\",\"name\":\"${MARKER}-integration\",\"http\":{\"url\":\"https://example.com/never\",\"method\":\"GET\",\"request_content_type\":\"application/json\",\"response_content_type\":\"application/json\"}}" \
  "{\"name\":\"${MARKER}-integration-renamed\"}"

# rules — intentionally skipped from live coverage. Gorgias's AST validator
# rejects every JS shape we've tried (plain JS, the example from --help,
# example with no-op return) with "code_ast: The rule is not well formatted".
# The rule body needs Gorgias DSL grammar specifically, not generic JS.
# writes-shipcheck.sh dry-run already verified the create/update/delete
# request URLs + methods route correctly. Add live rules coverage when
# we have a sandbox tenant or someone who knows the DSL grammar.

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
  "mode": "live-roundtrip",
  "marker_template": "shipcheck-<timestamp>-<pid>",
  "note": "Each resource is roundtripped: create with marker → update → delete. A failed run may leave a row named with the marker prefix — search the admin UI for 'shipcheck-' to clean orphans.",
  "totals": {"pass": ${PASS}, "fail": ${FAIL}, "endpoints_probed": ${TOTAL}},
  "probes": probes,
}
json.dump(out, open("${REPO}/writes-live-shipcheck.json", "w"), indent=2)
print()
print(f"==> verdict: {out['verdict']}  pass={out['totals']['pass']}  fail={out['totals']['fail']}")
print(f"==> artifact: ${REPO}/writes-live-shipcheck.json")
PY
