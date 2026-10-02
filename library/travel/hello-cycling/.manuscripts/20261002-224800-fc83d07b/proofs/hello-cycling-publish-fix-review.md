# HELLO CYCLING publish-fix verification

**PASS — no concrete remaining issue found in the two publication regressions.**

Reviewed the corrected source in /Users/zjsng/Projects/Personal/Coding/hello-cycling-cli for [PR 2237](https://github.com/mvanhorn/printing-press-library/pull/2237), using the same independent reviewer. No additional agents, source edits or GitHub writes.

- **Status-feed outage/cache preservation:** internal/cycling/client.go:41 and :194–209 return StatusFeedError for a failed or HTTP-200-empty status feed, retain station discovery and omit unavailable statuses. internal/cli/hello_cycling_commands.go:123–154 selects a saved snapshot with information/status records for eligible auto fallback; without one, discovery returns explicit source_missing/null counts. HTTP 401/403/429 remain errors. Sync and changes use liveOnly and return the outage error before snapshot replacement or comparison.
- **Derived change detection/original baseline time:** internal/cycling/model.go:400 compares selected counts, rental/return states and Stale as well as aggregate counts/operational flags. internal/cli/stations_changes.go:53 evaluates the baseline at before.ObservedAt. The regressions verify available→stale and full→incompatible transitions with equal counts, while a normal fresh heartbeat is not reported as a change.
- **Callback test isolation:** hello_cycling_outage_test.go saves the original hcFetchSnapshot and restores it with defer, including early-failure paths. Its subtests are sequential. The test verifies auto/live/no-cache behavior and byte-for-byte baseline preservation on sync/changes failure. No other CLI test writes the callback.

Independent checks completed:

```text
go test ./internal/cycling ./internal/cli ./internal/mcp
PASS: cycling, CLI, MCP

go test -race ./internal/cli -run '^TestHCStatusOutageFallbackAndSnapshotProtection$' -count=1
PASS: no race reported
```

The focused tests include the new HTTP 200 empty-status schema case, typed outage/discovery boundary, usability transition comparison and command-level original-observation-time regression. Local mock-server tests required ordinary sandbox escalation. No browser/CDP was used.

Fresh provider checks, packaging and publication remain with the builder; this proof verifies the targeted fixes and callback isolation.

