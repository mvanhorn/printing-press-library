# Phase 4.95 Local Code Review — isitagentready-pp-cli

Review path: `ce-correctness-reviewer` subagent over the hand-written files
(internal/store/*, internal/cli/{scanhelpers,check,report,advice,guide,gate,open_advice,history,diff,compare,batch}.go).
Out of scope (per phase rules): internal/cliutil, internal/mcp/cobratree, generated files.

## Autofix summary (2 warnings fixed in-place, 0 errors)
- `internal/store/store.go`: `Load()` read the JSONL store with no lock while `compare`'s
  fan-out could `Append()` concurrently → switched `appendMu sync.Mutex` to `storeMu sync.RWMutex`;
  `Append` takes the write lock, `Load` takes the read lock.
- `internal/cli/compare.go`: same URL passed twice double-scanned + double-persisted in auto mode →
  added `dedupURLs` (by `store.NormalizeURL`) before the fan-out; added `TestDedupURLs`.
- Verified: `go build`, `go vet`, and `go test -race ./internal/store ./internal/cli` all green.

No error-severity findings. The reviewer traced every hand-written file (commands, pure decision
functions, error-code mapping, boundCtx coverage, verify/dogfood safety) and found the rest clean:
dry-run guards precede all filesystem/store/network reads, guide's net/http is verify-guarded +
boundCtx-bounded with a deferred body close, EvaluateGate guards nil prev, exit codes map correctly.

## Residual risks (accepted / by-design — not fixed)
- Under `--agent` (compact), list output drops the `description` field (it is in the shared
  `compactVerboseListFields`); the load-bearing `prompt`/`skillUrl`/`check` survive at all row counts,
  and `description` is recoverable via `--select`. Shared-pipeline behavior, not a logic bug.
- `Load` skips unparseable JSONL lines and `persistScan` downgrades a write error to a stderr warning:
  a malformed/unwritable store silently yields fewer records (documented intent).
- Human-output `truncate` slices by byte offset (could split a multibyte rune mid-display); display-only,
  never affects JSON/--agent output or decision logic.
- `EvaluateGate` treats a corrupt `prev` record as "no baseline" (disables --no-regress for that run)
  rather than erroring; `current` parse errors still fail loudly.

## Out-of-scope / retro candidates (generated template code — not patched here)
- Unused generated partial-failure machinery (`allowPartialFailure` flag, `detectPartialFailure`,
  `partialFailureErr`) in generated `root.go`/`helpers.go`. Template-shape dead code emitted for
  batch-mutating APIs; this API has none. Drives scorecard `dead_code` 1/5. Retro candidate; Polish
  (5.5) may strip it from the printed CLI.

## Simplification
- Standalone `/simplify` deferred to Phase 5.5 Polish, which performs dead-code removal and
  simplification over the same in-scope code as part of its diagnostic loop.
