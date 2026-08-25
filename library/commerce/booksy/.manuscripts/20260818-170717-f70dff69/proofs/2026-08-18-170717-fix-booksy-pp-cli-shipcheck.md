# Booksy CLI — Shipcheck

## Legs (with token in env)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | HOLD (unverified dim: live_api_verification only) |

## Scorecard: 93/100 — Grade A
Notables: Output Modes 10, Auth 10, README 10, Doctor 10, Agent Native 10, MCP Quality 10, MCP Remote Transport 10, Local Cache 10, Workflows 10, Insight 10. Weaker: Error Handling 8, Cache Freshness 5, Dead Code 2/5, MCP Token Efficiency 7. `live_api_verification` unverified (omitted from denominator) — the scorecard has no credential path for its internal live probe.

## Live verification (the real signal)
- `--live-check` Sample Output Probe: **6/6 novel features passed** against the real Booksy API.
- Phase 5 **full live dogfood: 100/100** (matrix_size 100), authed reads included via the user's session token; `book` correctly refused under the harness (no real appointment placed).

## Fixes applied
1. verify-skill: `book` Use line dropped inline `--flag <val>` tokens (were miscounted as 4 required positionals) → `book <business_id>`.
2. dogfood: added missing `Examples` section to the framework `feedback` command (generator gap — retro candidate) → 100/100.
3. Corrected `availability`/`earliest` help + narrative: they are public (time_slots works without a token), not token-gated.

## Verdict: ship
All 6 real legs PASS; scorecard 93/A with every flagship/novel feature verified working against the live API; 100/100 live dogfood. The lone scorecard hold is an *unverified* (not failed) dimension the scorecard can't probe without credentials, superseded by the passing live dogfood + sample probe.

## Retro candidate
- Generator: the framework `feedback` command ships without an `Examples:` section, which dogfood's help check requires. Every generated CLI hits this. Should be emitted by the template.
