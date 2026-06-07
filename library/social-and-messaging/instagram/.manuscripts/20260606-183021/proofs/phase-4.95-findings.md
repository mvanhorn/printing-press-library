# Phase 4.95 Local Code Review — instagram

Review path: direct subagent dispatch (correctness + security + maintainability) over
hand-authored source (internal/store/instagram_analytics.go, internal/cli/instagram_brands.go,
+ the 7 novel command files). Generator-reserved internal/cliutil and internal/mcp/cobratree
out of scope.

## Autofix summary
6 findings autofixed in-place across 1 round (build/vet/test green after):
- Subagent round: 3 fixes — missing rows.Err() checks in pullBrandRivals + pullBrandHashtags
  (silent tracked-row drop); split media-list parse-failure from genuine-empty; `cap` builtin
  shadow rename.
- Agent (post-review) round: 3 correctness fixes in the headline data path —
  1. fetchInsightTotals now classifies errors: HTTP 400 tolerated (unsupported metric), but
     401/403/429/5xx/transport/parse errors propagate, so a bad token can no longer silently
     persist zero-filled account/media snapshots that would corrupt growth/compare/top-posts.
  2. account-snapshot now only writes when the profile resolved, and writes NULL (not 0) insight
     columns when insights errored — prevents all-zeros pollution of the time-series.
  3. media-insight auth/transport errors now surface as fetch-failures instead of zeroing metrics.
  4. compare + hashtag-perf latest-row self-joins switched from MAX(captured_at) to MAX(id),
     eliminating duplicate brand/hashtag rows on same-second double-pull ties.

## Verified clean (no fix needed)
SQL injection (all parameterized; metric column from whitelist map), NULL-safe scans,
resource leaks (defer Close), division-by-zero guards, parseIGTime used for Graph +0000
timestamps everywhere, goroutine safety in pull (distinct slice indices, mutex-protected
limiter), dry-run short-circuits before IO, usageErr validation.

## Out-of-scope retro candidates
None observed.

## Known minor limitations (surfaced, intentionally not changed)
- Reels watch-time read as int64 then cast to float64 — truncates sub-unit precision in the
  `formats` watch-time column. Negligible (watch-time is large-int ms); a float-returning
  insight fetch would be needed to preserve it. Acceptable for v1.
- pullBrandMedia fans out one goroutine per media item, bounded by --media-limit (default 25).
  Fine at default; a bounded worker pool would be warranted only if the limit is raised much higher.

Convergence: findings cleared at round 1 (in-scope). Build/vet/go test ./internal/... all PASS.
