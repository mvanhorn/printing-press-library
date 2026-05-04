# Apartments-pp-cli Polish Report

## Delta

|                | Before  | After   | Delta |
|----------------|---------|---------|-------|
| Scorecard      | 76/100  | 84/100  | +8    |
| Verify pass    | 100%    | 100%    | —     |
| Dogfood        | WARN    | PASS    | ↑     |
| Tools-audit    | 0       | 0       | —     |
| Go vet         | 0       | 0       | —     |
| verify-skill   | PASS    | PASS    | —     |

## Fixes applied

1. Removed 5 dead helper functions (extractResponseData, printProvenance, replacePathParam, wantsHumanTable, wrapWithProvenance) from internal/cli/helpers.go — these were generator-scaffold residue not used by the hand-written commands.
2. Restored the `--plain` flag with a proper TSV implementation (`printPlain`); the flag was previously declared but unread (its only consumer was the deleted `wantsHumanTable`).

## Skipped findings

- verify EXEC FAIL on digest/rentals/sync-search — domain-required-flags, not a defect.
- Type Fidelity 3/5 — synthetic spec; path validity N/A.
- Data Pipeline Integrity 7/10 — no `sql` command in this CLI's data layer.
- Breadth 7/10 / Vision 6/10 — surface-area dimensions; out of scope for polish.
- MCP Token Efficiency 7/10 / MCP Remote Transport 5/10 — spec-level fixes, regenerate-required, out of scope mid-pipeline.

## Ship recommendation

**`ship`** — all gates pass (verify ≥80, scorecard ≥75, dogfood PASS, verify-skill exit 0, workflow-verify not workflow-fail, tools-audit 0 pending, go vet clean).
