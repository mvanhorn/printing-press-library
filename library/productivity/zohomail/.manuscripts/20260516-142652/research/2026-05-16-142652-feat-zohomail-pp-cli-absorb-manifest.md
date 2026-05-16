# Absorb Manifest

## Source Package
- Local source: `/Users/jacques/DevFolder/pp-zohomail-cli`
- Published library path: `library/productivity/zohomail`
- CLI binary: `zohomail-pp-cli`
- Module: `github.com/mvanhorn/printing-press-library/library/productivity/zohomail`

## Shape
- Single stdlib-only Go CLI library package with `cmd/zohomail-pp-cli` executable entrypoint.
- Testable entrypoint is `run(args, stdout, stderr)`.
- HTTP behavior is covered with `httptest.Server` tests.
- Printing Press verifier bridge lives in `internal/cli/verify_skill_contract.go` under `//go:build ignore` for contract validation only.

## Publish Notes
- Root package exports `Main()` so the library path can be imported while retaining the executable command.
- OAuth callback handling includes a regression guard for extra callback requests.
- Phase 5 live OAuth was skipped because credentials are user-specific; local tests and verifier checks cover the public package.
