# Shipcheck Report — robinhood-agentic-pp-cli (Phase 4)

## Umbrella result: PASS (7/7 legs)

| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | runtime probes across the command tree |
| validate-narrative | PASS | 9 examples ok, 0 failed; 1 unsupported (`auth login` is side-effectful — expected) |
| dogfood | PASS | mechanical matrix; no dead flags/paths/example drift; novel features present |
| workflow-verify | PASS | primary workflow |
| apify-audit | PASS | n/a (no Apify actors) |
| verify-skill | PASS | every SKILL flag/command/positional resolves to shipped source |
| scorecard | PASS | 94/100 Grade A |

## Scorecard: 94/100 (Grade A)

Perfect or near-perfect on Output Modes, Auth, Error Handling, README, Doctor, Agent Native, MCP Remote Transport, MCP Tool Design, MCP Surface Strategy, Local Cache, Breadth, Vision, Workflows, Insight. MCP Quality 8/10, Terminal UX 9/10, Agent Workflow 9/10, Type Fidelity 5/5, Dead Code 5/5, Path Validity 10/10, Auth Protocol 10/10, Sync Correctness 10/10.

Two sub-10s (both structural, not functional):
- **Cache Freshness 5/10** — the sync/cache TTL heuristics score conservatively for a brokerage API where staleness matters; acceptable (live reads always available via `--data-source live`).
- **Data Pipeline Integrity 7/10** — reflects the hand-written MCP transport bridge that the structural scorer can't fully introspect (it expects a REST client). Behaviorally verified by the transport unit-test suite.

## Top blockers found + fixed
1. **validate-narrative FAIL** — the `watchlists quotes <list>` recipe named a command that didn't exist. Rather than drop an absorbed feature (manifest row #49), built `watchlists quotes <list-id>` (items→quotes join). Re-validated: PASS.

## Before/after
- verify pass rate: PASS on first full run (after Phase 3 build).
- scorecard: 94/100 stable across both runs.

## Behavioral correctness (novel features)
Every transcendence command exercised via its dry/empty path (guard set/status, portfolio history, audit, surface diff, winrate/wheel/settle arg-validation → exit 2, all dry-runs → exit 0). Live behavioral verification is Phase 5 (read-only; no transfers).

## Final recommendation: **ship** (pending Phase 5 read-only live smoke)

All ship-threshold conditions met: shipcheck exits 0, every leg PASS, scorecard 94 ≥ 65, no flagship feature returns wrong/empty output in structural testing. No known functional bugs in shipping-scope features.
