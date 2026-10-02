# Local review findings

Seven substantive findings autofixed in place across two verification rounds; final documentation wording cleared in round three. Source and independently verified checks are recorded in hello-cycling-independent-review.md. This project was created locally without commits; source hashes and the binary-owned acceptance fingerprint identify the reviewed source.

Review path: direct subagent dispatch using exactly one user-authorized fresh-context gpt-6.1-sol MAX reviewer for correctness, security, API contracts, performance and output plausibility. No extra reviewer agents were used.

## Template and reserved-package retro candidates

Gosec 2.26.1 scanned 106 files / 34,504 lines and reported 38 generated findings; zero findings remain in hand-written HELLO CYCLING code. The raw generated io.ReadAll shape is bounded by the preserved standard-HTTP client hook for CLI and MCP metadata requests. Generated chrome cookie/login utilities are outside the anonymous domain workflow. SQL table-name construction and local configuration paths below belong to the Press framework. These findings are retained for an upstream template review; a clean static-security score is not claimed.

- `internal/client/chrome.go:149` — HIGH G402: TLS InsecureSkipVerify may be set to true.. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/client.go:437` — HIGH G119: Sensitive headers should not be re-added in redirect policy callbacks. Generated template surface; recorded rather than changed in the printed tree.
- `internal/platform/gate.go:22` — HIGH G101: Potential hardcoded credentials. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cli/teach.go:210` — HIGH G703: Path traversal via taint analysis. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:3558-3561` — MEDIUM G201: SQL string formatting. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:3255-3258` — MEDIUM G201: SQL string formatting. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:1204` — MEDIUM G201: SQL string formatting. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:1180` — MEDIUM G201: SQL string formatting. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/learnings.go:609` — MEDIUM G202: SQL string concatenation. Generated template surface; recorded rather than changed in the printed tree.
- `internal/platform/migration.go:156` — MEDIUM G202: SQL string concatenation. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:253` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/platform/ratelimit.go:148` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/platform/profile.go:510` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/learn/teach_log.go:112` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/learn/playbooks.go:73` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/config/config.go:108` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cli/teach_playbook.go:376` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cli/teach.go:150` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cli/teach.go:127` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cli/feedback.go:204` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cli/feedback.go:66` — MEDIUM G304: Potential file inclusion via variable. Generated template surface; recorded rather than changed in the printed tree.
- `internal/learn/journal.go:265` — MEDIUM G117: Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern. Generated template surface; recorded rather than changed in the printed tree.
- `internal/platform/profile.go:302` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cliutil/testenv/testenv.go:76` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template surface; recorded rather than changed in the printed tree.
- `internal/cliutil/testenv/sandbox_unix.go:15` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/client.go:780` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:3346` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/store/store.go:3089` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/learn/journal.go:278` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/client.go:1430` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:204` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:158` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:151` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:146` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:111` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:107` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:97` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.
- `internal/client/chrome.go:80` — LOW G104: Errors unhandled. Generated template surface; recorded rather than changed in the printed tree.

Additional Press analyzer candidates: the static pricing-show/profile-show leaf collision misclassifies pricing arguments; actual exact leaf help and Tokyo live workflow pass. Static helpers are recognized reliably only when constructors and common flags are literal in their command files. Dogfood sync can reinsert an unsupported universal uniqueness sentence; it was removed after final sync and rechecked. The initial scorecard live-check classified a missing-baseline changes error as graceful empty; that sample was excluded from output review, and actual successful changes output plus full live matrix establish the feature.

## Surface-to-user findings

None remain. Count snapshots, geometric distances and generic GBFS class/model limits are disclosed in README/SKILL and every relevant output.

Convergence: substantive implementation findings cleared in round two; final documentation-only warning cleared in round three by the same reviewer.
