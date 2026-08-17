# Wahoo CLI — Shipcheck Report

## Shipcheck umbrella: PASS (6/6 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative (--strict --full-examples) | PASS |
| dogfood | PASS |
| workflow-verify | PASS (no manifest → workflow-pass) |
| verify-skill (+ canonical-sections) | PASS |
| scorecard | PASS |

## Scorecard: 93/100 — Grade A
- 10/10: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Quality, MCP Remote Transport, Local Cache, Path Validity, Auth Protocol, Data Pipeline Integrity, Sync Correctness, Workflows, Insight.
- Type Fidelity 5/5, Dead Code 5/5.
- Breadth 8/10, Vision 9/10, Agent Workflow 9/10.
- Weak dims (polish targets, non-blocking): **MCP Desc Quality 3/10**, MCP Tool Design 5/10, MCP Token Efficiency 7/10, Cache Freshness 5/10 (cache intentionally disabled — rate-limited API).

## Reviews
- **Phase 4.8 / 4.9 (SKILL + README correctness):** consolidated reviewer. Fixed: `permissions revoke` → `permissions` (leaf command) in README/SKILL/research.json; README env table `WAHOO_CLOUD_OAUTH2` Required Yes → No (optional token override; `auth login` is primary). Auth narrative confirmed accurate (no `auth set-token`, no `WAHOO_REDIRECT_URI`).
- **Phase 4.95 (code review):** PASS, no defects. SQL all static literals filtered by hardcoded resource_type (no injection); comma-ok type assertions guard nil maps; `parseLooseFloat` excludes missing metrics from aggregates (no phantom zeros); haversine + CTL/ATL EWMA + watts/kg + TSS math verified; backup uses temp-file+rename, surfaces partial failures, guarded by IsVerifyEnv; boundCtx applied to backup HTTP. mcp:read-only annotations correct.
- **Phase 4.85 (output review):** satisfied via real-data behavioral smoke (demo SQLite DB). All 6 commands produce correct output: digest windowing + missing-metric exclusion, bests record selection, ftp progression + watts/kg, load CTL/ATL/TSB, routes find distance/ascent/haversine filters, backup record-write + partial-failure surfacing. NULL-safe string-metric parsing verified end to end.
- **Phase 4.7 (sync param-drop):** N/A — vendor OpenAPI spec, no traffic-analysis.

## Known non-bug note
Scorecard live-probe flagged `backup` "empty output". This is a no-data/no-credential artifact: the probe env has no synced mirror and no OAuth token, so backup correctly emits the "run sync first" hint and exits 0. backup is proven functional against a populated demo DB (3 records written; failed FIT download surfaced in failures[] + stderr). Not a fix-before-ship bug.

## Behavioral correctness
All novel-feature commands resolve as real Cobra commands (Phase 3 gate), pass `--help`/`--dry-run`, and produce correct output against real data. `go vet` + `go test ./internal/cli/...` green (14 hand-authored tests).

## Live testing
Skipped (Phase 5). The Wahoo Cloud API requires OAuth2 + a Wahoo-approved app; no client_id/token available. Built and verified against mocks + a populated demo DB. Marker: `phase5-skip.json` (skip_reason auth_required_no_credential). The CLI ships a working `auth login` (PKCE) for when an app is approved.

## Final verdict: SHIP
All ship-threshold conditions met: shipcheck exit 0, all legs PASS, scorecard 93 ≥ 65, no flagship feature returns wrong/empty output (backup empty = no-data artifact, verified working). MCP description quality is the one polish target → Phase 5.5.
