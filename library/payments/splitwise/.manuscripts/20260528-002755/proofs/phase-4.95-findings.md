# Phase 4.95 Local Code Review — splitwise reprint

Scope: the 2 new codex-written files (splitwise_split.go, splitwise_recurring.go). The 6 ported files are verbatim copies of prior live-validated code; reviewed-by-port.

## Reviewed directly (in-scope, internal/cli/)
- splitwise_split.go: PASS. Cents-based allocation (no float drift); equal/exact/percent/shares each sum exactly via largest-remainder; exactly-one-mode validation; payer membership check; --record gated print-by-default → IsVerifyEnv short-circuit → client call; verify-friendly RunE. No security/correctness issues.
- splitwise_recurring.go: PASS. mcp:read-only + pp:data-source local; filters payment/deleted; sync hints called; cadence/overdue math sound; empty-collection. Trivial non-blocking nit: mostCommonString has no deterministic tie-break (cosmetic label/currency display only) — not fixed, not worth churn.

## Autofix summary
0 findings required autofix. New code clean on first review.

## Retro candidates (out of scope — machine)
- v4.19.0 emits Novel* novel-feature stubs whose names/files (balances.go/newNovelBalancesCmd) collide with hand-authored impls on reprint; forced delete-stubs + rewire-root.go. Reprint friction worth a generator hook. (retro)
- resource_type still keyed by endpoint name (get-friends etc.); dual-form workaround still needed; related to unfixed #2327. (retro)
