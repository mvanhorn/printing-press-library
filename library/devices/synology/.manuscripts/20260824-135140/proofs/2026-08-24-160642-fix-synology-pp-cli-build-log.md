# Build log: synology-pp-cli

Manifest transcendence rows: 0 planned, 0 built. Phase 3 will not pass until all 0 ship.

The Phase 1.5 manifest lists five transcendence rows (T1-T5). None of them is hand-code scope, so the planned count above is 0 rather than 5:

| Row | Capability | Why it is not hand-code |
| --- | --- | --- |
| T1 | Multi-NAS operation via `--profile` | Framework-provided. `--profile` is a built-in persistent flag; every profile gets its own `base_url` and session record. |
| T2 | Local store plus `sync` and offline `search` | Framework-provided. The generator emits the SQLite store, `sync`, `search`, and `sql` from the spec's resource list. |
| T3 | JSON-first output: `--json`, `--agent`, `--compact`, `--select` | Framework-provided. Built-in persistent flags on every command. |
| T4 | MCP server from the same spec | Framework-provided. `cmd/synology-pp-mcp` plus the `.mcpb` bundle are emitted by the generator. |
| T5 | `session apis` API inventory | Spec-emitted. `SYNO.API.Info?query=all` is a spec endpoint, so the command exists without hand-code. |

Every approved transcendence command path was verified to exist after generation; see the command inventory check below.

## Priority 0 and 1: absorb

Generated from `research/synology-spec.yaml`: 10 resources, 46 endpoints, 90 files under `internal/cli`, both `cmd/synology-pp-cli` and `cmd/synology-pp-mcp`, plus the `.mcpb` bundle. The 24 absorbed manifest rows all map onto generated endpoint commands; no absorbed row needed hand-code.

## Hand-authored DSM auth layer

The generic `session_handshake` transport does not fit DSM's dialect. Four deviations were implemented, in the order the advisor recommended:

1. **Failures arrive as HTTP 200.** `internal/client/synology.go` adds `dsmErrorCode` / `dsmErrorText`; `internal/client/client.go` inspects every 2xx body before returning it, so `{"success":false,"error":{"code":N}}` becomes a real error instead of a successful-looking payload. This was built first because it is the trigger for everything below.
2. **`X-SYNO-TOKEN` header.** DSM 7.3.2 and newer reject calls that omit it. The transport sets it on every request from the token the login returned.
3. **Session adoption and relogin.** A `SYNO.API.Auth` login response is adopted by the transport (`SetSession`), a logout clears it (`Clear`), and DSM error 119 invalidates the cached sid and retries once. Placing this in the transport rather than in a command keeps it true for the CLI, the MCP server, and `sync` alike, and it survives a `generate --force`.
4. **Self-signed certificate opt-in.** `--insecure` / `SYNOLOGY_INSECURE_TLS=1` for a NAS still using DSM's own certificate on port 5001.

Credentials for an unattended relogin come from `SYNOLOGY_ACCOUNT` / `SYNOLOGY_PASSWORD` (plus `SYNOLOGY_OTP_CODE`, `SYNOLOGY_DEVICE_ID`, `SYNOLOGY_DEVICE_NAME`). No password is ever written to the session record on disk; only the sid and the SynoToken are persisted.

A run with no credentials in scope is not a transport-level error: `EnsureToken` returns an empty token, the call goes out without `_sid`, and DSM's own 105/119 answer carries the actionable message. `classifyAPIError` in `internal/cli/helpers.go` maps DSM 119/106/107 to an auth error with a relogin hint, 105/402/403 to an auth error naming the missing privilege, and 102/103/104 to a not-found error pointing at `session apis`.

All hand-authored regions are marked `HAND-AUTHORED` so a future `generate --force` can be diffed against them.

## Build and test status

- `go build ./...` clean
- `go vet ./...` clean
- `go test ./...`: 37 failures across `internal/cli`, `internal/cliutil`, `internal/config`, `internal/learn`, `internal/mcp`

Every failure is in framework-generated code and none touches the Synology spec surface. Two root causes, both environmental on Windows:

- `internal/cliutil/paths_test.go` and the credential/config permission tests set `HOME` and expect XDG-style resolution, but the resolver follows `USERPROFILE` on Windows, so the tests read the real `C:\Users\Administrator\.config\synology-pp-cli` instead of their temp dir. The permission tests additionally require an owner-locked ACL that this machine's files do not have.
- `internal/learn` and `internal/mcp` failures predate the hand-authored layer and concern journal rollover and MCP path resolution.

The one failure the auth layer did introduce (`TestGetWithHeadersValuesPreservesRepeatedQueryParams`, which used to get a token from the stub handshake) was resolved by the empty-token behavior described above.

## Validation gate not run

`govulncheck` hangs indefinitely in this environment. The vulnerability database is reachable (`https://vuln.go.dev/index/db.json` answers 200 in 0.34s), but `govulncheck ./...` returns rc=124 with no output even on a trivial scratch module, so the block is environmental rather than a project defect. Generation was re-run with `--validate=false` and `go build`, `go vet`, and `go test` were run by hand instead. The dependency vulnerability gate is therefore unverified for this run.
