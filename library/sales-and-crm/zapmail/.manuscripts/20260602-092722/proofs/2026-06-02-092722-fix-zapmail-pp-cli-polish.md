# Zapmail CLI Polish Pass

| Metric | Before | After |
|--------|--------|-------|
| Scorecard | 93 | 93 |
| Verify | 100% | 100% |
| Publish-validate | FAIL | PASS |
| Tools-audit | 1 pending | 0 |
| Go vet | 0 | 0 |

## Fixes
- Added creator.name to .printing-press.json (manifest gate).
- Placed phase5-acceptance.json in .manuscripts/<run-id>/proofs/ (phase5 gate).
- Accepted thin-short on `version` command in tools ledger with rationale.

## Skipped (structural, not defects)
- mcp_token_efficiency 7/10: small ~22-tool typed surface; collapse pattern is for 50+ endpoint APIs; transport already [stdio,http].
- cache_freshness 5/10, data_pipeline_integrity 7/10: reflect API data model.
- profile/workflow verify execute=false: needs live auth/network; verdict PASS, 0 critical.

## Recommendation: ship (no further polish)
