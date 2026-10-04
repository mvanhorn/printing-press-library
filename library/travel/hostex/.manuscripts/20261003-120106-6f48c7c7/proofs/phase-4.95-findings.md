# Phase 4.95 local code review (self-review; nested agents unavailable in the worker context)

Scope: client.go envelope hook, hostex_envelope.go, sync.go/store.go patches, which.go/which_writes.go, revenue_rollup.go, hand-authored novel commands.
- go vet clean; go test ./... passes (internal/cli, client, store, mcp, platform, cliutil, config, learn).
- Native timeout-boundary check: the three novel commands that make live requests (price-parity, oversell-watch, revenue-rollup) call boundCtx and use flags.newClient(); the others read the local mirror.
- Fixed in place: 5xx error_code on a write was retried unconditionally in the old envelope hook; the re-applied hook now respects the ambiguous-write gate (canRetryAmbiguousFailure). Regression tests added for the envelope hook and the sync params.
- No findings in generator-reserved paths (internal/cliutil, internal/mcp/cobratree).
