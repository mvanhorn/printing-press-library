# Security static-analysis review

Pinned scanner: gosec v2.26.1. Initial scan: 23 findings, of which three are in hand-authored Travel code. Those three were returned to the domain worker for explicit body-close handling and narrowly justified cache-path/directory-permission annotations. Final scan: 20 generated-framework findings remain; zero findings in hand-authored Travel code.

The remaining 20 findings are generated framework/template candidates, retained below without modifying shared Printing Press or masking them. The public Travel client is separate, accepts no credentials, uses fixed hosts, and never invokes the generated store. Review found no critical defect in the shipped public workflows.

| File / line | Rule | Review context |
|---|---|---|
| `internal/client/client.go:454` | G119 | Generated redirect code gates credential re-stamping on same origin; shipped auth is none. |
| `internal/platform/gate.go:22` | G101 | Enum value invalid_credentials is a status, not a credential. |
| `internal/store/store.go:3296-3299` | G201 | Generated store SQL identifier/path composition; inspected identifier validation and fixed ordering. Public Travel does not use this store. |
| `internal/store/store.go:2993-2996` | G201 | Generated store SQL identifier/path composition; inspected identifier validation and fixed ordering. Public Travel does not use this store. |
| `internal/store/store.go:1037` | G201 | Generated store SQL identifier/path composition; inspected identifier validation and fixed ordering. Public Travel does not use this store. |
| `internal/store/store.go:1013` | G201 | Generated store SQL identifier/path composition; inspected identifier validation and fixed ordering. Public Travel does not use this store. |
| `internal/platform/migration.go:156` | G202 | VACUUM target path is SQL-quote escaped; generated local migration only. |
| `internal/store/store.go:249` | G304 | Generated local configuration/profile/cache/feedback path; user-controlled local file operation, not public-source URL inclusion. |
| `internal/platform/ratelimit.go:148` | G304 | Generated local configuration/profile/cache/feedback path; user-controlled local file operation, not public-source URL inclusion. |
| `internal/platform/profile.go:510` | G304 | Generated local configuration/profile/cache/feedback path; user-controlled local file operation, not public-source URL inclusion. |
| `internal/config/config.go:105` | G304 | Generated local configuration/profile/cache/feedback path; user-controlled local file operation, not public-source URL inclusion. |
| `internal/cli/feedback.go:204` | G304 | Generated local configuration/profile/cache/feedback path; user-controlled local file operation, not public-source URL inclusion. |
| `internal/cli/feedback.go:66` | G304 | Generated local configuration/profile/cache/feedback path; user-controlled local file operation, not public-source URL inclusion. |
| `internal/platform/profile.go:302` | G302 | Generated directory creation/chmod uses owner traversal permissions; inspect separately from file permissions. |
| `internal/cliutil/testenv/testenv.go:76` | G302 | Generated directory creation/chmod uses owner traversal permissions; inspect separately from file permissions. |
| `internal/cliutil/testenv/sandbox_unix.go:15` | G302 | Generated directory creation/chmod uses owner traversal permissions; inspect separately from file permissions. |
| `internal/client/client.go:800` | G302 | Generated directory creation/chmod uses owner traversal permissions; inspect separately from file permissions. |
| `internal/store/store.go:3084` | G104 | Generated cleanup/auxiliary error handling; retained as template maintenance candidate, outside public Travel code. |
| `internal/store/store.go:2827` | G104 | Generated cleanup/auxiliary error handling; retained as template maintenance candidate, outside public Travel code. |
| `internal/client/client.go:1512` | G104 | Generated cleanup/auxiliary error handling; retained as template maintenance candidate, outside public Travel code. |
