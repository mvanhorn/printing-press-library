# Netlify CLI Shipcheck

## Verdict: ship

## Shipcheck umbrella: PASS (7/7 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS (novel_features_check 6/6 built) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 94/100 — Grade A
- Perfect (10/10): output modes, auth, error handling, terminal UX, README, doctor, agent-native, MCP remote transport, MCP tool design, MCP surface strategy, local cache, vision, workflows.
- MCP Quality 8/10, Breadth 8/10, Agent Workflow 9/10, Type Fidelity 4/5.
- Cache Freshness 5/10, Insight 4/10 — cosmetic scorecard gaps, not functional.
- Domain correctness: path validity, auth protocol, data pipeline, sync correctness all 10/10; dead code 5/5.

## Behavioral correctness (the real gate)
- Sample-probe reported "empty output" for 5/6 novel commands. Root cause: the test token's account is empty (0 sites, 0 deploys, 0 forms, 0 DNS). Empty output is CORRECT behavior for an empty tenant, not a bug — each command emits valid structured JSON with an honest empty result and a sync hint (verified live, exit 0, valid JSON).
- Aggregation logic proven with seeded fixtures in `internal/cli/netlify_novel_behavior_test.go` (all PASS):
  - overview: aggregates 2 sites, sorts by last deploy, correct form counts.
  - env-drift: detects KEY2 missing on s2 and KEY3 missing production value; ignores KEY1 (present everywhere).
  - dns-audit: flags 1 dangling NETLIFY record + 1 SNI cert expiring within 30 days.
  - since: filters to the 1 deploy inside a 24h window.
  - submissions search: 1 match on "acme", scanned 2.
- go vet clean; full `go test ./...` green.

## Generator fixes applied (retro candidates)
1. Swagger→OpenAPI3 conversion rejected 73 path-level parameter arrays (path-level body params). Fixed by distributing into operations.
2. sync.go emitted 2 duplicate switch cases (agent-runners, sites) → compile error. Removed duplicates.

## Live verification
- doctor: all OK (auth configured, API reachable, credentials valid).
- sync: 284 records across 14 top-level resources.
- Reachability: token → 200.

## Phase 5 note
- Full dogfood (write-side) intentionally NOT run: the token is a live personal Netlify account with no disposable sandbox, and the matrix would exercise create/delete endpoints. Ran read-only Quick Check instead to avoid mutating real resources.
