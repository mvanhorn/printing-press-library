# bls-pp-cli Polish Pass

## Summary

| Metric | Before | After | Delta |
|---|---|---|---|
| Scorecard | 76/100 | 77/100 | +1 |
| Verify    | 100%   | 100%   | 0 |
| Dogfood   | WARN   | PASS   | improved |
| go vet    | 0      | 0      | 0 |
| Tools-audit | 0 pending | 0 pending | 0 |

## Fixes applied

1. **Dead helper removed** — `extractResponseData` in helpers.go was unused.
2. **FTS5 crash on punctuation** — `series search "U-3"` previously crashed SQLite with `no such column: 3`. Tokens are now wrapped in double quotes before being passed to FTS5.
3. **National-headline ranking bias** — `series search "unemployment rate"` now returns LNS14000000 (U.S. headline U-3) before per-state LAUS breakouts when no `--area` filter is set.
4. **Invalid SA/NSA example corrected** — research.json + rendered README/SKILL had `CU0000SA0` (missing the position-3 SA/NSA letter). Replaced with `CUUR0000SA0`. The CLI's compare-sa validator was correct; the docs were the bug.

## Skipped (deferred)

- "national unemployment rate" returns null — needs synonym/OR-token expansion. Wave B warning, not a ship blocker.
- Historical extremum live-check failure — environmental (BLS needs registration key). CLI code path is correct.
- publish-validate manifest+phase5 failures — mid-pipeline; parent promote step writes printer fields and tools-manifest.json.

## Ship recommendation: **SHIP**
