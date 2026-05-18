# Acceptance Report: brainos

Level: Quick Check
Tests: 6/6 passed

## Results
1. doctor — PASS: API reachable, 35 tables synced (338 total records)
2. thoughts list --limit 3 --json — PASS: real data returned from live API
3. trading-sessions list --json — PASS: empty (no sessions in db yet)
4. mcp-servers list --json — PASS: 2 real MCP servers returned
5. sync --full — PASS: 338 records, 36 resources, 0 errors, 1.6s
6. brain since 168h --json — PASS: 35 thoughts returned (correct 7-day window)

## Novel command spot-checks
- brain since 168h: 35 events, types: [thought] ✓
- trading calibrate: returns valid JSON (sparse data — no sessions yet) ✓
- memory load: returns valid JSON ✓
- brain lag: 50 thoughts, 0 matched tasks (natureos-dual-model-tasks empty) ✓
- trading pulse: "no sessions synced yet" message (expected) ✓

## Fixes applied
1. resource_type in SQLite uses hyphens not underscores — fixed in all 6 novel files

Gate: PASS
