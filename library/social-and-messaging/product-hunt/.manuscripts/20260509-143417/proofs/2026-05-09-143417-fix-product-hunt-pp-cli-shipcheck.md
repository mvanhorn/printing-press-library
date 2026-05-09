# Shipcheck Report: product-hunt-pp-cli

## Result: PASS (6/6 legs)

| Leg | Result |
|-----|--------|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS (no manifest) |
| verify-skill | PASS |
| validate-narrative | PASS |
| scorecard | PASS |

## Scorecard: 74/100 — Grade B

Gaps:
- path_validity: 0/10 — expected for GraphQL-only API (no REST paths in spec)
- dead_code: cleaned up (removed classifyDeleteError, extractResponseData, replacePathParam, truncateJSONArray, ignoreMissing flag)

## Fixes applied this session

- digest.go: added `--yesterday` flag + `topicFilter` filtering in topic loop
- posts_vote_rate.go: added `--topic` and `--days` flags with filtering logic
- topics_trending.go: added `--days` flag (configurable comparison window)
- SKILL.md: fixed `posts cross-topic` positional syntax → `--topics` flag; fixed "syncs" prose
- research.json: fixed cross-topic examples, sync --days → sync --since
- helpers.go: removed 4 dead functions (classifyDeleteError, extractResponseData, replacePathParam, truncateJSONArray)
- root.go: removed dead `--ignore-missing` flag
- internal/phgraphql/client_test.go: added 3 table-driven tests for New, NewDryRun, isNumericID
- Added pp:client-call annotations to novel commands making live API calls

## Ship recommendation: ship

All mandatory gates pass. No functional bugs in approved features. No API key available so live smoke testing skipped (Phase 5 skipped with phase5-skip.json).
