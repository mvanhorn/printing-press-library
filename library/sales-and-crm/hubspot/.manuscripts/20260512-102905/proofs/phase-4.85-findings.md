# Phase 4.85 — Output Review Findings (Wave B, warnings only)

## status: WARN

### format-bugs

- **Location**: `internal/cli/transcendence_helpers.go` `computeDedup` (line 553) and the analogous `computeClosedWonHandoff` path
- **Problem**: `dedup` and `closed-won-handoff` commands emit `"rows": null` for empty result sets (Go nil slice → JSON null), while sibling rankers `pipeline-health` and `engagement-decay` emit `"rows": []`. Agent consumers iterating or calling `.length` will crash on null but not on `[]`.
- **Fix**: Initialize result slices with `make([]T, 0)` instead of `var x []T` so the empty marshal yields `[]`.

## Wave B policy

Findings logged as warnings. Fix applied inline (mechanical, 1 line per command).
