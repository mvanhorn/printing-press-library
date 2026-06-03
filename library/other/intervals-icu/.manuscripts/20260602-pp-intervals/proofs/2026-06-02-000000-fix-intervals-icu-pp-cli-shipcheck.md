# intervals.icu CLI — Shipcheck

## Shipcheck umbrella: 6/6 legs PASS
verify PASS | validate-narrative PASS | dogfood PASS | workflow-verify PASS | verify-skill PASS | scorecard PASS

## Scorecard: 95/100 — Grade A
- Agent Native 10, Local Cache 10, Breadth 10, Vision 10, Workflows 10, MCP transport/design/surface 10.
- Soft gaps: Insight 4/10, Cache Freshness 5/10, MCP Quality 8/10, Type Fidelity 4/5.

## Live dogfood (Phase 5): 282/282 PASS (full matrix, real API)
- Auth confirmed end-to-end: constant-username Basic (base64("API_KEY:<key>")). athlete 0 -> resolved self.
- All 5 novel commands verified against live data: form, curve compare, wellness trends, since, gear status.
- Initial live-probe surfaced 2 real bugs, both fixed:
  1. curve endpoints require an ActivityType `type` param (HTTP 422) -> added --type (default Ride).
  2. since accepted any positional, silently defaulting a garbage window -> now validates (exit 2).
- curve compare peak extraction fixed for intervals.icu's {list:[{secs,values}]} shape.

## Fixes applied (printed-CLI)
- config.go AuthHeader constant-username Basic.
- sync.go wired activity/wellness/events/gear/workouts with {id}=athlete id, oldest since-param.
- 5 novel commands implemented + table-driven tests.

## Verdict: ship
All ship-threshold conditions met; no known functional bugs in shipping-scope features.
PII: live responses contained athlete name/activity names; redacted from this report per cardinal rule.
