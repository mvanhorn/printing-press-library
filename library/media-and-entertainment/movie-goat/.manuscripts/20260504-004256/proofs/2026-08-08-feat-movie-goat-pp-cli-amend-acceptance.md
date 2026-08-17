# movie-goat-pp-cli Amend Acceptance Report — feat/movie-goat-omdb-key-in-config (PR #1599)

Scope: verification evidence for the OMDb-credential amend at head `68cb19d5e`
(PR mvanhorn/printing-press-library#1599), not a reprint. The base print evidence lives
alongside in this run's proofs (run `20260504-004256`). The amend changes no dependencies,
no MCP surface, and no network behavior — it is a credential-storage change, so the smoke
runs against an isolated HOME with a dummy key and zero network calls.

```
Amend Acceptance: movie-goat
  Level: full module test suite + isolated-HOME credential round-trip
  Tests: go test ./... — 6 packages PASS, 0 failed
  Auth: dummy key in throwaway config; no real credential involved
  Gate: PASS
```

## Finding-by-finding verification

| ID | Contract | Evidence |
|----|----------|----------|
| F1 | `omdb_api_key` has a persistent slot in `config.toml` | smoke: `auth set-omdb-token` writes the key to the isolated config; `auth logout` leaves `omdb_api_key = ''` on disk (round-trip clear); env-supplied values never reach disk, pinned by `config_test.go` |
| F2 | `auth set-omdb-token` mirrors `auth set-token` | smoke: subcommand saves and reports the target path; discoverable under `auth --help` |
| F3 | `auth status` / `doctor` report the OMDb credential and its source | smoke: `config:omdb_api_key` after save; `env:OMDB_API_KEY` when the environment variable is set (precedence preserved) |

## Verification legs (all PASS)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `gofmt -l .` — clean
- `go test ./...` — 6 packages ok, 0 failures (includes the new `config_test.go`: file/env
  resolution, precedence, graceful absence, save/clear round-trip, no-leak-to-disk)
- `verify_skill.py --dir library/media-and-entertainment/movie-goat/` — all checks passed
- `govulncheck ./...` — 0 vulnerabilities in called code (1 module-level finding in a
  dependency whose vulnerable path this code does not call, unchanged from `main`)

Raw command transcripts: [`2026-08-08-feat-movie-goat-pp-cli-amend-build-log.md`](2026-08-08-feat-movie-goat-pp-cli-amend-build-log.md)

## Fixtures used

- Isolated `HOME` under a session scratchpad, dummy token `test-omdb-key-123`, environment
  variable exercised as `OMDB_API_KEY=env-wins`. No live API call, no real credential, and
  the user-level config was never touched.
