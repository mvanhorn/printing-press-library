# Gosec triage

Pinned gosec v2.26.1 scanned 106 files / 38,945 lines and reported 30 raw findings. Each finding is in an unchanged framework file present in the initial actual Press generation, confirmed byte-identical to the retained emission. None targets hand-written Repark source/command modules. Per the polish skill these are retained as template retro candidates; generated/reserved packages were not edited to silence the scanner. Post-triage unresolved hand-written findings: 0. Raw scanner exit: 1 (findings), not an all-clean claim. Two emitted framework files lack the standard Generated header; retained-generation byte parity establishes their provenance.

| Rule | File:line | Classification |
|---|---|---|
| G119 | internal/client/client.go:454 | Unchanged emitted framework; Sensitive headers should not be re-added in redirect policy callbacks |
| G101 | internal/platform/gate.go:22 | Unchanged emitted framework; Potential hardcoded credentials |
| G703 | internal/cli/teach.go:210 | Unchanged emitted framework; Path traversal via taint analysis |
| G201 | internal/store/store.go:3610-3613 | Unchanged emitted framework; SQL string formatting |
| G201 | internal/store/store.go:3307-3310 | Unchanged emitted framework; SQL string formatting |
| G201 | internal/store/store.go:1239 | Unchanged emitted framework; SQL string formatting |
| G201 | internal/store/store.go:1215 | Unchanged emitted framework; SQL string formatting |
| G202 | internal/store/learnings.go:609 | Unchanged emitted framework; SQL string concatenation |
| G202 | internal/platform/migration.go:156 | Unchanged emitted framework; SQL string concatenation |
| G304 | internal/store/store.go:253 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/platform/ratelimit.go:148 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/platform/profile.go:510 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/learn/teach_log.go:112 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/learn/playbooks.go:73 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/config/config.go:105 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/cli/teach_playbook.go:376 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/cli/teach.go:150 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/cli/teach.go:127 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/cli/feedback.go:204 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/cli/feedback.go:66 | Unchanged emitted framework; Potential file inclusion via variable |
| G304 | internal/cli/export.go:213 | Unchanged emitted framework; Potential file inclusion via variable |
| G117 | internal/learn/journal.go:265 | Unchanged emitted framework; Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern |
| G302 | internal/platform/profile.go:302 | Unchanged emitted framework; Expect file permissions to be 0600 or less |
| G302 | internal/cliutil/testenv/testenv.go:76 | Unchanged emitted framework; Expect file permissions to be 0600 or less |
| G302 | internal/cliutil/testenv/sandbox_unix.go:15 | Unchanged emitted framework; Expect file permissions to be 0600 or less |
| G302 | internal/client/client.go:800 | Unchanged emitted framework; Expect file permissions to be 0600 or less |
| G104 | internal/store/store.go:3398 | Unchanged emitted framework; Errors unhandled |
| G104 | internal/store/store.go:3141 | Unchanged emitted framework; Errors unhandled |
| G104 | internal/learn/journal.go:278 | Unchanged emitted framework; Errors unhandled |
| G104 | internal/client/client.go:1504 | Unchanged emitted framework; Errors unhandled |
