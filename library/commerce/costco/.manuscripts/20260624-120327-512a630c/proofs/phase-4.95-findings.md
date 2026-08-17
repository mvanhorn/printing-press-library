# Phase 4.95 Local Code Review — costco-pp-cli

Reviewer: feature-dev:code-reviewer subagent over hand-written source.

## Autofixed in-place
- **C1 (HIGH) store.go QueryJSON read-only guard inverted + connection read-write.**
  Fix: added `store.OpenReadOnly` (SQLite `query_only(true)` pragma on every pooled
  connection); `sql`/`search` now use it. Corrected the multi-statement guard to reject
  ANY interior `;` (previously a trailing `;` let `SELECT 1; DROP TABLE x;` through).
  Removed the latent exported `DB()` read-write accessor. Added TestOpenReadOnlyBlocksWrites
  + chained-statement cases to store_test.go.
- **I1 (MED) orders.go `--max-pages 0` silently returned empty.** Added clamp to 10.
- **I2 (MED) orders.go extra round-trip on exact page boundary.** Added `len(got) < pageSize` break.

## Noted, verify in Phase 5 live (not autofixed)
- **I5 (LOW) savings double-count risk.** computeSavings reports InstantSavings, CouponSavings,
  and TotalSavings separately; if Costco folds coupons into instantSavings upstream, TotalSavings
  double-counts. Fields are surfaced separately so the breakdown is visible. Verify against a real
  receipt that carries both during Phase 5.

## Clean (reviewer confirmed)
costco_graphql.go, receipts.go, search.go, export.go, history_depth.go, spend.go,
item_history.go, returns_window.go, store.go Upsert/SearchItems, doctor.go JWT block.

Review path: feature-dev:code-reviewer subagent (Agent tool). Convergence: in-scope findings cleared in 1 round.
