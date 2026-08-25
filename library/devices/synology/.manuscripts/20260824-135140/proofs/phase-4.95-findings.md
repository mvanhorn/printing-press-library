# Phase 4.95 local code review: synology-pp-cli

Review path chosen: `security-reviewer` subagent, read-only, over the hand-authored DSM layer plus every framework file that touches it (`internal/client/synology.go`, `internal/client/session.go`, `internal/client/client.go`, `internal/config/config.go`, `internal/cli/{root,auth,helpers,session_login,data_source}.go`, `internal/store/store.go`, `internal/mcp/tools.go`, `cmd/*/main.go`). Security lens, because the only deviation from the generated transport is a credential exchange that carries an account password in a query string.

Autofix summary: 7 findings reported, 7 fixed in this session, 0 surfaced to the user for a decision, 0 deferred.

## Findings and dispositions

| # | Severity | Finding | Disposition |
| --- | --- | --- | --- |
| 1 | HIGH | `session login --dry-run` printed `passwd=<plaintext>` to stderr. `maskCredentialText` only redacts values it already knows are credentials, and a bare password matches nothing. | FIXED. `dryRun` now redacts by parameter name via `isDSMSecretParam`. |
| 2 | HIGH | The account password leaked through unmasked network-error text at two sites: `client.go`'s transport-failure branch (Go's `*url.Error` embeds the real, params-merged URL) and `session.go`'s `dsmLogin`, which applied no masking at all. A first-run login against DSM's self-signed certificate hits this on the normal path. | FIXED. `scrubDSMSecrets` redacts `passwd`, `password` and `otp_code` query parameters by name wherever they appear in free text; it runs at the end of `maskCredentialText`, and `dsmLogin` wraps its own transport errors in `scrubDSMSecretError`. |
| 3 | HIGH | `session login` ran through the `auto` data-source strategy, so `writeThroughCache` upserted the login response body - the sid and the SynoToken - into the local SQLite store. Nothing expired that row, and `session logout` never touched it. | FIXED. `writeThroughCache` skips the `session` resource type. |
| 4 | MEDIUM | The same credentials landed in the HTTP response cache. DSM wires login and logout as GETs, and cache invalidation only fires for mutating verbs, so logout left the cached sid on disk. | FIXED. `responseCacheEnabled` now takes the request path and returns false for DSM auth calls. |
| 5 | MEDIUM | `session.json`, which holds the live sid and SynoToken, was written with `os.WriteFile`: truncate-in-place, and its mode argument only applies on creation, so a pre-existing file at looser permissions was never tightened. | FIXED. It now uses `cliutil.AtomicWritePrivateFile`, the same helper `config.json` already used. |
| 6 | LOW | The learn journal's redaction pattern matched `password` but not `passwd` or `otp`. Not exploitable today - the journal records a value's class, never its value - but it would have silently reintroduced a leak if that ever changed. | FIXED, defense in depth. |
| 7 | LOW | `auth login --cookies-file` called `ImportSession` with an empty token, discarding a still-valid cached session id. Correctness, not a leak. | FIXED. It passes the cached token through. |

## Convergence outcome

One review round, no disagreement to reconcile. The reviewer also confirmed three things as safe with evidence rather than assumption: `--insecure` / `SYNOLOGY_INSECURE_TLS` is strictly opt-in and never inferred from port or host; session adoption keys off the outgoing request's own hardcoded path, so a hostile response cannot get itself adopted as a session; and the DSM 200-body error branch quotes the pre-params path, which carries no credential.

## Verification

- `go build ./...`, `go vet ./...`: clean.
- `go test ./internal/client/`: ok. `internal/client/synology_secret_test.go` is hand-authored and asserts that a password survives neither `scrubDSMSecrets` nor `scrubDSMSecretError`, that innocent text is left byte-identical, and that `isDSMSecretParam` accepts `passwd`/`password`/`otp_code` and rejects ordinary parameters.
- Finding 1 was reproduced and re-checked against the real binary, which needs no network for a dry run: `session login --account admin --passwd 'S3cr3tPass' --dry-run` now prints `&passwd=***`.

## Retro candidates

None. Nothing credential-relevant was found in the generator-reserved areas (`internal/cliutil/`, `internal/mcp/cobratree/`); every fix landed in files this run owns.
