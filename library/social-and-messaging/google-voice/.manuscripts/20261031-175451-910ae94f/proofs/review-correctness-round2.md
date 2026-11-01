# Google Voice correctness + API-contract review — round 2

Scope: the whole current `internal/voice/{codec,remote,session,mirror,history,sync}.go`, `internal/cli/voice_*.go`, wiring and tests, including `review_regressions_test.go`. Reviewed against the approved absorb manifest and upstream positional protocol. No production source or ledger edits; no real sessions/messages/HARs inspected and no remote calls.

Round 1 disposition: all ten required findings are addressed in current source. The shared scheduler, independent refresh-gap/deep-history checkpoints, account verification, singleton-account constraint, cancellable sync lock, strict populated-field decoding, metadata fidelity, half-open bounds, unread handling and separate context observation axes were re-reviewed together.

## Required custom finding

### [P2] Explicit invalid MCP message counts are silently expanded to 100

- **Location:** `internal/voice/remote.go:245–248` (Gateway `api2thread/get` branch).
- **API evidence:** `internal/mcp/tools.go:59` exposes `count` as a number described as 1 through 100 without integer/range schema constraints. The CLI validates its integer count at `internal/cli/voice_core.go:268–269`.
- **Reproduction:** Synthetic `TestRound2GatewayInvalidExplicitCount` supplies each of `{"thread_id":"t","count":0}` and `{"thread_id":"t","count":1.5}` to the Gateway with a private synthetic profile and fake HTTP transport. Both succeed and send the upstream body with a requested count of **100**. `num` discards integer-conversion errors; either invalid value becomes zero, which is then treated as omission.
- **Impact:** The MCP tool accepts values the CLI rejects and expands a caller's explicit bound to the maximum page size. Fractional values are valid according to the current number-shaped MCP schema, so host schema checking does not prevent this path.
- **Minimal fix:** Distinguish an absent `count` key from a present value. Only absence defaults to 100; for a supplied count, retain the integer-conversion error and require 1 through 100 before `Remote.Get`. If revising generated tool schema, update its source specification/generator path rather than hand-patching generated output. The necessary runtime fix is in the custom Gateway and requires no reserved-package edit.
- **Classification:** Custom Voice adapter bug; not a template or reserved-package finding.

## Verification

- `go test ./internal/voice ./internal/cli ./internal/mcp` passes.
- Additional overlay-only tests pass for 250 initial messages collected through repeated one-page runs, followed by 225 newer messages after prior history exhaustion: all 475 distinct IDs are present and refresh/deep cursors converge.
- A separate overlay test passes when inventory re-dirties a thread while its refresh gap is pending: completing the gap preserves both the deep cursor and latest-refresh work.
- The invalid-count test fails as expected with `count=0 -> upstream count=100` and `count=1.5 -> upstream count=100`.
- Sanitized overlay source: `proofs/review-correctness-round2-synthetic-test.go.txt`. Reproduce from the target using `go test -overlay /private/tmp/gv-review-correctness-r2/overlay.json ./internal/voice -run TestRound2 -v`.

Arrivals' open-ended continuation remains the same nonblocking observation from round 1; no omission/duplication was established. No additional source-data-loss issue was found in the revised scheduler. No generator-template or reserved-package fix is proposed. Return the whole merged implementation for round 3 after the custom count validation is corrected.
