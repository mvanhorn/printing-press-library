# Security scan triage

Scanner: gosec v2.26.1. Raw findings: 38. Unresolved new domain-code findings: 0. The scan completed and wrote JSON; it did not exit cleanly.

All issues are in the Press-emitted framework, unchanged by this build. Except the reserved testenv helper and platform migration file, they have explicit Generated/DO NOT EDIT headers. The two headerless files were emitted in the same fresh generation and are framework code, not airport integration code. They are preserved for generator review, under the skill’s generated-file escape hatch.

The airport provider always passes skipTLSVerify=false and refuses redirects. Thus the generated TLS switch and sensitive-header redirect hooks are not used by the live airport commands. No credential was supplied; the source is anonymous.

| File:line | Rule | Severity | Finding | Disposition |
|---|---|---|---|---|
| internal/client/chrome.go:193 | G402 | HIGH | TLS InsecureSkipVerify may be set to true. | Generator review; emitted framework file |
| internal/client/client.go:446 | G119 | HIGH | Sensitive headers should not be re-added in redirect policy callbacks | Generator review; emitted framework file |
| internal/platform/gate.go:22 | G101 | HIGH | Potential hardcoded credentials | Generator review; emitted framework file |
| internal/cli/teach.go:210 | G703 | HIGH | Path traversal via taint analysis | Generator review; emitted framework file |
| internal/store/store.go:3450-3453 | G201 | MEDIUM | SQL string formatting | Generator review; emitted framework file |
| internal/store/store.go:3147-3150 | G201 | MEDIUM | SQL string formatting | Generator review; emitted framework file |
| internal/store/store.go:1191 | G201 | MEDIUM | SQL string formatting | Generator review; emitted framework file |
| internal/store/store.go:1167 | G201 | MEDIUM | SQL string formatting | Generator review; emitted framework file |
| internal/store/learnings.go:609 | G202 | MEDIUM | SQL string concatenation | Generator review; emitted framework file |
| internal/platform/migration.go:156 | G202 | MEDIUM | SQL string concatenation | Generator review; emitted framework file |
| internal/store/store.go:253 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/platform/ratelimit.go:148 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/platform/profile.go:510 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/learn/teach_log.go:112 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/learn/playbooks.go:73 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/config/config.go:108 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/cli/teach_playbook.go:376 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/cli/teach.go:150 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/cli/teach.go:127 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/cli/feedback.go:204 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/cli/feedback.go:66 | G304 | MEDIUM | Potential file inclusion via variable | Generator review; emitted framework file |
| internal/learn/journal.go:265 | G117 | MEDIUM | Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern | Generator review; emitted framework file |
| internal/platform/profile.go:302 | G302 | MEDIUM | Expect file permissions to be 0600 or less | Generator review; emitted framework file |
| internal/cliutil/testenv/testenv.go:76 | G302 | MEDIUM | Expect file permissions to be 0600 or less | Generator review; emitted framework file |
| internal/cliutil/testenv/sandbox_unix.go:15 | G302 | MEDIUM | Expect file permissions to be 0600 or less | Generator review; emitted framework file |
| internal/client/client.go:792 | G302 | MEDIUM | Expect file permissions to be 0600 or less | Generator review; emitted framework file |
| internal/store/store.go:3238 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/store/store.go:2981 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/learn/journal.go:278 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/client.go:1471 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:248 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:202 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:195 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:190 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:155 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:151 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:141 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
| internal/client/chrome.go:124 | G104 | LOW | Errors unhandled | Generator review; emitted framework file |
