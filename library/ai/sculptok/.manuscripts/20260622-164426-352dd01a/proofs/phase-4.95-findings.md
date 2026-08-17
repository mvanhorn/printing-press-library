# Phase 4.8/4.9/4.95 Review Findings

## Docs (4.8/4.9) — PASS with 2 warnings, both fixed in README.md
- "account credits" -> "credits balance" (no `account` command exists).
- "auth-status" -> "auth status" (correct subcommand name).

## Code review (4.95) — no high severity; fixed in place
- [MEDIUM] internal/sculptok/client.go: query params concatenated unescaped -> switched to url.Values/Encode (promptIDs, page/limit now safely encoded). FIXED.
- [LOW] internal/cli/search.go: in-place slice reuse (events[:0]) -> fresh make(). FIXED.
- [LOW] internal/store/store.go Reconcile: O(events x jobs) re-query -> preload prompt_ids once. FIXED.
- [LOW] internal/store/store.go: scan errors swallowed via continue -> LEFT (deliberate local-mirror tolerance for malformed rows).

## Confirmed clean
SQL parameterized (whitelisted analytics expr switch; bound LIKE), envelope code!=0 -> typed error, resources closed (bodies/files/rows/store), no path traversal in downloadResults, AdaptiveLimiter + RateLimitError honored, verify-friendly RunE (no MinimumNArgs/MarkFlagRequired), context propagation via boundCtx, no goroutine leaks.
