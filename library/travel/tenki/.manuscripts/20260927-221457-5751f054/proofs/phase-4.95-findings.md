Root local code review — PASS for approved CLI scope

Review path: direct sole root orchestrator review, per explicit user instruction. Reviewed correctness, security, maintainability, source/API semantics and performance; all eight live paths bind context/timeouts before requests. Strict host/path/redirect allowlists, bounded bodies/cache decoding, finite criteria, date coverage, source kind/year and error preservation are exercised by regression/live tests. No arbitrary source URLs, script execution or external writes in product commands.

Autofix summary: output provenance and partial date-range findings were corrected before this gate; see build log, patches and regression tests. No commits requested. Current formal review converged at round1 with no in-scope custom finding.

Template/out-of-scope scanner candidates (not patched in generated files):

- internal/client/client.go:454 G119 (HIGH): Redirect code checks origin before re-stamping; public product client is separate and sends no auth. Template: internal/generator/templates/client.go.tmpl
- internal/platform/gate.go:22 G101 (HIGH): Enum/environment-variable names, not credentials. Template: internal/generator/templates/platform_gate.go.tmpl
- cmd/tenki-pp-mcp/main.go:36 G101 (HIGH): Enum/environment-variable names, not credentials. Template: internal/generator/templates/main_mcp.go.tmpl
- internal/store/store.go:3296-3299 G201 (MEDIUM): Identifiers are validated and values parameterized before interpolation. Template: internal/generator/templates/store.go.tmpl
- internal/store/store.go:2993-2996 G201 (MEDIUM): Identifiers are validated and values parameterized before interpolation. Template: internal/generator/templates/store.go.tmpl
- internal/store/store.go:1037 G201 (MEDIUM): Identifiers are validated and values parameterized before interpolation. Template: internal/generator/templates/store.go.tmpl
- internal/store/store.go:1013 G201 (MEDIUM): Identifiers are validated and values parameterized before interpolation. Template: internal/generator/templates/store.go.tmpl
- internal/platform/migration.go:156 G202 (MEDIUM): VACUUM target is a locally chosen path with single quotes escaped. Template: internal/generator/templates/platform_migration.go.tmpl
- internal/store/store.go:249 G304 (MEDIUM): Intended user-configured local config/cache/state file access; no remote path source. Template: internal/generator/templates/store.go.tmpl
- internal/platform/ratelimit.go:148 G304 (MEDIUM): Intended user-configured local config/cache/state file access; no remote path source. Template: internal/generator/templates/platform_ratelimit.go.tmpl
- internal/platform/profile.go:510 G304 (MEDIUM): Intended user-configured local config/cache/state file access; no remote path source. Template: internal/generator/templates/platform_profile.go.tmpl
- internal/config/config.go:105 G304 (MEDIUM): Intended user-configured local config/cache/state file access; no remote path source. Template: internal/generator/templates/config.go.tmpl
- internal/cli/feedback.go:204 G304 (MEDIUM): Intended user-configured local config/cache/state file access; no remote path source. Template: internal/generator/templates/feedback.go.tmpl
- internal/cli/feedback.go:66 G304 (MEDIUM): Intended user-configured local config/cache/state file access; no remote path source. Template: internal/generator/templates/feedback.go.tmpl
- cmd/tenki-pp-mcp/main.go:83-86 G112 (MEDIUM): Real template limitation: optional authenticated MCP HTTP server omits ReadHeaderTimeout. No HTTP server launched; CLI delivery unaffected. Template: internal/generator/templates/main_mcp.go.tmpl
- internal/platform/profile.go:302 G302 (MEDIUM): These operations set directory permissions to0700; file permissions remain0600. Template: internal/generator/templates/platform_profile.go.tmpl
- internal/cliutil/testenv/testenv.go:76 G302 (MEDIUM): These operations set directory permissions to0700; file permissions remain0600. Template: internal/generator/templates/cliutil_testenv.go.tmpl
- internal/cliutil/testenv/sandbox_unix.go:15 G302 (MEDIUM): These operations set directory permissions to0700; file permissions remain0600. Template: internal/generator/templates/cliutil_testenv_sandbox_unix.go.tmpl
- internal/client/client.go:800 G302 (MEDIUM): These operations set directory permissions to0700; file permissions remain0600. Template: internal/generator/templates/client.go.tmpl
- internal/store/store.go:3084 G104 (LOW): Generated optional cursor/debug fallback ignores errors; not used by the product page cache. Template: internal/generator/templates/store.go.tmpl
- internal/store/store.go:2827 G104 (LOW): Generated optional cursor/debug fallback ignores errors; not used by the product page cache. Template: internal/generator/templates/store.go.tmpl
- internal/client/client.go:1504 G104 (LOW): Generated optional cursor/debug fallback ignores errors; not used by the product page cache. Template: internal/generator/templates/client.go.tmpl

The optional MCP HTTP header-timeout limitation is disclosed in README and acceptance. Remaining scanner matches are triaged framework patterns; full details remain in gosec.json. No source change is needed for the approved local CLI.
