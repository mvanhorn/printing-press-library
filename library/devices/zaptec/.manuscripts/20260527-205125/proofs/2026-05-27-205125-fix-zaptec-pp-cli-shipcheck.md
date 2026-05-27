# Zaptec CLI — Shipcheck

## Final result: PASS (6/6 legs)

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | command tree wiring, examples, data wiring |
| verify | PASS | runtime + auto-fix |
| workflow-verify | PASS | primary workflow |
| verify-skill | PASS | every SKILL flag/command exists in source |
| validate-narrative | PASS | all 10 quickstart/recipe commands resolve + dry-run |
| scorecard | PASS | 89/100 — Grade A |

## Scorecard 89/100 (Grade A)
Strong: Output Modes 10, Auth 10, Error Handling 10, Doctor 10, Agent Native 10, MCP Quality 10, Local Cache 10, Workflows 10, Path Validity 10, Auth Protocol 10, Data Pipeline 10, Sync 10, Breadth 9.

## Blockers found and fixed
1. **Go toolchain conflict (env, not code).** Local Go 1.24.4 vs generated `go 1.26.3`; govulncheck pinned to 1.25.10 and couldn't load 1.26 packages. Fixed by exporting `GOTOOLCHAIN=go1.26.3` (toolchain present in module cache). All Go gates pass under it.
2. **validate-narrative FAIL** — `auth login` quickstart example errored under `--dry-run` (demanded credentials first). Fixed: `auth login` now short-circuits cleanly on `--dry-run`.
3. **verify-skill FAIL** — false failure: the leg shells out to `python3`, which this Windows box intercepts with the Microsoft Store stub. Fixed by putting the real Python 3.13 + a `python3` shim on PATH. Re-ran: "All checks passed". Not a SKILL/source mismatch.
4. **Inaccurate recipe** — a SKILL `state --select operationMode,...` example named fields the decoded `state` output doesn't have. Corrected to `--select name,value` in SKILL.md and research.json.

## Known gaps (non-blocking, scorecard-only)
- `mcp_token_efficiency` 4/10, `mcp_remote_transport` 5/10, `mcp_tool_design` 5/10 — the MCP surface mirrors a moderately large command tree (endpoint-mirror). Improving these needs spec-level MCP enrichment (transport/orchestration) and is a polish-phase candidate, not a ship blocker.
- Scorecard "Sample Output Probe: binary not executable" — Windows-only: the probe looks for the binary without `.exe`. Cosmetic.

## Behavioral spot-checks (offline / no creds)
- `pause <id> --dry-run` → `POST .../sendCommand/502`, exit 0 (correct command-ID mapping).
- `cost --by month --json` on empty store → valid JSON, `total_kwh: 0`.
- `sessions anomalies` / `chargers stale` empty → `[]` (not null).
- Unit tests for decode tables + round2 pass.

## Verdict: ship
All ship-threshold conditions met. No known functional bugs in shipping-scope features. Live API verification (against the user's real charger) is pending Phase 5 credentials.
