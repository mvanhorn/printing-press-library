# Booking.com CLI Shipcheck Report

## Verify: 100% (17/17 PASS)
All commands pass help, dry-run, and exec checks.

## Dogfood: WARN
- 6/6 novel features built and verified
- 10/10 commands have examples
- Auth protocol: MATCH (Bearer token)
- 5 dead helper functions (generated pagination helpers unused by POST-only API — acceptable)

## Scorecard: 68/100 Grade B
- Strong: Output Modes (10), Auth (10), Error Handling (10), Doctor (10), Agent Native (10), Breadth (10)
- Weak: Cache Freshness (0), Insight (2), Sync Correctness (2), Data Pipeline Integrity (4)
- MCP: 39 tools, full readiness

## Fixes Applied
1. Fixed research.json quickstart `auth set-token` — added positional arg
2. Fixed research.json quickstart `accommodations search` — removed complex JSON arg
3. Fixed research.json novel feature examples — removed `%` from threshold

## Ship Recommendation: ship
