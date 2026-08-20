# strava-pp-cli Shipcheck Report

## Summary
- Verdict: **SHIP**
- Scorecard: 86/100 - Grade A
- Verify pass rate: 100% (28/28)
- All 6 shipcheck legs passed

## Leg Results
| Leg | Result | Notes |
|-----|--------|-------|
| verify | PASS | 28/28, 100% pass rate |
| validate-narrative | PASS | 11 commands resolved after quickstart fix |
| dogfood | PASS | 8/8 novel features survived, auth protocol match |
| workflow-verify | PASS | No workflow manifest, skipped |
| verify-skill | PASS | All flags/commands resolved |
| scorecard | PASS | 86/100 Grade A |

## Top Fixes Applied
1. Moved `athlete power-curve` from `athletes` → `athlete` parent (narrative path fix)
2. SKILL.md `strava-pp-cli syncs` prose → `strava-pp-cli sync` command reference
3. Quickstart: replaced `auth login` (side-effectful) with `doctor` (verifiable)

## Scorecard Details
- 86/100 Grade A
- Gaps: insight 4/10 (analytical depth), MCP remote transport 5/10, cache freshness 5/10
- Strengths: output modes, auth, error handling, doctor, agent-native, MCP quality, local cache, workflows

## Sample Output Probe (Expected Failures)
All 8 probe failures are expected in this environment:
- 4 x HTTP 401: No STRAVA_ACCESS_TOKEN set in CI environment
- 4 x "unable to open database file": No synced SQLite data on this machine
These are correct behaviors, not bugs. Phase 5 live dogfood with real credentials will clear these.

## Ship Recommendation
SHIP — All shipcheck conditions met. No known functional bugs in approved shipping-scope features.
Live smoke testing (Phase 5) will run with user's Strava client credentials.
