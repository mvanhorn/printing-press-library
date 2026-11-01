# Google Voice correctness + API-contract review — final round 3

**Result: clear. No remaining actionable correctness/API-contract findings in this reviewer's scope.**

Re-reviewed the complete current custom implementation rather than only the prior finding locations: `internal/voice/{codec,remote,session,mirror,history,sync}.go`, new Unix/Windows privacy guards, `internal/cli/voice_*.go`, relevant command/MCP wiring, original tests, and `review_regressions_test.go`. The approved absorb manifest and checked-out upstream protocol remain the behavioral reference. No production source or ledger changed; no real session, message snapshot, HAR or remote service was accessed.

All ten round 1 findings and the round 2 Gateway count finding are addressed. The final Gateway distinguishes omitted count from explicit numeric/integer values outside 1–100 and rejects invalid input before credentials load. The scheduler now skips a folder already exhausted in the same run while preserving fairness and resumability across runs. I also checked the merged storage-opening changes: private leaf/trusted ancestor validation, pathname identity checks after lock waiting and before lazy database work, account ownership validation and separate sync/schema locks.

The runtime fixture hook preserves existing fixture arguments and uses the existing `--home` and `--no-learn` paths for test invocations. It does not add an authentication mechanism or embed private fixture values in source.

Verification on the final merged source:

- `go test ./internal/voice ./internal/cli ./internal/mcp` — pass.
- `go vet ./internal/voice ./internal/cli ./internal/mcp` — pass.
- Additional round 2 overlay tests rerun unchanged — all pass: 250 initial messages collected via repeated one-page runs, then 225 newer records correctly bridged after earlier history exhaustion (475 distinct IDs); concurrent inventory re-dirtying preserves deep-history and refresh-gap state; explicit counts 0 and 1.5 are now rejected.
- The current regressions include same-run exhausted-inventory termination, malformed positional schemas, account verification, rate-limit metadata, unknown unread handling, fractional duration/transcript/account-version fidelity, half-open sub-millisecond boundaries, independent observation timestamps, sync-lock cancellation, singleton account binding and inherited/foreign writable ACL rejection.

Arrivals' open-ended continuation remains the previously documented nonblocking observation; no missing or duplicate record path was established. The Windows storage path deliberately fails closed and the shipping target remains the private Mac build; no supported target was silently weakened during this review.

No generator-template or reserved-package finding is added. `internal/cliutil/` and `internal/mcp/cobratree/` remain excluded from fixes.

**Correctness/API-contract findings cleared at round 3.** Root should determine overall Phase 17 convergence from the merged final reports of all reviewers.
