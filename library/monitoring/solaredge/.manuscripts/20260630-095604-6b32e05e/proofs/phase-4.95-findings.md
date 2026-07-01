# Phase 4.95 Local Code Review Findings

**Review path:** Direct subagent dispatch via the Agent tool — 3 parallel reviewers (correctness, security, maintainability) against the 7 hand-written files (`internal/cli/site_health.go`, `site_underperformance.go`, `site_changes.go`, `equipment_faults.go`, `budget_status.go`, `solaredge_novel_helpers.go`, `internal/store/solaredge_migrations.go`). No working-dir-shaped review skill was used directly because the dispatch contract called for persona-specific lenses (correctness/security/maintainability), which the direct subagent path supports natively.

**Convergence outcome:** Findings cleared at round 1. All HIGH/Medium-severity in-scope findings were fixed; remaining items are either by-design tradeoffs (documented) or out-of-scope generator code.

## Autofix summary

7 findings autofixed in-place in round 1 (no commit hashes — working dir, not yet committed to git):
- HIGH correctness bug: `site changes`'s "prior period" slice included all history before the current window instead of an equal-length prior window (`site_changes.go`).
- Medium maintainability: `equipment faults`'s "latest battery telemetry" assumed array order instead of sorting by timestamp (`equipment_faults.go`).
- Medium maintainability: custom `solaredge_call_log` table was lazily created per-call instead of registered in the canonical `extras.go` migration hook, bypassing the store's locked/version-stamped migration transaction (`extras.go`, `solaredge_migrations.go`).
- 3x duplicated logic extracted into shared helpers in `solaredge_novel_helpers.go`: `countInventoryEquipment` (was duplicated in `site_health.go` and `site_changes.go`), `parseEnergySeriesValues` + `sumEnergyPoints` (was duplicated in `site_underperformance.go` and `site_changes.go`).
- Magic numbers given named constants with rationale comments: `solarEdgeEnergyHistoryCapDays` (364), `solarEdgeMinBaselineDays` (7), `solarEdgeMaxUnderperformanceSinceDays` (300), `solarEdgeMaxChangesSinceDays` (180), plus a doc comment citing the vendor's `batteryState` enum (0/1/2/3/4) on `checkBatteryState`.

## Surface-to-user findings

None. All findings were mechanical, small-scope, and behavior-preserving — no competing implementation existed, none implied shipping-scope shrinkage, and none implied a Phase 1 research miss.

## Template-shape retro candidates

None identified in the 7 hand-written files. (Separately, two generator-template issues were already logged in the build log: the `version`-resource-name collision and the "Solaredge" vs "SolarEdge" casing defect in generated `root.go`/`auth.go` Short/Long text — those are retro candidates from Phase 2/4.9, not this phase.)

## Out-of-scope retro candidates

- Security reviewer noted (informational, not actioned): `internal/mcp/tools.go`'s `handleSQL` runs raw agent-supplied SQL with no parameterization, mitigated by a `SELECT`/`WITH`-only allowlist and a `mode=ro` handle — this is `internal/mcp/` framework code, working as designed, not a finding against this CLI's hand-written code.
- Dogfood-reported dead helper `writeNoop` (generator-emitted, `internal/cli/helpers.go`) — already logged in the shipcheck report.

## Security review summary

No critical or high-severity findings. SQL injection: none (parameterized throughout, including the new `solaredge_call_log` queries). Credential leakage: none (API key masked in all error/log/dry-run paths via existing generator-provided masking). Path traversal: none (siteId/serialNumber values are percent-encoded via `replacePathParam`, never raw-concatenated). Unsafe deserialization / shell injection: none in scope. Missing input validation on `siteId` is informational only — downstream percent-encoding and parameterized SQL neutralize any injection risk; a malformed siteId just produces a normal 4xx from the live API.
