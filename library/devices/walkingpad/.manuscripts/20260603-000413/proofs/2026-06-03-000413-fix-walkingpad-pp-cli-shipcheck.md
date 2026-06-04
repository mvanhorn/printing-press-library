# WalkingPad CLI — Shipcheck

## Leg results (`shipcheck --no-live-check`)
| Leg | Result | Notes |
|-----|--------|-------|
| verify | **FAIL (1)** | 22/22 commands PASS help+dry-run+exec (100%, 0 critical). Sole failure: "Data Pipeline: sync crashed" — verify runs `<binary> sync` unconditionally; a BLE device CLI has no `sync`. Machine gap, not a CLI defect. |
| validate-narrative | PASS | 10 narrative commands resolved; full examples pass under verify. |
| dogfood | PASS (leg exit 0) | Verdict FAIL reasons are all device-CLI-shape gaps (see below). Novel features 5/5 survived. |
| workflow-verify | PASS | |
| verify-skill | PASS | All SKILL flags/commands/positional-args exist. |
| scorecard | PASS (leg) | Low HTTP-API dims (Insight/Agent-Workflow/Error-Handling) are largely N/A for a device CLI. |

## Behavioral correctness (manual)
- All 22 commands: help/dry-run/exec PASS.
- `speed 3.0 --dry-run` emits correct CRC payload `f7a2011ec1fd` (verified vs protocol).
- Safety gating verified: physical-effect/config writes require `--live` + `--confirm-physical-effect`; verify-env no-ops (exit 0); dry-run previews exact payload.
- `today`/`trends`/`streak`/`calories`/`export` operate on the local store (empty-store safe).
- Unit tests pass for the protocol codec (CRC + status/record parse vs documented frame), history rollups/streak, and the MET calorie model.

## Machine gaps (retro candidates — NOT CLI defects)
`verify` and `dogfood` have no device-spec (`protocol: ble`) awareness and apply HTTP-API-only checks to device CLIs:
1. **verify data-pipeline** runs `<binary> sync` → "sync crashed" (device CLIs have no sync). `internal/pipeline/runtime.go:runDataPipelineTest`.
2. **dogfood auth-protocol** reads `internal/client/client.go` → "MISMATCH: file not found" (device CLIs have no HTTP client).
3. **dogfood examples** discover via `agent-context` → SKIP (device CLIs don't emit `agent-context`).
4. **scorecard** scores HTTP-API dims (Path Validity, Auth Protocol, Insight, Agent Workflow) as N/A/0 for device CLIs.

Fixing these requires `verify`/`dogfood`/`scorecard` to detect `protocol: ble` device specs and skip/replace the HTTP-API checks. Out of scope for a printed CLI; strong retro signal for the device-CLI generator track.

## Verdict
- **CLI: functionally complete and correct.** All applicable checks pass; the only shipcheck FAILs are verifier false-negatives for the device-CLI shape.
- **Real correctness gate is Phase 5 live dogfood** against the physical WalkingPad C2 (user has the device; protocol confirmed live during discovery).
- Recommendation: **ship pending live validation** (Phase 5). Document the verifier gaps; do not fake `sync`/`client.go`/`agent-context` to satisfy HTTP-API checks (would violate anti-reimplementation).
