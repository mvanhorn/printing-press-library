# Phase19 security scanner judgment

Pinned scanner: gosec v2.26.1. Both scans completed and emitted JSON; exit1 reflects raw findings, not scanner failure. Before 25 raw findings:4 handwritten cleanup findings and21 individually accepted generated/reserved findings. After 21 raw findings:0 unresolved handwritten findings. No generated or reserved package was changed for score alone. These decisions use actual path/guard behavior, not the Generated header.

## Fixed findings

| Rule | Source | Resolution |
|---|---|---|
| G304 | `internal/source/discovery.go:129` | Added narrow nosec rationale for fixed .choices.json under private configured cache directory; path does not derive from public choice labels. |
| G104 | `internal/notebook/notebook.go:182` | Explicitly marked best-effort Close while returning the original Scan failure. Normal successful Close error remains propagated. |
| G104 | `internal/notebook/notebook.go:191` | Explicitly marked best-effort Close while returning the original Rows.Err failure. Normal successful Close error remains propagated. |
| G104 | `internal/cli/tabelog_helpers.go:29` | Explicitly marked best-effort db.Close only when notebook initialization already failed, preserving the initialization error. |

## Remaining raw findings, individually assessed

| Rule | Source | Decision | Behavior and boundary |
|---|---|---|---|
| G119 | `internal/client/client.go:454` | Accepted constrained generated redirect callback | Re-stamping requires !redirectLeavesOrigin: the destination Host must equal the original request Host; a prior HTTPS hop cannot become plaintext. The preceding destination guard rejects unsupported schemes, HTTPS downgrade, and off-origin private IP literals. Config.AuthHeader only returns an operator-supplied auth_header; none is discovered from an account or environment in this no-auth print. This acceptance covers the guarded re-stamp, not a claim that every custom header/DNS redirect is universally safe. Native Tabelog traffic uses its own strict https://tabelog.com/en/ allowlist, five-redirect cap, and no generated auth/config headers. Generic Go inherited-header/subdomain semantics and custom-header policy remain framework limitations, outside native source behavior. |
| G101 | `internal/platform/gate.go:22` | False positive | invalid_credentials is a public error enum, not a password or token. |
| G201 | `internal/store/store.go:3554-3557` | Accepted validated identifiers | ResolveByName validates the dynamic JSON field with validIdentifierRE before formatting; values/resource remain bound parameters. |
| G201 | `internal/store/store.go:3251-3254` | Accepted validated identifiers | ListField validates field with validIdentifierRE; typed table/column names are quoted and lookup values are bound. |
| G201 | `internal/store/store.go:1099` | Accepted validated identifiers | TypedListRange receives its table from validated typed-table metadata; newest-first order is assembled only from fixed recognized columns. |
| G201 | `internal/store/store.go:1075` | Accepted validated identifiers | The PRAGMA table name passes validIdentifierRE before formatting. |
| G202 | `internal/platform/migration.go:156` | Accepted quoted operator destination | VACUUM INTO quotes single quotes in targetPath. The explicit migration target is checked for pre-existence and derived from the platform state scope; this no-auth CLI does not register platform management. |
| G304 | `internal/store/store.go:249` | Accepted fixed private journal | The journal path is derived from dbPath plus -journal and opened with O_EXCL and mode 0600; no restaurant field selects it. |
| G304 | `internal/platform/ratelimit.go:148` | Accepted fixed private lock | ledgerLockPath is derived from the private platform state root and opened exclusively with 0600; no public source data selects it. |
| G304 | `internal/platform/profile.go:510` | Accepted fixed private cache-key | The fixed cache-key path is derived from stateRoot and created O_EXCL/0600; existing key contents are validated. |
| G304 | `internal/config/config.go:105` | Accepted explicit operator configuration | This reads the explicitly requested config file, or the config path derived from the CLI home. It is not inclusion of a fetched URL or restaurant value. |
| G304 | `internal/cli/feedback.go:204` | Accepted fixed private feedback ledger | This reads the fixed feedback.jsonl under StateDir; feedback is hidden framework plumbing, not a source-selected file. |
| G304 | `internal/cli/feedback.go:66` | Accepted fixed private feedback ledger | This appends the fixed private feedback.jsonl with 0600 under StateDir; it does not interpret feedback as code. |
| G304 | `internal/cli/export.go:213` | Accepted explicit operator output | The operator supplies --output for an intentional export. The destination directory is private and the file is created/tightened to0600; this is not arbitrary source-data inclusion. Export is hidden from the runtime MCP surface. |
| G302 | `internal/platform/profile.go:302` | False positive directory permission | 0700 applies to a private directory: owner execute is required to traverse it. File contents retain0600. |
| G302 | `internal/cliutil/testenv/testenv.go:76` | False positive reserved test directory | 0700 applies to a private test home directory, not a data file; reserved package untouched. |
| G302 | `internal/cliutil/testenv/sandbox_unix.go:15` | False positive reserved test directory | 0700 applies to a private test home directory, not a data file; reserved package untouched. |
| G302 | `internal/client/client.go:800` | False positive cache directory permission | ensureCachePerms uses 0700 for the resource directory and 0600 for the cache file; owner traversal is necessary. |
| G104 | `internal/store/store.go:3342` | Accepted optional generic status hint | A missing generic sync-state timestamp becomes an absent age hint. This is outside normalized notebook snapshot reads/writes; no commit or snapshot error is swallowed. |
| G104 | `internal/store/store.go:3085` | Accepted optional generic cursor | A missing generic raw-sync cursor yields an empty cursor. No normalized source snapshot or notebook mutation is silently committed through this path. |
| G104 | `internal/client/client.go:1509` | Accepted nonpersistent preview formatting | The ignored Encode result is pretty-printing an already parsed JSON request to stderr in a dry-run diagnostic. It does not persist data or send a request. |

## Prior security changes remain intact

Phase17 security re-review remains the deeper behavioral proof: recipe positionals remain behind a flag terminator; all emitted generated nonbinary response reads retain the 32 MiB decoded guard, with explicit binary streams preserved; output delivery retains exclusive randomized CreateTemp FD/write/close/rename/cleanup. Native Tabelog remains 4 MiB and an exact English-origin URL/redirect allowlist. This polish did not change those paths or optional OAuth branches. No universal OAuth/client security claim is made.

Artifacts: `polish-gosec-{before,after}.json`, `phase17-security-r2.md`, `phase19-polish-cleanup.patch`, `phase19-polish-cleanup-provenance.json`.

Final post-metadata scan: polish-gosec-final.json completed and has the identical 21 accepted rule/path/line tuples; no new findings. Final go vet is clean. The narrow MCP registration/recipe hint edits introduced no transport, input, storage or authorization changes.
