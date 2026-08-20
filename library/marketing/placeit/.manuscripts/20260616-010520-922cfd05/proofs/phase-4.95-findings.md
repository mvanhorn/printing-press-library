# Phase 4.95 Local Code Review — Placeit CLI

Reviewer: general-purpose subagent over hand-written Go (algolia, catalog, 7 novel commands).
Overall: no panics, no SQLi, no leaks, no hangs, no concurrency bugs. 3 medium correctness issues autofixed in-place + 1 low improvement.

## Autofixed (verified)
1. [med] rank.go percentile self-inclusion — sole top template could never read 100. Fixed: denominator excludes self (n-1); verified top mockup now reads 100.0.
2. [med] catalog.go search --page dropped on local/auto-fallback path — pagination silently returned page 0. Fixed: threaded page+offset into searchLocal; verified page0 != page1.
3. [med] catalog.go auto-fallback swallowed the local error and re-returned the live error. Fixed: surfaces joined "live failed (...); local fallback also failed: ..." error.
4. [low] kit.go live pool used IndexMain (relevance) — "most popular per slot" could miss high-purchase items ranked low. Fixed: live pool now IndexBestSelling.

## Accepted (low, documented)
- watch.go new_count capped by --limit (50) — a watchlist scanning the newest N; surplus growth beyond N per run not reported. Acceptable for the use case.
- gaps.go aTotals counts (a,b) pairs not distinct templates — intentional, used only to rank columns; cell counts are correct.

## Confirmed non-issues
- Division by zero guarded (percentile n==0 early return).
- Algolia public search key: read-only /query only, env-overridable. Intended.
- All store.Open* paired with defer Close(); boundCtx applied to every sibling-client call (default --timeout 60s).
