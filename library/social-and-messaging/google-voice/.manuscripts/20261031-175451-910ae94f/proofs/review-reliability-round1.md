# Reliability and maintainability review — round 1

Target: `working/google-voice-pp-cli`, focusing on custom `internal/voice/*.go`, `internal/cli/voice_*.go`, and command wiring. Review path: direct reviewer subagent dispatch under Printing Press phase 17. This report does not mark the merged review round converged.

Four actionable findings remain. All four are custom-code issues, not generator-template findings. No target source was edited, and no real session, message, HAR, remote service, or external messaging surface was accessed. Reproduction source is in the separate `working/review-reliability-synthetic-round1` copy. Data and credentials in those tests are synthetic; temporary SQLite/session files use private test directories.

## Findings

### R1 — [P1] Refresh can permanently skip a gap while reporting exhausted history

Location: `internal/voice/mirror.go:188` (refresh checkpoint update), with scheduling at lines 273–285.

A completed thread retains `history_complete=1` when a later latest-page refresh returns a nonempty older cursor. The update only saves a refresh cursor when `history_complete=0`, and only changes completeness when the returned cursor is empty. The scheduler then excludes the thread entirely. If more than the 100-message refresh limit arrived since the previous sync, messages between the previous local history and the latest 100 are never requested, but the run reports `observed_exhausted` and zero pending threads.

Reproduction: `TestReviewReproRefreshLosesGap` initially syncs message 0 to exhaustion, then serves 101 new messages. Inventory returns the latest 15; thread refresh returns messages 2–101 and a `gap` cursor leading to messages 0–1. Sync saves only 101 of the 102 source IDs, never follows `gap`, and reports `observed_exhausted`, `pending=0`.

Small fix: make a nonempty cursor from a new refresh reopen pending work, and drain that new cursor until it overlaps an already observed ID or exhausts before claiming completeness. Do not overwrite an unrelated ongoing backfill checkpoint; preserve both work items if an older backfill is already pending. At minimum, a fresh nonempty cursor must not be discarded merely because an earlier history observation exhausted.

### R2 — [P2] The inventory-first scheduler starves valid small budgets and overstates requested-folder exhaustion

Location: `internal/voice/mirror.go:243`–269; completeness calculation at lines 303–307.

Every run processes the supplied folders in the same order before it attempts any thread work. Thus `--max-pages 1` always revisits the first inventory folder and never downloads older thread history; `--folders texts,archived --max-pages 1` never reaches archived at all. A two-page budget with two folders also never reaches thread work. This contradicts the documented instruction to rerun bounded sync to resume saved work. The completeness query counts only existing rows, so a requested second folder that was never visited is also reported as inventory exhausted.

Reproduction: `TestReviewReproSmallBudgetStarvation` repeats the valid two-folder, one-page sync four times. It makes zero thread requests and zero archived inventory requests; every result nevertheless reports `inventory_exhausted=true` while leaving the same pending thread.

Small fix: persist a rotating work position or select the least recently worked inventory/thread task so every valid page budget makes eventual progress. Include all requested folders, including missing rows, when calculating this result's inventory completeness.

### R3 — [P2] Page transactions permit concurrent syncs to regress committed cursors

Location: `internal/voice/mirror.go:198` (unconditional inventory checkpoint upsert); similarly thread updates at lines 188 and 192; work selection begins at line 249.

Sync reads a checkpoint, makes a network request, and later writes its result without an exclusive sync lock or checking that the saved input checkpoint is still current. Atomic page commits prevent partial pages, but do not prevent a slower concurrent sync from overwriting newer progress. Separate CLI/MCP invocations can therefore move a durable cursor backward and repeat already completed collection work. The same issue applies to thread checkpoint updates.

Reproduction: `TestReviewReproConcurrentCheckpointRegression` holds one synthetic sync's initial list response while two other syncs advance the saved cursor to `cursor-2`. Releasing the older request successfully overwrites the checkpoint with `cursor-1`.

Small fix: serialize collection per mirror using a process-safe exclusive lock with context-bounded acquisition, while retaining per-page commits, or validate the checkpoint/version atomically before each page commit and reject/replan stale work. A mutex confined to one `Mirror` object does not protect separate CLI processes.

### R4 — [P2] The Gateway drops HTTP 429 semantics and retries before Retry-After

Location: `internal/voice/remote.go:235`–240 (Gateway error translation).

`Remote.call` correctly converts 429 into `cliutil.RateLimitError`, but Gateway returns that as a `RoundTripper` transport error. Generated typed endpoint/MCP clients therefore enter the transient-network retry branch rather than their HTTP 429 branch. They retry after exponential 1/2/4-second delays instead of the upstream `Retry-After`, and do not adjust the outer limiter for 429. This is an adapter issue; the generated client already has the required status/header handling.

Reproduction: `TestReviewReproGatewayIgnoresRetryAfter` routes a generated read client through Gateway to a synthetic transport that always returns HTTP 429 with `Retry-After: 30`. It sends a second request after approximately 1.003 seconds. The emitted diagnostic explicitly calls the HTTP 429 a network error and schedules a one-second retry.

Small fix: translate `RateLimitError` into a sanitized HTTP 429 response carrying an accurate `Retry-After` header at the Gateway boundary, as is already done for `HTTPError`, so the outer client uses its existing rate-limit policy. Do not patch the generated client or reserved utility packages for this custom adapter failure.

## Validation and scope notes

Command executed in the synthetic source copy with dependency network access disabled:

```text
GOPROXY=off GOTOOLCHAIN=local go test ./internal/voice -run TestReviewRepro -v -count=1
```

All four reproduction tests passed by asserting the current incorrect behavior. These are proof fixtures, not claims that regression tests pass after fixes. Sanitized transcript: `proofs/review-reliability-round1-synthetic.txt`. Synthetic test source: `working/review-reliability-synthetic-round1/internal/voice/review_repro_test.go`.

Inspected without additional findings: bounded/parameterized local queries, chronological context tie-breaking, activity NULL handling, page rollback on source-ID conflict, hard failure on throttling/auth/schema errors, explicit custom-request timeout context, generated TLS transport reuse, and explicit rate-limit configuration. Custom extension wiring uses markerless registry hooks and preserved novel wrappers; no defect was identified by source inspection. Regeneration itself was not run in this review.

Global JSON/select/compact/CSV/plain/quiet output reaches the common print pipeline for normal custom command output. The explicit JSONL export branch bypasses several global formatting flags; that observation was sent to the parent for output-review ownership rather than duplicated as a reliability finding.

Template-shape or reserved-path retro candidates: none identified by this reviewer. No fixes proposed for `internal/cliutil/` or `internal/mcp/cobratree/`.
