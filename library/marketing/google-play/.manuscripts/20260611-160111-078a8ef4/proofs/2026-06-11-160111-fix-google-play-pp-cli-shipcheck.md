# Google Play CLI — Shipcheck Report

## Verdict: ship

## Shipcheck umbrella (6/6 legs PASS)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 86/100 — Grade A
Strong: Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native, MCP Desc Quality, MCP Remote Transport, Local Cache, Workflows, Insight (10/10), Path Validity, Sync Correctness — all 10/10.
Weaker (acceptable for a no-auth from-website CLI): Cache Freshness 5/10 (no auto-refresh; manual `top`/`reviews` snapshot model is intentional), Breadth 7/10, Vision 7/10, Data Pipeline Integrity 7/10, Type Fidelity 2/5 (positional protojson hand-parsed into Go structs; the generic JSON output is correct), Dead Code 4/5.

## Sample Output Probe: 7/7 (100%)
Before/after fix: the 4 local-store transcendence commands initially emitted a bare `[]` on an empty DB (losing the queried subject), failing the probe at 3/7. Fixed by emitting an identifying empty-state view (appId/term + a sync hint) instead of `[]`. Insight rose 4/10 -> 10/10 and the probe to 7/7.

## Top blockers found and fixed
1. **verify-skill FAIL (11 findings)** — `--country` referenced in README/SKILL but not detected in source, because it was registered via a shared helper. Fixed by promoting `--country`/`--lang` to persistent root flags (locale flags applicable to every command). verify-skill now exits 0.
2. **Sample probe 3/7** — local-store commands lost the subject on empty DB. Fixed (above).

## Behavioral correctness (in-session, live)
Every novel + absorbed command was exercised against the live store and a real snapshot DB and produced correct output (see build log). No flagship feature returns wrong/empty output on populated data.

## Verify pass rate
- before/after fixes: verify PASS both times (runtime breakage was never present); the two fixes were verify-skill + scorecard sample probe.
- scorecard total: 84 -> 86.

## Known gaps
None blocking. review-digest complaint-term frequency is mechanical (no NLP) by design; cache freshness is manual-snapshot by design for a rate-limited no-auth source.
