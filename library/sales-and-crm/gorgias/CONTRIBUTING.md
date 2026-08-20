# Contributing

Bug reports, install-path issues, and Gorgias API correctness fixes are
welcome. New features may be re-routed depending on how they interact
with the upstream Printing Press generator (this CLI is a generated +
hand-tuned snapshot).

## Reporting bugs

Open an issue against [Printing Press Library](https://github.com/mvanhorn/printing-press-library/issues/new).
Useful detail to include:

- `gorgias-pp-cli version --json`
- `gorgias-pp-cli doctor --json` (redact `config_path` if it leaks a
  username you'd rather keep private — every other field is safe to share)
- The exact command that misbehaved + the `--json` output
- Your OS + architecture and how you installed the CLI.

## Sending a fix

1. Fork + branch from `main`.
2. `go test ./... && gofmt -l ./` must both come back clean.
3. If you're touching the wire layer (`internal/client/`, `internal/store/`,
   `internal/cli/*_create.go` / `*_update.go`), run the live shipcheck
   against your own Gorgias tenant before opening the PR. Scripts live
   in `scripts/`:
   - `scripts/shipcheck.sh` — exercises every read endpoint
   - `scripts/writes-live-shipcheck.sh` — safe-roundtrip create/update/delete
     on `tags` / `macros` / `teams` / `views` / `widgets` / `integrations`
   - `scripts/mcp-shipcheck.sh` — MCP server transcript
4. Open the PR against `main`. CI runs `go vet`, `go test`, `gofmt`, plus
   the hero-command smoke (`version --json`, `agent-context --json`,
   `which`, `api`).
5. Include the shipcheck artifact (or its tail) in the PR body if you
   ran one.

## Code style

- `gofmt` for formatting; CI fails on unformatted files.
- Resource error classification goes through `errors.As(*client.APIError)`,
  not substring matching of `err.Error()`. See `internal/cli/helpers.go`
  for the pattern.
- Single-emission JSON error envelopes — under `--json`, failures emit
  exactly one document on stderr (or embedded in stdout for commands
  whose normal output is a status report). See `internal/cli/root.go`
  `writeExecuteError` / `silenceEmission` for the contract.
- No retry wrappers in the HTTP client. 5xx and 429 propagate as a
  typed `*APIError`; callers decide how to handle.

## Releasing

Maintainer-only. Releases are handled through Printing Press Library and
its generated installer surfaces.

## License

Apache-2.0. By submitting a PR, you agree your contributions are
licensed under the same terms (no CLA — Apache-2.0's terms-of-license
covers contributions).
