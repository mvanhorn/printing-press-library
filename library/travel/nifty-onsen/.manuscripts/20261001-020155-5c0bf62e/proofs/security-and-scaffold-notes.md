# Security/static scaffold triage

Pinned gosec v2.26.1 scanned the tree. Zero findings remain in the handwritten provider parser/client/command/MCP extension. The raw scanner exits nonzero for 22 inherited generated/reserved findings. These are recorded rather than presented as a wholly clean security scan.

The active MCP provider surface excludes generic SQL. Generated SQL/store/config/platform helpers remain preserved. G101 matches below are environment variable names, not credentials. HTTP MCP is optional, authenticated, and was not started; its missing read/header timeout is an inherited template candidate. Generic client decompression/file-permission and store path/query construction findings are inherited framework candidates; primary bath HTTP uses its own bounded client. No generator-reserved files or shared/global configuration were changed.

| Rule | File | Line | Detail |
|---|---|---:|---|
| G119 | internal/client/client.go | 454 | Sensitive headers should not be re-added in redirect policy callbacks |
| G101 | internal/platform/gate.go | 22 | Potential hardcoded credentials |
| G101 | cmd/nifty-onsen-pp-mcp/main.go | 36 | Potential hardcoded credentials |
| G201 | internal/store/store.go | 3296-3299 | SQL string formatting |
| G201 | internal/store/store.go | 2993-2996 | SQL string formatting |
| G201 | internal/store/store.go | 1037 | SQL string formatting |
| G201 | internal/store/store.go | 1013 | SQL string formatting |
| G202 | internal/platform/migration.go | 156 | SQL string concatenation |
| G304 | internal/store/store.go | 249 | Potential file inclusion via variable |
| G304 | internal/platform/ratelimit.go | 148 | Potential file inclusion via variable |
| G304 | internal/platform/profile.go | 510 | Potential file inclusion via variable |
| G304 | internal/config/config.go | 104 | Potential file inclusion via variable |
| G304 | internal/cli/feedback.go | 204 | Potential file inclusion via variable |
| G304 | internal/cli/feedback.go | 66 | Potential file inclusion via variable |
| G112 | cmd/nifty-onsen-pp-mcp/main.go | 83-86 | Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server |
| G302 | internal/platform/profile.go | 302 | Expect file permissions to be 0600 or less |
| G302 | internal/cliutil/testenv/testenv.go | 76 | Expect file permissions to be 0600 or less |
| G302 | internal/cliutil/testenv/sandbox_unix.go | 15 | Expect file permissions to be 0600 or less |
| G302 | internal/client/client.go | 800 | Expect file permissions to be 0600 or less |
| G104 | internal/store/store.go | 3084 | Errors unhandled |
| G104 | internal/store/store.go | 2827 | Errors unhandled |
| G104 | internal/client/client.go | 1504 | Errors unhandled |

The unsupported generated tail template is preserved byte-identically in internal/cli/tail.go.disabled and in the saved original scaffold. It is excluded from the active CLI because this provider supplies no generic incremental JSON resource endpoint. All seven approved provider workflows remain implemented.

Publishing, PR creation, reservations, purchases, payments, account changes and global setup changes were not performed.
