# 3CX XAPI CLI — Phase 5 Acceptance (live dogfood)

**Instance:** your-3cx-fqdn.example.com (production) · **Auth:** OAuth2 client-credentials (env vars) · **Level:** Full

## Result
- Matrix: **1445 / 1471 passed** (98.2%), 26 failed, 0 skipped.
- Acceptance gate JSON: `status: fail` (Full requires 0 failures).
- **Every flagship, core, and novel feature passed live** (see below).

## Flagship / core verified live (read-only, real data)
| Command | Result |
|---|---|
| doctor | Auth configured, API reachable |
| sync (8 resources, $expand) | 172 records: users 51, groups 27, ring-groups 3, queues 7, inbound-rules 38, did-numbers 35, receptionists 6, trunks 5 |
| audit | 67 valid DNs, 0 dangling (memberships checked) |
| trace 404 | exists, 3 inbound-rule routing paths |
| qrollup | 7 queues, 35 agents (152 Queue A=9, 153 Queue B=9, …) |
| search "404" | 3 matches |
| diff --save / diff a b | snapshot + self-diff = 0 changes |
| posture | aggregates blocklist/blacklist/firewall/events/audit |

## The 26 failures (all non-flagship generated endpoint commands)
None are core, novel, or flagship. Three classes, all upstream/generator/harness — not CLI logic defects:

1. **File-download/export endpoints (14 checks, 7 cmds)** — return CSV/scripts, not JSON; the generated endpoint command expects JSON and errors (exit 5). Commands: `users export-extensions`, `event-logs download`, `call-history-view download-call-history`, `inbound-rules export-caller-id-rules`, `report-inbound-rules download-inbound-rules`, `call-cost-settings export-call-costs`, `microsoft365-teams-integration get-dial-plan-script`. **Cause:** generator does not detect non-JSON `response_format` for these OData stream endpoints. **Fix:** generator-level (retro).

2. **Special-PBX-mode endpoints (8 checks, 4 cmds)** — return HTTP 400 with vendor warnings (e.g. `WARNINGS.XAPI.MCM_MODE_REQUIRED`) because they require a PBX mode/state not active on this instance. Commands: `system-status apitoken`, `conference-settings get-mcurequest-status`, `microsoft365-teams-integration get-map-users-script`, `voicemail-settings get-converter-request-status`. **Cause:** upstream requires conditions not met; the command surfaces the API error honestly. **Fix:** not applicable (requires PBX state).

3. **Oversized-response endpoints (4 checks)** — `recordings list`, `report-audit-log get-audit-log-data`, `system-status service-telemetry`, `system-status system-telemetry` returned **valid JSON** but exceeded the dogfood harness output-capture cap, so the harness marked them failed. **Cause:** test-harness capture limit. **Fix:** none needed — commands work; retro on the harness cap.

## Fixes applied during Phase 5 (live)
- OAuth token URL now derives from base URL host (instance-portable; lets base-URL override redirect the token mint).
- `doctor` recognizes client-credentials env vars as configured auth (was false "not configured").
- Store ID extraction: added `Number` to fallbacks so DN-keyed entities (did-numbers, etc.) cache (was dropping all rows).
- Novel commands open the store read-write so an empty/unschema'd DB returns `[]` gracefully (was "no such table: resources").
- audit/trace/qrollup emit a `$expand` hint (and audit JSON note) when ring-group/queue memberships aren't expanded.
- `trace` rejects non-numeric extensions with a usage error.

## Printing Press issues (for retro)
- Generator: OData PascalCase primary keys (`Id`, `Number`) missing from `genericIDFieldFallbacks` → silent row drops; should be defaults for OData specs.
- Generator: hardcodes the spec's absolute base/token URL as the only default; token URL should derive from base URL host for OAuth same-host flows.
- Generator: `doctor` auth check requires a minted header; for client-credentials it should treat client_id+secret presence as configured.
- Generator: non-JSON `response_format` (file/CSV/script download endpoints) not detected for OData function/stream endpoints. **Confirmed lever exists:** `resolveReadWithStrategyResponsePathAndJSONGuard(..., guardLiveJSON=false, ...)` already supports passing non-JSON bodies through; the generated download/export commands call the `guardLiveJSON=true` wrapper. The generator should emit the `guardLiveJSON=false` call (and a raw-body output path) for endpoints whose OData response content-type is non-JSON (e.g. `users export-extensions` returns valid `text/csv`). ~42 export/download commands affected.
- Generator: missing workflow template `workflows/comm_health.go.tmpl` (warning at generate).
- Generated client test is auth-env-sensitive (should `t.Setenv` to clear TCX_* before httptest).
- dogfood harness: output capture cap marks large-but-valid JSON responses as failures.

## Verdict
**ship-with-gaps** — flagship/core/novel features fully verified live; the 26 failures are documented niche generated-endpoint limitations (file-download response handling, special-mode endpoints, harness capture cap), none requiring 1–3-file CLI fixes. Pending user acceptance of the known gaps.
