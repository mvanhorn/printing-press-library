# Generated security scanner findings retained for Printing Press review

Scanner: gosec v2.29.0, built against golang.org/x/tools v0.52.0. All packages loaded; no Golang errors. Handwritten findings: 5 before, 0 after. Remaining 51 findings are in generator-owned files and are retained as retro candidates per polish policy. They were not silently suppressed or claimed fixed.

| Rule | File | Line | Finding |
|---|---|---:|---|
| G115 | internal/cliutil/darwin_acl.go | 230 | integer overflow conversion uint32 -> int32 |
| G115 | internal/client/client.go | 3988 | integer overflow conversion uint16 -> byte |
| G115 | internal/client/client.go | 3986 | integer overflow conversion uint16 -> byte |
| G115 | internal/client/client.go | 4121 | integer overflow conversion rune -> uint32 |
| G115 | internal/client/client.go | 3982 | integer overflow conversion rune -> uint16 |
| G115 | internal/client/client.go | 3980 | integer overflow conversion rune -> uint16 |
| G115 | internal/cliutil/text.go | 115 | integer overflow conversion rune -> byte |
| G115 | internal/cli/store_adopt_darwin.go | 212 | integer overflow conversion int32 -> uint64 |
| G115 | internal/cli/store_adopt_darwin.go | 167 | integer overflow conversion int32 -> uint32 |
| G115 | internal/cli/store_adopt_darwin.go | 155 | integer overflow conversion int32 -> uint32 |
| G115 | internal/cliutil/filelock_unix.go | 76 | integer overflow conversion int -> uint32 |
| G115 | internal/cliutil/darwin_acl_darwin.go | 143 | integer overflow conversion int -> uint32 |
| G115 | internal/cliutil/creds_perms_unix.go | 71 | integer overflow conversion int -> uint32 |
| G115 | internal/cliutil/creds_perms_unix.go | 53 | integer overflow conversion int -> uint32 |
| G101 | internal/platform/gate.go | 23 | Potential hardcoded credentials |
| G101 | internal/learn/match.go | 78 | Potential hardcoded credentials |
| G703 | internal/cli/teach.go | 211 | Path traversal via taint analysis |
| G201 | internal/store/store.go | 4441-4444 | SQL string formatting |
| G201 | internal/store/store.go | 4014-4017 | SQL string formatting |
| G201 | internal/store/store.go | 1682 | SQL string formatting |
| G201 | internal/store/store.go | 1658 | SQL string formatting |
| G202 | internal/store/store.go | 1206 | SQL string concatenation |
| G202 | internal/store/learnings.go | 610 | SQL string concatenation |
| G202 | internal/platform/migration.go | 158 | SQL string concatenation |
| G304 | internal/store/store.go | 311 | Potential file inclusion via variable |
| G304 | internal/platform/ratelimit.go | 149 | Potential file inclusion via variable |
| G304 | internal/platform/profile.go | 511 | Potential file inclusion via variable |
| G304 | internal/learn/teach_log.go | 113 | Potential file inclusion via variable |
| G304 | internal/learn/playbooks.go | 74 | Potential file inclusion via variable |
| G304 | internal/config/config.go | 185 | Potential file inclusion via variable |
| G304 | internal/client/client.go | 388 | Potential file inclusion via variable |
| G304 | internal/cli/teach_playbook.go | 380 | Potential file inclusion via variable |
| G304 | internal/cli/teach.go | 151 | Potential file inclusion via variable |
| G304 | internal/cli/teach.go | 128 | Potential file inclusion via variable |
| G304 | internal/cli/store_adopt_darwin.go | 100 | Potential file inclusion via variable |
| G304 | internal/cli/helpers.go | 4732 | Potential file inclusion via variable |
| G304 | internal/cli/helpers.go | 4693 | Potential file inclusion via variable |
| G304 | internal/cli/feedback.go | 209 | Potential file inclusion via variable |
| G304 | internal/cli/feedback.go | 67 | Potential file inclusion via variable |
| G304 | internal/cli/export.go | 214 | Potential file inclusion via variable |
| G117 | internal/learn/journal.go | 266 | Marshaled struct field "SessionKey" (JSON key "session_key") matches secret pattern |
| G302 | internal/platform/profile.go | 303 | Expect file permissions to be 0600 or less |
| G302 | internal/cliutil/testenv/testenv.go | 79 | Expect file permissions to be 0600 or less |
| G302 | internal/cliutil/testenv/sandbox_unix.go | 16 | Expect file permissions to be 0600 or less |
| G302 | internal/client/client.go | 1775 | Expect file permissions to be 0600 or less |
| G103 | internal/cli/store_adopt_darwin.go | 212 | Use of unsafe calls should be audited |
| G103 | internal/cli/store_adopt_darwin.go | 197 | Use of unsafe calls should be audited |
| G103 | internal/cli/store_adopt_darwin.go | 179 | Use of unsafe calls should be audited |
| G104 | internal/store/store.go | 4105 | Errors unhandled |
| G104 | internal/store/store.go | 3739 | Errors unhandled |
| G104 | internal/learn/journal.go | 279 | Errors unhandled |
