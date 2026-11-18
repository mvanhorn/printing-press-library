# Phase 4.95 local code review (2026-11-17)

- Review path: direct subagent dispatch (one agent acting as correctness, security and maintainability reviewers) over internal/edeka and the hand-written internal/cli files; plus the native timeout-boundary check (boundCtx added to every hand-written RunE).
- Autofix summary: 15 findings (1 high, 6 medium, 8 low) autofixed in-place in one round: thank-you detection for PlaceOrder (no page-wide "vielen dank" match), --per-term validation, order history innermost cards and digit-bearing order numbers, list add reports partial results on failure, order place --confirm requires --expect-total and refuses on a changed total, payment options marked mutating for MCP, product URL keeps its query, negative cents, Set-Cookie domains limited to edeka.shop, typed exit codes (edekaErr, ErrCheckoutBlocked), auth import rejects an invalid --market, DevTools websocket must be loopback, scanner errors surfaced, session save merges concurrent runs under the lock. New tests: TestReviewFixes, TestSaveMergedKeepsOtherRunsCookies, TestParseTotal.
- Template-shape retro candidates: none.
- Out-of-scope retro candidates: none (two generated cliutil tests fail on Windows only; skipped with a patch record).
- Surface-to-user findings: none.
- Convergence: findings cleared at round 1 (go vet and go test green after the fixes; a second full review round was not run, the fixes were each re-checked by tests or by reading).
- /simplify: not run (scope is small and the fixes are local).
