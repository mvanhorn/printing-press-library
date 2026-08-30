# Phase 4.85 (Agentic Output Review): SKIPPED, then partially superseded

Phase 4.85 reviews the shape and usefulness of real command output, which requires capturing that output from live calls. It was skipped when the run reached it because no locally built Go binary on this generation host could complete an outbound connection. That block was later identified and lifted, and Phase 5's live dogfood matrix subsequently ran green against the real DSM host, so the concern this phase exists to catch is now covered by evidence, even though the phase itself was never re-run.

## The block, and its root cause

The generated CLI and a minimal reproducer (`net.DialTimeout("tcp", addr, 8*time.Second)`) both hung indefinitely against the DSM host and against public addresses, sitting at 0% CPU well past their own timeout, which means the socket was being held rather than refused. `curl` reached the same NAS endpoint with HTTP 200 in 52ms.

The cause was per-application filtering by ESET Security, the active security product on this host: Windows Defender reported `RealTimeProtectionEnabled: False` while `ekrn`, `efwd` and `ekrnEpfw` ("ESET Firewall Helper") all ran. The discriminator was code signing, not the destination: `gh`, a signed Go binary, completed an internet request in under a second, while the unsigned locally built binaries hung. A per-application firewall awaiting an interactive allow decision that never surfaces in a non-interactive session explains every observation, including why a hard dial timeout never fired. Disabling the ESET firewall restored outbound connectivity for freshly built binaries immediately, which confirmed the diagnosis.

Two earlier explanations were tested and refuted, and are recorded here so they are not retried:

- **ProtonVPN default route.** Refuted. With ProtonVPN off, `Get-NetRoute` showed a single default route via the LAN gateway and the behaviour was unchanged.
- **ProtonVPN split-tunnel WFP filters.** Four `ProtonVPN Split Tunnel redirect app` filters were active while the `ProtonVPNCallout` driver was stopped, which looked like a blackhole. Stopping `ProtonVPN Service` removed all four (verified: zero remaining), and the behaviour was still unchanged.

The block was environmental, not a defect in the CLI.

## What was verified

- Every command's help, flag wiring, exit code and dry-run request shape, through `verify` (32/32) and the shipcheck dogfood leg.
- The dry-run request preview of `session login`, by hand, which is how the Phase 4.95 password-redaction fix was confirmed.
- After the block was lifted: the full Phase 5 live dogfood matrix against a live DS415+, 192 passed and 0 failed, which exercises real response payloads for every non-mutating command.

## What remains unverified

Phase 4.85's own editorial judgement on output shape was never re-run, so the README recipes' `--select` paths were not re-read against live payloads command by command. The live matrix proves the payloads parse and the commands succeed; it does not prove every documented `--select` path is the most useful projection.
