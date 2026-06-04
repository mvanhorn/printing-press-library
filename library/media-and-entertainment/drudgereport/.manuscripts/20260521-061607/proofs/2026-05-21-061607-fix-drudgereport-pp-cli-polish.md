# Drudge Report Polish Result

## Delta
- Verify: 100% → 100% (no change, already maxed)
- Scorecard: 85/100 → 87/100 (+2)
- Tools-audit: 0 pending → 0 pending
- Dead Code: 3/5 → 5/5
- Dead flags: 1 → 0
- Dead functions: 2 → 0
- Drudge package tests: 0 → 3 (TestOutboundDomain, TestRSSSlot, TestParseHTMLBasicZones)

## Fixes applied
- Removed dead `allowPartialFailure` flag and supporting state in `internal/cli/root.go`
- Removed dead helpers `partialFailureErr`, `detectPartialFailure`, and `partialFailureReport` in `internal/cli/helpers.go` (Google-Ads-shaped batch-failure machinery, not relevant to Drudge)
- Filtered empty `outbound_domain` rows in `sources` and `bent` SQL queries so leaderboards never lead with a blank-key row
- Added `drudgeStoreEmpty` and `emitDrudgeNoData` helpers; wired `sources`, `tenure`, `bent`, `tail` to emit the same `{"error":"no_data","message":"..."}` envelope as `digest` so JSON callers can distinguish "empty window" from "empty store"
- Added `internal/drudge/parser_test.go` with table-driven tests for `outboundDomain`, `rssSlot`, and `ParseHTML` zone detection (clears dogfood's pure-logic-no-tests warning)

## Skipped findings (with rationale)
- **`feed`/`page` unregistered commands**: intentionally suppressed; `feed` had a cross-host URL concat bug in the generator, `page` returned raw 89 KB HTML (user-facing surface is `splash`/`headlines`/`breaking`). Both kept linked via `_ = newFeedPromotedCmd` to remain compilable.
- **`defaultSyncResources` empty**: spec has no bulk-list endpoints by design; Drudge population happens via `splash`/`headlines` as a side effect of fetch, not `sync`.
- **`internal/drudge/fetch.go` no rate limiter**: single-endpoint-per-command; the existing `internal/client` transport handles rate limiting where it matters.
- **MCP token efficiency 7/10**: only 2 public MCP tools at this scale; the scorer dim is calibrated for large APIs.
- **Cache Freshness 5/10**: the `drudge_snapshot.captured_at` column IS the freshness model; no separate cache-freshness helper needed.
- **Type Fidelity 3/5**: Drudge's response is HTML; adding fake types would be scoring-game scaffolding.

## Verdict
- ship_recommendation: **ship**
- further_polish_recommended: **no** — all Phase 1 gates pass cleanly; remaining items are structural by design.

```
---POLISH-RESULT---
scorecard_before: 85
scorecard_after: 87
verify_before: 100
verify_after: 100
dogfood_before: WARN
dogfood_after: WARN
ship_recommendation: ship
further_polish_recommended: no
```
