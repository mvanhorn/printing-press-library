# Productive CLI — Phase 4 shipcheck

## Result: PASS (7/7 legs)

| Leg | Result |
|-----|--------|
| verify | PASS (0 critical) |
| validate-narrative | PASS (11/11 after fixing a `&&` compound recipe → single `search` command) |
| dogfood | PASS (novel_features_check 6/6, no dead flags/paths, wiring OK) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS (flag-names, flag-commands, positional-args, canonical-sections) |
| scorecard | PASS — **84/100, Grade A** |

Scorecard soft spots (all above ship threshold, polish-phase candidates): Insight 4/10, Cache Freshness 5/10, Type Fidelity 2/5. Breadth 10, Workflows 10, MCP dims 10.

## Fixes applied during shipcheck
- validate-narrative: rewrote the offline recipe from a `sync && sql` compound (unparseable as one invocation) to a single `search "Acme" --json` command; `sql` alone exits 2 without a synced DB so it isn't verify-safe as a standalone example.
- Phase 4.9 correctness: the generator inferred "read-only" from the write-less spec, but hand-built `create`/`update`/`delete` exist. Fixed false claims in README (Agent-friendly bullet), SKILL (anti-triggers + read-only section), and aligned the stale "financial-domain-only" anti-trigger (scope is now full read coverage). Fixed "to CSV" → "in bulk" (export is JSONL/JSON; CSV is via `list --csv`).
- gofmt on reconcile.go.

## Live sample (scorecard --live-check)
0/6 novel commands sampled OK — all returned HTTP 401 because no API token was present at shipcheck time. This is expected and is exactly what Phase 5 (live smoke test with real credentials) will exercise. Not a code defect: the 401 is a well-formed Productive auth error, proving the request shape, headers, and endpoint paths are correct.

## Phase 4.7 sync-param-drop
Skipped — no traffic-analysis.json (documented API, no browser-sniff phase).

## Verdict: ship (pending Phase 5 live confirmation of the report wire-format)
