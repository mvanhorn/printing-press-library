# 2026-08-23 Agent platform CSV, help, doctor PATH

## Intent

Preserve P1/P2 agent-platform surfaces across reprints:

- `--csv` unwraps a root `{items:[...]}` envelope (reservation list) into CSV rows.
- `-h` / `--help` stay short (Use, Short, local flags, examples). `--help-all` shows global flags.
- `doctor` warns when the binary lives in `~/.local/bin` or `GOPATH/bin` off PATH, and names `wanderlog-pp-cli` (not `wanderlog`).
- CLI version string `1.1.0`.

## Touched Surface

- `internal/cli/helpers.go`: `printCSV` / `csvObjectRows`.
- `internal/cli/root.go`: `--help-all`, short HelpFunc (do not drop `--verbose` / `--also-stdout`).
- `internal/cli/doctor.go`: install PATH report helper.
- `internal/cli/version.go`: `1.1.0`.
- `internal/cli/root_test.go`, `internal/cli/doctor_test.go`.

## Verification

- `go test ./internal/cli/ -count=1 -timeout 120s`
