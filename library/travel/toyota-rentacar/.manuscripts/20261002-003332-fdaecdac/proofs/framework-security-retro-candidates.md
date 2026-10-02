# Generated framework static-analysis candidates

The pinned gosec scan reports no findings in hand-authored Toyota code. These findings are in emitted framework/reserved helpers and are retained for Printing Press template review, per the polish ownership boundary. No product-owned exception was accepted.

- `internal/client/client.go:454` — HIGH G119: Sensitive headers should not be re-added in redirect policy callbacks. Generated template/reserved helper; not patched in this printed CLI.
- `internal/platform/gate.go:22` — HIGH G101: Potential hardcoded credentials. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cli/teach.go:210` — HIGH G703: Path traversal via taint analysis. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:3450-3453` — MEDIUM G201: SQL string formatting. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:3147-3150` — MEDIUM G201: SQL string formatting. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:1191` — MEDIUM G201: SQL string formatting. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:1167` — MEDIUM G201: SQL string formatting. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/learnings.go:609` — MEDIUM G202: SQL string concatenation. Generated template/reserved helper; not patched in this printed CLI.
- `internal/platform/migration.go:156` — MEDIUM G202: SQL string concatenation. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:253` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/platform/ratelimit.go:148` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/platform/profile.go:510` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/learn/teach_log.go:112` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/learn/playbooks.go:73` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/config/config.go:104` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cli/teach_playbook.go:376` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cli/teach.go:150` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cli/teach.go:127` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cli/feedback.go:204` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cli/feedback.go:66` — MEDIUM G304: Potential file inclusion via variable. Generated template/reserved helper; not patched in this printed CLI.
- `internal/learn/journal.go:265` — MEDIUM G117: Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern. Generated template/reserved helper; not patched in this printed CLI.
- `internal/platform/profile.go:302` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cliutil/testenv/testenv.go:76` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template/reserved helper; not patched in this printed CLI.
- `internal/cliutil/testenv/sandbox_unix.go:15` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template/reserved helper; not patched in this printed CLI.
- `internal/client/client.go:800` — MEDIUM G302: Expect file permissions to be 0600 or less. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:3238` — LOW G104: Errors unhandled. Generated template/reserved helper; not patched in this printed CLI.
- `internal/store/store.go:2981` — LOW G104: Errors unhandled. Generated template/reserved helper; not patched in this printed CLI.
- `internal/learn/journal.go:278` — LOW G104: Errors unhandled. Generated template/reserved helper; not patched in this printed CLI.
- `internal/client/client.go:1504` — LOW G104: Errors unhandled. Generated template/reserved helper; not patched in this printed CLI.
