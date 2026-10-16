# Phase 4.95 local code review
- Review path: direct subagent dispatch — correctness (opus), security (opus round 1 / sonnet round 2), maintainability (sonnet); round 3 combined convergence reviewer (opus) on the round-2 diff.
- Autofix summary: ~30 findings autofixed in-place across 3 rounds; see commits b114806 af8da40 9610661 9508740 dad2303 febfe8f .
- Round 3 found one HIGH regression introduced by round 2 (custom per-site redirect client re-sent credentials across hosts and dropped downgrade/private-address guards); fixed by restoring the generated client's CheckRedirect and sharing only the transport, with regression tests for cross-host second hop and https->http downgrade. Not re-reviewed after this fix (3-round cap); covered by tests.
- Convergence outcome: findings cleared at round 3 (final fix verified by tests).

## Template-shape / out-of-scope retro candidates
- internal/client/client.go ~1412: unbounded io.ReadAll on API responses (generator-owned).
- internal/config/config.go: RANKMATH_SITE substituted into https://{site}/wp-json without validation (userinfo/@, query, fragment); hand patch only covers the empty default. Generator should validate endpoint template values like a host.
- README/SKILL Freshness Contract lists non-existent command paths (ai_visibility get, gsc-dashboard ...).
- validate-narrative --framework-only treats hand-added doctor subcommands as framework flags (false positive).

## Surface-to-user findings
- None requiring a tradeoff decision.
- Simplify pass: applied snapshot SQL ranking/batch insert, O(1) path matching, shared helpers, seohead micro-cleanups; skipped seohead single-pass tokenizer rewrite, Fetch non-2xx semantics change, audit PostID type change, shared table printer, generic analytics decoder (behavior-changing or out of scope).
