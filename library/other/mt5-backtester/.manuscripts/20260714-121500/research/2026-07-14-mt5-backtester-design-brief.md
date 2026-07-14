# mt5-backtester — Design Brief (hand-build research)

**Printed:** hand-built by ek-labs, packaged for the printing-press-library catalog on 2026-07-14 (run `20260714-121500`). The upstream surface is MetaTrader 5's Strategy Tester, driven via `terminal64.exe` config-file conventions and `metaeditor64.exe` — no API spec exists (`spec_format: internal`).

## Problem

MT5's Strategy Tester is GUI-only by design. Running one backtest means clicking through the terminal; running a parameter sweep across symbols and timeframes means an afternoon of clicking. The terminal does, however, honor an undocumented-but-stable headless convention: launch `terminal64.exe /config:<file.ini>` with a `[Tester]` section and `ShutdownTerminal=1`, and it runs the test, writes an HTML report, and exits.

## Architecture decisions

1. **INI generation + subprocess orchestration.** `run` builds the `[Tester]` ini (EA, symbol, period, dates, model, deposit, optional `.set` inputs), launches the terminal, waits for clean exit, then locates and parses the HTML report. No UI automation, no injected DLLs.
2. **Report parsing to text/JSON/CSV.** The HTML strategy-tester report is scraped into structured metrics: net profit, profit factor, Sharpe, drawdown, win rate, trade count. `report` also works standalone on any existing report file.
3. **Batch grids.** `batch --file batch.json` expands defaults × EAs × symbols × periods into sequential runs with ranked output and a top-performer summary. `template` writes a starter file.
4. **`.set` files as first-class.** MT5 `.set` files carry optimization ranges (`value||start||step||stop||Y/N`). `setfile show|export|list` parses and writes them preserving ranges, and `run --input KEY=VAL` overrides single inputs without destroying the range metadata.
5. **Named terminal profiles.** Multiple brokers/portable installs are managed via `profile add|use|list` (JSON store) and `--profile` per run; `service` wraps NSSM for always-on terminals.
6. **Windows-only, honestly.** The tool shells Windows executables; no pretense of cross-platform support. goreleaser targets windows_amd64/arm64 only.

## Novel features (validated in the shipped tree)

Headless backtest runner · batch test grid · `.set` file support with range preservation · named terminal profiles · MQL5 compiler wrapper · HTML report parser.

## Relationship to `library/other/mt5`

Sibling CLI, zero overlap: this tool never connects to an account, never sends orders, and uses file/subprocess transport; the `mt5` CLI does live account operations over a Python bridge and never drives the Strategy Tester.
