# Phase 4.95 Local Code Review — Findings

## Autofix summary
3 findings autofixed in-place across 1 round (build/vet/test green after).

- forkable_csrf.go: CSRF token cache was process-global via sync.Once — changed to a per-Client cache (package map keyed by *Client + per-cache mutex) so distinct sessions/base URLs don't share a token.
- forkable_csrf.go: sync.Once cached a failed fetch permanently, disabling CSRF for the process on any transient failure — now caches only on success and retries on the next call.
- why_picked.go: `chosen` map was populated from all diners' pieces when --user defaulted to 0, mismarking other users' items as "chosen" for the score-resolved user — now filters pieces by the resolved userID.

## Non-blocking (accepted)
- why_picked.go resolveItemNames silently continues on per-menu name-resolution failure — acceptable, item names are cosmetic; scores/ids still returned.
- menus(ids: %d) passes a single scalar per call (one query per menu id); schema accepts it. Confirmed no injection (int64 via %d only).

## Security: PASS
No credential/token/cookie value logged or written to output. All GraphQL query interpolation uses int64 via %d (why_picked delivery/user/menu ids); no string flags reach query text.

## Convergence: findings cleared at round 1.
## Review path: direct subagent dispatch (correctness + security personas).
