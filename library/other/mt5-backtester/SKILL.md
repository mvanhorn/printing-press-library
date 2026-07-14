---
name: pp-mt5-backtester
description: "Use this skill whenever the user asks to backtest a MetaTrader 5 Expert Advisor, compile MQL5 source files (.mq5/.mqh), parse MT5 strategy tester HTML reports, run batches of MT5 backtests across symbols/timeframes, or work with MT5 .set files (parse, generate, list, apply with overrides). Headless wrapper around terminal64.exe + metaeditor64.exe — no Strategy Tester UI involvement. Windows-only. Triggers on phrases like 'backtest my EA', 'run MT5 strategy tester', 'compile this mq5 file', 'parse this MT5 report', 'batch test EURUSD GBPUSD', 'show my MyEA.set file', 'override one input from the set file'."
author: "ek-labs"
license: "Apache-2.0"
argument-hint: "<command> [args] | install"
allowed-tools: "Read Bash PowerShell"
metadata:
  openclaw:
    requires:
      bins:
        - mt5-backtester-pp-cli
    install:
      - kind: go
        bins: [mt5-backtester-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/cmd/mt5-backtester-pp-cli
---

# MT5 Backtester — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mt5-backtester-pp-cli` binary. **Verify the CLI is installed before invoking any command from this skill.** If missing:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/cmd/mt5-backtester-pp-cli@latest
```

Then verify: `mt5-backtester-pp-cli --version`. Ensure `$(go env GOPATH)\bin` (Windows: `%USERPROFILE%\go\bin`) is on `PATH`.

Set `MT5_PATH` once so the CLI doesn't need `--terminal` on every call:

```powershell
[System.Environment]::SetEnvironmentVariable("MT5_PATH","C:\Program Files\MetaTrader 5","User")
```

Do not proceed with skill commands until `mt5-backtester-pp-cli config` reports the resolved terminal path.

## Requirements

- **Windows** (MetaTrader 5 only runs on Windows).
- **MetaTrader 5** installed with at least one broker account logged in (demo accounts work).
- **Go 1.22+** to build the CLI.
- MT5 terminal must have history downloaded for the symbols being tested (open a chart on each symbol once).

## When to Use This CLI

Reach for this when the user wants to:

- Run a single MT5 strategy-tester backtest from the shell.
- Compile a `.mq5` source file via `metaeditor64.exe`.
- Parse an existing MT5 strategy-tester HTML report.
- Run a batch of backtests from a JSON file (defaults + per-job overrides).
- Inspect or generate a `.set` file (with or without optimization ranges).
- Manage multiple MT5 terminal installs via named profiles.

Don't reach for this for **live trading, order placement, account management, or parameter optimization sweeps** — those need a different tool (a Python `MetaTrader5`-based CLI is the right home for them).

## Commands

### `mt5-backtester-pp-cli run` — Single backtest

```
mt5-backtester-pp-cli run --ea <name|path> [--symbol EURUSD] [--period H1]
                      [--from YYYY.MM.DD] [--to YYYY.MM.DD]
                      [--model 0|1|2|4] [--deposit 10000] [--currency USD] [--leverage 100]
                      [--set FILE] [--input key=value ...]
                      [--timeout 4h] [--no-shutdown] [-o text|json|csv]
```

Tick models: `0` Every Tick (most accurate, slowest), `1` 1-Minute OHLC (default), `2` Open Prices Only (fastest), `4` Real Ticks.

`--set FILE` loads inputs from a `.set` file (resolved under `MQL5\Profiles\Tester\` if not absolute). `--input KEY=VAL` overrides individual values; optimization ranges from the original `.set` are preserved.

### `mt5-backtester-pp-cli compile` — Compile MQL5

```
mt5-backtester-pp-cli compile <file.mq5> [--inc <include-dir> ...]
```

### `mt5-backtester-pp-cli report` — Parse a tester report

```
mt5-backtester-pp-cli report <file.htm> [-o text|json|csv]
mt5-backtester-pp-cli report --dir "<tester-dir>"   # most recent report in dir
```

Reports normally land in `%APPDATA%\MetaQuotes\Terminal\<hash>\tester\`.

### `mt5-backtester-pp-cli batch` — Multiple backtests from JSON

```
mt5-backtester-pp-cli template                      # write batch.json starter
mt5-backtester-pp-cli batch --file batch.json [--timeout 4h] [-o text|json|csv]
```

Each job accepts `ea`, `symbol`, `period`, `from`, `to`, `model`, `deposit`, `currency`, `leverage`, `profile`, `inputs`, `set_file`, `label`. `defaults` apply to all jobs; per-job fields override.

### `mt5-backtester-pp-cli setfile` — `.set` files

```
mt5-backtester-pp-cli setfile show <file.set>     # pretty-print with ranges
mt5-backtester-pp-cli setfile export --output FILE --input KEY=VAL [...]
mt5-backtester-pp-cli setfile list                # all .set under Profiles\Tester
```

### `mt5-backtester-pp-cli profile` — Named terminal profiles

```
mt5-backtester-pp-cli profile add  --name <n> --terminal <path> [--portable] [--default]
mt5-backtester-pp-cli profile use  <name>      # set default
mt5-backtester-pp-cli profile list
mt5-backtester-pp-cli profile remove <name>
```

### `mt5-backtester-pp-cli config` — Show resolved paths

## How it works

MT5 supports a headless tester mode via `.ini` files:

```
terminal64.exe /config:backtest.ini
```

`mt5-backtester-pp-cli` generates the ini (including a `[TesterInputs]` block built from `.set` files or `--input` flags), launches the terminal, waits for `ShutdownTerminal=1` to exit, then finds and parses the HTML report MT5 writes.

## Environment

| Variable    | Description                                                  |
|-------------|--------------------------------------------------------------|
| `MT5_PATH`  | MT5 install directory (e.g. `C:\Program Files\MetaTrader 5`) |

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → `mt5-backtester-pp-cli --help`.
2. **`install`** → `go install ...@latest`.
3. **Anything else** → map user intent to one of `run | compile | report | batch | template | setfile | profile | config | service` and pass through flags. Always verify the binary is present first; if not, offer to install.

See `README.md` in this directory for the full reference, set file format details, and roadmap.
