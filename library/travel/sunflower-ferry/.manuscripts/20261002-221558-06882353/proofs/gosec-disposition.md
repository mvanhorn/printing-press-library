# Security static analysis disposition

Pinned gosec v2.26.1 scanned 103 files / 35,594 lines, with 30 findings. No finding touches hand-written internal/ferry or the native planning handlers. Scan exit 1 is the reported findings count, not a missing/failed scan. All findings below arise from shared Press-emitted code. These are classified under the printing-press-polish generator escape hatch; no arbitrary suppression or generated source patch was applied.

The current release has auth.type=none and recommended domain commands use the restricted native ferry client. Lower-level generic source tools and optional framework/local learning surfaces retain shared-template findings; this is disclosed in the acceptance report and README Known Gaps. Template retro candidates remain for upstream review, and are not represented as a clean whole-repository security scan.

| Rule | File:line | Assessment |
|---|---|---|
| G119 | `internal/client/client.go:454` | Generated redirect callback re-adds header state; generic authenticated transport needs upstream review. This no-auth release does not configure customer auth, and native ferry redirects revalidate the HTTPS first-party allowlist. |
| G101 | `internal/platform/gate.go:22` | Generated platform gate contains GateInvalidCredentials = invalid_credentials, an enum status value, not an embedded account credential. |
| G703 | `internal/cli/teach.go:210` | Generated teach import accepts an operator-selected local guidance path; shared tooling candidate, independent of ferry provider transport. |
| G201 | `internal/store/store.go:3450-3453` | Generated SQLite SQL uses formatted query/identifier construction; schema/identifier validation and parameter binding belong in upstream store review. No native ferry SQL calls. |
| G201 | `internal/store/store.go:3147-3150` | Generated SQLite SQL uses formatted query/identifier construction; schema/identifier validation and parameter binding belong in upstream store review. No native ferry SQL calls. |
| G201 | `internal/store/store.go:1191` | Generated SQLite SQL uses formatted query/identifier construction; schema/identifier validation and parameter binding belong in upstream store review. No native ferry SQL calls. |
| G201 | `internal/store/store.go:1167` | Generated SQLite SQL uses formatted query/identifier construction; schema/identifier validation and parameter binding belong in upstream store review. No native ferry SQL calls. |
| G202 | `internal/store/learnings.go:609` | Generated learning/filter SQL concatenation is a shared store candidate; native ferry commands do not call it. |
| G202 | `internal/platform/migration.go:156` | Generated migration quotes the destination with strings.ReplaceAll(targetPath, single quote, doubled single quote) before VACUUM INTO and restricts verified-tenant destination; scanner does not model that escaping. |
| G304 | `internal/store/store.go:253` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/platform/ratelimit.go:148` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/platform/profile.go:510` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/learn/teach_log.go:112` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/learn/playbooks.go:73` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/config/config.go:104` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/cli/teach_playbook.go:376` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/cli/teach.go:150` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/cli/teach.go:127` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/cli/feedback.go:204` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G304 | `internal/cli/feedback.go:66` | Generated local tooling reads explicit operator-selected files/config; review path authorization in shared templates. Native ferry sources use HTTPS allowlists and bounded reads. |
| G112 | `cmd/sunflower-ferry-pp-mcp/main.go:86-89` | Generated optional HTTP MCP server lacks ReadHeaderTimeout. Default stdio package is used in live/bundle verification; optional HTTP server template remains an upstream hardening candidate. |
| G117 | `internal/learn/journal.go:265` | Generated session_key is a learning correlation identifier; potential scanner false positive, not a provider session cookie/token. Native anonymous source tokens are never emitted. |
| G302 | `internal/platform/profile.go:302` | Generated local file mode candidate; testenv modes are harness fixtures, while platform/client artifact modes require upstream review. |
| G302 | `internal/cliutil/testenv/testenv.go:76` | Generated local file mode candidate; testenv modes are harness fixtures, while platform/client artifact modes require upstream review. |
| G302 | `internal/cliutil/testenv/sandbox_unix.go:15` | Generated local file mode candidate; testenv modes are harness fixtures, while platform/client artifact modes require upstream review. |
| G302 | `internal/client/client.go:800` | Generated local file mode candidate; testenv modes are harness fixtures, while platform/client artifact modes require upstream review. |
| G104 | `internal/store/store.go:3238` | Generated ignored-error candidate (cleanup/read path); no native ferry ignored-error finding. |
| G104 | `internal/store/store.go:2981` | Generated ignored-error candidate (cleanup/read path); no native ferry ignored-error finding. |
| G104 | `internal/learn/journal.go:278` | Generated ignored-error candidate (cleanup/read path); no native ferry ignored-error finding. |
| G104 | `internal/client/client.go:1504` | Generated ignored-error candidate (cleanup/read path); no native ferry ignored-error finding. |
