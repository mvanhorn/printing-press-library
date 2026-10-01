# Generator diagnostics

The current security scan has zero unresolved findings in hand-authored code. The following 22 findings remain in generator-emitted/reserved packages and were classified as Printing Press retro candidates; no upstream source, shared configuration or other CLI was modified. Reachable vulnerability scan: none found.

- G119 internal/client/client.go:454 — Sensitive headers should not be re-added in redirect policy callbacks
- G101 internal/platform/gate.go:22 — Potential hardcoded credentials
- G101 cmd/eplus-pp-mcp/main.go:36 — Potential hardcoded credentials
- G201 internal/store/store.go:3296-3299 — SQL string formatting
- G201 internal/store/store.go:2993-2996 — SQL string formatting
- G201 internal/store/store.go:1037 — SQL string formatting
- G201 internal/store/store.go:1013 — SQL string formatting
- G202 internal/platform/migration.go:156 — SQL string concatenation
- G304 internal/store/store.go:249 — Potential file inclusion via variable
- G304 internal/platform/ratelimit.go:148 — Potential file inclusion via variable
- G304 internal/platform/profile.go:510 — Potential file inclusion via variable
- G304 internal/config/config.go:104 — Potential file inclusion via variable
- G304 internal/cli/feedback.go:204 — Potential file inclusion via variable
- G304 internal/cli/feedback.go:66 — Potential file inclusion via variable
- G112 cmd/eplus-pp-mcp/main.go:83-86 — Potential Slowloris Attack because ReadHeaderTimeout is not configured in the http.Server
- G302 internal/platform/profile.go:302 — Expect file permissions to be 0600 or less
- G302 internal/cliutil/testenv/testenv.go:76 — Expect file permissions to be 0600 or less
- G302 internal/cliutil/testenv/sandbox_unix.go:15 — Expect file permissions to be 0600 or less
- G302 internal/client/client.go:800 — Expect file permissions to be 0600 or less
- G104 internal/store/store.go:3084 — Errors unhandled
- G104 internal/store/store.go:2827 — Errors unhandled
- G104 internal/client/client.go:1504 — Errors unhandled

Structural dogfood also reports generic dead helpers/maxAge and no dedicated archive sync; this CLI intentionally has no offline mirror. MCP typed metadata counts two spec endpoint tools, while the rebuilt runtime exposes nine tools; the source-backed six canonical discovery tools are verified as read-only CLI mirrors.
