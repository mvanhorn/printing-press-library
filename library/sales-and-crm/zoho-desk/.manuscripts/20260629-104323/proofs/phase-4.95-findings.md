# Phase 4.95 Local Code Review — zoho-desk-pp-cli

Reviewer: subagent over in-scope hand-written files only.

## Autofixed in-place (2)
- HIGH doctor.go: org_id block dereferenced cfg without nil guard → panic when config.Load returns (nil,err). FIXED: wrapped in `if cfg != nil`. Verified: doctor no longer panics on bad config.
- LOW rebalance.go --apply: bailed on first PATCH error, hiding movesApplied/partial progress. FIXED: accumulate per-move failures, always print view with movesApplied + failures[] + stderr warning.

## Accepted / not fixed (1)
- LOW config.go: env-sourced ZOHO_DESK_ORG_ID can persist into on-disk Headers on a later save(). Accepted: orgId is not a secret, env wins on every load so behavior stays correct, and the fix touches the generator-owned persisted() path. Documented, not patched.

## Passed
SQL injection (parameterized/constant), nil/div-by-zero guards, time-fallback correctness, []-not-null marshaling, rebalance write-gating + dogfood cap, resource leaks (Close deferred), orgId env-wins + nil-init.
