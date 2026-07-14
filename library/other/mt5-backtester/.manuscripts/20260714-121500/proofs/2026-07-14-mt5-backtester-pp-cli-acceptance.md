# mt5-backtester-pp-cli — Acceptance Record (packaging run 20260714-121500)

Structural validation executed on the packaging machine on 2026-07-14 (Go 1.26.5, windows/amd64, no MT5 terminal present). Runtime behavior (real Strategy Tester runs, compiles, report parses, batch grids) was exercised during development on an MT5 host; this record covers what was re-verified at packaging time.

## Packaging-run validation (2026-07-14)

| Check | Result | Evidence |
|---|---|---|
| `go build ./...` | PASS | clean; see `build-log.txt` |
| `go vet ./...` | PASS | clean |
| `go test ./...` | PASS | internal/setfile suite ok (set-file parse/write round-trips preserving `value||start||step||stop||Y/N` ranges) |
| `mt5-backtester-pp-cli --version` | PASS | `mt5-backtester-pp-cli version 2.0.0` |
| `mt5-backtester-pp-cli --help` | PASS | full tree renders (run, compile, report, batch, setfile, profile, service, config, template); capture in `mt5-backtester-pp-cli-help.txt` |
| `verify_skill.py` | SKIP (by design) | verifier requires `internal/cli/*.go`; this CLI's cobra tree lives in `cmd/mt5-backtester-pp-cli/main.go`, and the CI workflow skips such layouts |
| `govulncheck ./...` | PASS | 0 reachable vulnerabilities |

## Packaging-run changes

The binary and all documentation were renamed from the standalone `pp-mt5-backtester` (and its README's incorrect bare `pp-mt5` usage, which collided with the sibling live-trading CLI) to the catalog-convention `mt5-backtester-pp-cli`. The profile store path moved accordingly (`~/.mt5-backtester-pp-cli/profiles.json`). No behavioral changes.

## Verdict

**PASS** — structurally validated at packaging time. Full runtime verification requires a Windows host with MetaTrader 5 installed and history downloaded; first-run checklist for operators: `mt5-backtester-pp-cli config`, then a short `run --ea "MACD Sample" --symbol EURUSD --period H1`.
