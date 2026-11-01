# Reliability and maintainability review — round 3

Outcome: **clear — no actionable findings remain in this reviewer's scope.** Review path: direct reviewer subagent dispatch under Printing Press phase 17. Merged convergence still requires all three final reviewer reports to be clear.

Re-reviewed the full original custom scope in `working/google-voice-pp-cli`: `internal/voice/` (codec, remote/Gateway, session publication, privacy guards, mirror schema/page transactions, sync scheduling/locking, history queries), `internal/cli/voice_core.go`, `internal/cli/voice_history.go`, preserved novel-command wrappers and root/client-hook wiring.

The round 2 scheduler finding is resolved: work selection receives the current run ID, skips a folder once its inventory exhausted in that run, continues unfinished inventory, and permits a fresh observation in a subsequent run. The regression fixture verifies an empty folder with a five-page maximum makes one list call, commits one page, and returns `observed_exhausted`, even when an unnecessary second request would fail.

The earlier fixes remain intact: independent refresh-gap/deep-history checkpoints and anchors, fair work ordering across bounded resumptions, selected-folder exhaustion accounting, process-safe cancellable sync serialization before run allocation, terminalization of abandoned runs, singleton account binding, per-Remote account verification, and HTTP 429/Retry-After translation. Rechecked query ordering, NULL handling, submillisecond half-open bounds, truthful context observation timestamps, cancellation/finalization, transport timeout/TLS/rate-limit handling, output-flag routing, and regeneration-preserved extension wiring without finding additional defects.

Also reviewed the final changes: explicit Gateway counts must be integers from 1 through 100 before credential IO; private storage validates the leaf and canonical directory ancestors and rechecks database identity after the schema lock; session publication validates its directory; operator-supplied fixture annotations add happy-path arguments without replacing normal authentication resolution.

Validation executed with dependency network access disabled:

```text
GOPROXY=off GOTOOLCHAIN=local go test ./internal/voice -count=1
ok google-voice-pp-cli/internal/voice 8.293s

GOPROXY=off GOTOOLCHAIN=local go test ./internal/cli -run '^TestVoice' -count=1
ok google-voice-pp-cli/internal/cli 0.135s
```

These tests use synthetic responses/credentials and private temporary files. No target source or ledger edits, real session/message/HAR reads, remote requests, or external messages were performed. This report is the only new review artifact from this pass.

Template-shape and reserved-path retro candidates: none identified by this reviewer. Reliability/maintainability findings cleared at round 3.
