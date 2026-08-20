# Sumble CLI — Shipcheck

## Shipcheck umbrella: PASS (5/5 legs)
| Leg | Result |
|-----|--------|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS (no manifest) |
| verify-skill | PASS (flag/command/positional checks) |
| scorecard | PASS — 86/100 Grade A |

novel_features_check: planned 7, found 7, none missing.

## Scorecard highlights (86/100, Grade A)
Output Modes 10, Auth 10, Error Handling 10, Agent Native 10, MCP Quality 10,
MCP Remote Transport 10, Local Cache 10, Doctor 10, Workflows 10, Sync 10,
Path Validity 10. Weaker: MCP Tool Design 5 (nested bodies emit as JSON-string
flags --filters/--organization — generator limitation for nested objects),
Type Fidelity 3/5, Cache Freshness 5, Insight 6, Vision 7, README 8.

## Bug found and fixed (fix-before-ship)
- stack-diff -> organizations/enrich returned HTTP 422 "filters Field required":
  the API requires a filters object even when empty. Fixed: stack-diff now sends
  filters:{}. (The 422 meant no credits were spent on the live sample probe.)

## Code review (Phase 4.95, self-audited; subagent hit a transient 529)
- spend.go / stale.go SQL: interpolated identifiers come only from fixed allowlists
  (validated 'by' switch; hardcoded staleTables); user values bound via '?'. No injection.
- NULL-safe scans (sql.Null* / COALESCE) throughout; *sql.Rows and *store.Store closed on all paths.
- Verify-env safe: balance probe, stack-diff enrich, and reconcile match all short-circuit
  under cliutil.IsVerifyEnv() so the verifier never spends credits.
- Budget gate returns exit 2 when an estimate exceeds the ceiling (verified).
- go vet clean.

## Doc correctness (Phase 4.8/4.9)
- verify-skill PASS (every SKILL flag/command exists in source).
- validate-narrative PASS (every README/SKILL example resolves under verify).
- README has no placeholder literals; Quick Start + recipes reflect the real flag shapes.

## Verdict: ship
All ship-threshold conditions met. No known functional bugs in shipping-scope features.
Live dogfood (Phase 5) pending — will be run frugally given usage-based pricing.
