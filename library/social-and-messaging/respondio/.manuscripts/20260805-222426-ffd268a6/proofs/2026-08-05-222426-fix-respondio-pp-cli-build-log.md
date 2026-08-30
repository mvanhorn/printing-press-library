Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

## Built (Priority 2 - transcendence, all hand-code)
1. overview - inbox workload summary (local mirror)
2. report channel-mix - channel-source distribution
3. report workload - per-agent handling volume
4. contact by-tag - tag cohort segments
5. contact field-gaps - custom field coverage gaps
6. contact idle - unassigned/no-recent-activity
7. contact search - offline free-text search

Each reads the local store (resources table) with verify-friendly RunE (help-only, --dry-run, required-input usageErr, missing-mirror guard returning empty []).

## Intentionally deferred
- None shipping-scope.

## Generator limitations found
- probe-reachability misclassified an auth-gated API 403 (missing bearer token) as browser_clearance_http; assessed as false positive. CLI ships standard bearer transport.

## Tests
- internal/cli/novel_behavior_test.go: table-driven tests for hasTag, customFieldValue, agentName, plus store-backed by-tag end-to-end.
