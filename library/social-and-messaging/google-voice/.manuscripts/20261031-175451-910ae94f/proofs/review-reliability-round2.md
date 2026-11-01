# Reliability and maintainability review — round 2

Target: `working/google-voice-pp-cli`. Re-reviewed the full original custom reliability scope: `internal/voice/` (including new `sync.go`), custom Voice CLI commands, command wiring, transport flags, query bounds/order/NULL handling, page transactions, run terminalization, and preservation hooks. Review path: direct reviewer subagent dispatch under Printing Press phase 17.

Outcome: one new actionable custom-code finding. The four findings from this reviewer's round 1 are addressed. No target source or ledger changes, real session/message/HAR reads, remote requests, or external messages were performed. Synthetic checks ran only in the separate `working/review-reliability-synthetic-round2` source copy and private test directories.

## Finding

### R2-1 — [P2] Stop restarting exhausted inventory within the same collection run

Location: `internal/voice/sync.go:54`–60 (`nextJob`), invoked repeatedly at lines 137–161.

Every requested folder remains eligible for inventory work after it returns an empty continuation cursor. `nextJob` resets its cursor to `""` and immediately queues another complete scan, including when the folder already exhausted during this same run. Because at least one folder is required, `nextJob` can never return `ok=false`; collection always spends the entire maximum page budget or times out. Static threads are also repeatedly re-marked for refresh after they have already refreshed during the run.

This wastes requests, can leave an otherwise completed thread pending again depending on the budget, and can fail a run after all intended work already completed. With the normal one-request-per-second pacing, a large allowed page budget can hit the overall timeout even for an empty account.

Synthetic reproductions:

- `TestRound2ExhaustedFolderRepeats`: an empty `texts` folder returns `next_cursor=""` on every response. A single sync with `maxPages=5` lists the identical empty inventory five times, committing five pages before returning `observed_exhausted`.
- `TestRound2ExhaustedFolderCanFailAfterCompletion`: the first list exhausts the empty folder; the unnecessary second list returns a synthetic transport failure. Sync fails with one committed page and saved status `failed`, despite having already exhausted its only selected inventory with zero pending threads.

Small fix: pass the current run ID into work selection and omit completed folders once their latest inventory observation belongs to that run. Continue unfinished inventory cursors and thread jobs, and allow the loop to stop when that observation pass and pending thread work are done. Preserve cross-run ordering so one-page resumptions remain fair; a subsequent invocation may start a new observation pass.

## Validation

Executed in the synthetic source copy with dependency network access disabled:

```text
GOPROXY=off GOTOOLCHAIN=local go test ./internal/voice -run 'TestRound2|TestFairBudgetAndRefreshGap|TestSyncLockCancellationAndAccountSingleton|TestHalfOpenSubmillisecondBounds|TestGatewayIdentityUnknownReadAndRateStatus' -v -count=1
```

Result: PASS (1.296 seconds). Both new proof tests passed by asserting the current incorrect behavior. The existing fair-budget/gap, lock/account-singleton, submillisecond-bound/context-observation, and Gateway-status/read-state regression checks also passed. Proof source: `working/review-reliability-synthetic-round2/internal/voice/review_round2_repro_test.go`.

Round 1 verification: independent refresh-gap/deep-history state preserves both resumable tasks; selected-folder exhaustion includes unvisited folders; exclusive sync locking precedes run allocation and interrupted-run terminalization; Gateway now supplies HTTP 429 plus Retry-After to the outer client's existing policy. No additional findings identified in cancellation/finalization, query ordering/NULL safety, supported output/TLS/timeout/rate-limit flags, or regeneration-preserved registry wiring.

Template-shape and reserved-path retro candidates: none identified by this reviewer. This review does not count the merged reviewer round as converged while R2-1 remains outstanding.
