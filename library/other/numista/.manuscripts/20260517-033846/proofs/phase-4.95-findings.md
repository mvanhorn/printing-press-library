# Phase 4.95 — Native Code Review Findings

## Harness exemption
Claude Code's `/review` skill is GitHub-PR-oriented and doesn't accept a directory path target. Per the Phase 4.95 harness-exemption clause, the review was conducted inline by reading the hand-written source files. This document records what was checked, what was found, and what was fixed.

## Scope reviewed (~1,900 lines hand-written)
| File | LoC | Notes |
|---|---|---|
| `internal/cliutil/quota.go` | 80 | NEW — monthly variant adapted from PCGS daily |
| `internal/cli/audit.go` | 170 | NEW — lookup_log SQL views |
| `internal/cli/types_batch.go` | 366 | NEW — CSV/JSONL/text parse, checkpoint resumability, dry-run forecast |
| `internal/cli/types_series.go` | 164 | NEW — issues + prices fan-out for one type |
| `internal/cli/refresh.go` | 216 | NEW — selective field refresh |
| `internal/cli/crawl.go` + `crawl_issuer.go` | 212 | NEW — issuer-scoped page-through with forecast |
| `internal/cli/collection.go` + `collection_value.go` | 255 | NEW — quota-guarded collection valuation |
| `internal/cli/watchlist*.go` (5 files) | 291 | NEW — add/list/remove/check + snapshot diff |
| `internal/cli/users_collections_hydrate.go` | 114 | NEW — folder hydrate with price fan-out |
| `internal/cli/users_collected-items_add.go` | +169 | EDIT — `--from-file` extension for CSV/JSONL bulk import |
| `internal/store/store.go` | +143 | EDIT — `lookup_log`, `watchlist`, `price_snapshots` tables + helpers |
| `internal/client/client.go` | +98 | EDIT — LookupEvent type + LogHook insertion points |
| `internal/cli/root.go` | +99 | EDIT — `--quota`/`--quota-only` flags + LogHook installer + post-run quota line |

## Checklist applied
- [x] SQL parameterization (no string-concat queries in hand-written code)
- [x] Input validation (CSV/JSONL row parsing rejects malformed inputs with line numbers)
- [x] Error wrapping (`fmt.Errorf("xxx: %w", err)` throughout)
- [x] Credential handling (no `NUMISTA_API_KEY` references in hand code; auth header set entirely by generator-emitted `client.go`)
- [x] File-mode bits (checkpoint files `0o644` — public type-ID lists, no sensitive data; credential paths via generator's `0o600`/`0o700`)
- [x] Concurrency (store helpers acquire `writeMu` for INSERT/UPDATE; reads use QueryContext)
- [x] Resource cleanup (every `Open` paired with `defer Close`; transactions paired with `defer Rollback`)
- [x] Typed error surfacing (HTTP 429 path in `types_batch` uses `errors.As(err, &client.APIError)` and emits a friendly "quota exhausted; M IDs remain" message)
- [x] Annotation correctness (read-only commands carry `"mcp:read-only": "true"`; mutating commands omit it)
- [x] RunE shape (every command honors `dryRunOK(flags)` and `cliutil.IsVerifyEnv()` where applicable; no `cobra.MinimumNArgs` or `MarkFlagRequired` that would block verify probes)

## Findings autofixed in place

### 1. Silent cache-write failure in `users collections hydrate`
**File**: `internal/cli/users_collections_hydrate.go:93`
**Severity**: low
**Before**:
```go
_ = s.Upsert("issue_prices", key+"."+currency, priceData)
pricesFetched++
```
**Issue**: The bare `_ =` swallowed any store-write error, AND `pricesFetched` was incremented unconditionally — so the final summary count claimed N prices fetched even when caching failed for some of them. Two small problems composing into a single "the summary may lie" symptom.
**Fix applied**: surface the failure to stderr, increment only on success, mechanical and behavior-preserving.
```go
if err := s.Upsert("issue_prices", key+"."+currency, priceData); err != nil {
    fmt.Fprintf(os.Stderr, "warning: cache write for %s failed: %v\n", key, err)
    continue
}
pricesFetched++
```
Build + vet green after fix.

## Findings routed to retro (machine-level, not patched in printed CLI)

### R1. DB-open-per-call inefficiency in `types_batch`
**File**: `internal/cli/types_batch.go:297` (`quotaTrackedGet`)
**Issue**: `quotaTrackedGet` opens the store twice (once before the call, once after) to infer cache-hit via a quota-count delta. For a 2000-ID batch this opens the store 4001 times. Not a correctness issue, but the equivalent PCGS pattern is more efficient (it uses the LogHook return data directly to detect cache hits, never re-reading the quota inside the loop).
**Recommendation for retro**: emit a `withStore(flags, func(*store.Store) error) error` helper for novel-command authors. Or have novel templates reuse the LogHook return signal so the count is free.

### R2. `os.Exit(0)` bypasses `defer s.Close()` in `root.go` PersistentPreRunE
**File**: `internal/cli/root.go` (the `--quota` / `--quota-only` short-circuit)
**Issue**: When `--quota` is passed alone, the handler opens the store with `defer s.Close()` and then calls `os.Exit(0)`. `os.Exit` does NOT run deferred functions, so the SQLite handle isn't explicitly closed. The OS reaps the file descriptor on process exit, so this is benign in practice — but it is verbatim the PCGS reference pattern, which means the same minor leak exists there too.
**Recommendation for retro**: either replace `os.Exit(0)` with `s.Close(); os.Exit(0)`, or convert to a `quotaShortCircuitDone` sentinel error that the cobra run loop catches and exits cleanly so defers run.

## Out-of-scope (skipped)
- `internal/cliutil/` (`fanout.go`, `ratelimit.go`, `text.go`, `extractnumber.go`, `verifyenv.go`, `probe.go`) — generator-emitted, untouched by Phase 3 except imports
- `internal/mcp/cobratree/` — generator-emitted
- `internal/store/store.go` migration framework (only the `lookup_log` / `watchlist` / `price_snapshots` additions were reviewed; the rest is generator-emitted)

## Recommendation
**Proceed to Phase 5 (live dogfood).** One mechanical fix applied; two retro candidates filed. No blockers identified. The shipcheck verdict (PASS 6/6, Scorecard 85/100 Grade A) stands.
