# Power BI CLI — Shipcheck Proof

## Summary
- Run: 20260512-201900
- Scorecard: 85/100 — Grade A
- Verdict: **ship**

## Shipcheck legs (after fixes)
| Leg | Result | Note |
|---|---|---|
| dogfood | PASS | All 7 planned novel features built; novel_features_built synced to README/SKILL/root help |
| verify | PASS | All commands wired, --dry-run works, exit codes correct |
| workflow-verify | PASS | No workflow_verify.yaml needed |
| verify-skill | PASS* | *Umbrella reports FAIL on Windows due to Python cp1252 codec crash on ✓ char; with PYTHONIOENCODING=utf-8 reports "All checks passed" |
| validate-narrative | PASS | All 11 narrative commands resolved and dry-run successfully |
| scorecard | PASS | 85/100 Grade A |

## Built in Phase 3
- internal/cli/auth_login.go — device-code (default) + service-principal client_credentials
- internal/cli/auth_doctor.go — JWT decode + tenant-settings failure-mode explainer
- internal/cli/dax.go — dax run / save / list / show / delete with --csv, --file, --query, --out, --include-nulls
- internal/cli/report_export.go — POST → poll → download in one command with --wait
- internal/cli/refreshes_failures.go — iterate workspaces+datasets, surface failures
- internal/cli/dataset_describe.go — INFO.TABLES/COLUMNS/MEASURES with metadata fallback

## Scorecard gaps to note (non-blocking)
- mcp_token_efficiency 0/10 — the MCP orchestration tool surface emits more bytes than ideal for an agent's initial context. Acceptable for v1; mcp_remote_transport, mcp_tool_design, mcp_surface_strategy all score 10/10.
- cache_freshness 5/10 — only the framework sync is wired; per-resource freshness staleness checks not added.
- type_fidelity 3/5 — the spec uses minimal typing; some response bodies are passed as raw JSON because Power BI responses include schema-flexible fields.
- dead_code 4/5 — generator emitted a small handful of utility funcs not yet referenced by hand-written code.

## Notes on the build
- Spec hand-crafted: no OpenAPI exists for the user-facing Power BI REST API (only ARM specs in azure-rest-api-specs for capacity/embedded/private-links). Microsoft Learn docs are the canonical reference.
- ExecuteQueries endpoint deliberately not in the spec — built as the hand-written  command because the request body shape (queries array of objects) doesn't fit the internal YAML's flat body-param model.
- Browser-sniff gate: skip-silent (Microsoft Learn docs are complete; report-viewer URL the user provided is not an API endpoint).
- Reachability probe: 403 from /groups — expected when no auth header, treated as PASS per the matrix.

## Known gaps in v1 (disclosed in absorb manifest as stubs)
- Auto-pagination across the 100K row DAX cap — deferred. Power BI's executeQueries has a 100K row / 1M value / 15MB cap per call; rewriting arbitrary DAX with TOPN windowing requires partition-key UX that wasn't worth inventing under time pressure. Users hit the cap should rewrite their query with TOPN or filters.

## Live testing
- Phase 5 dogfood: skipped (phase5-skip.json: auth_required_no_credential). The host environment lacks Azure CLI and an interactive browser session to drive the device-code flow.
- First live test will be the user's: `powerbi-pp-cli auth login` (device code) then `powerbi-pp-cli auth doctor` to verify, then `powerbi-pp-cli groups list --json` to confirm visibility.
