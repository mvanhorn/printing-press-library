# MT5 Backtester Printed CLI Agent Guide

This directory is the `mt5-backtester-pp-cli` CLI, published in the printing-press-library catalog. Unlike most catalog entries it was **hand-built** by ek-labs following printing-press conventions, not produced by the CLI Printing Press generator — see `.printing-press.json`, `NOTICE`, and `.manuscripts/` for provenance. There is no generator template to fix upstream: defects are fixed in place here and recorded in `.printing-press-patches.json`.

## Local Operating Contract

This is a **Windows-only** tool: it launches `terminal64.exe` (Strategy Tester) and `metaeditor64.exe` (MQL5 compiler) as subprocesses. Nothing works without a local MetaTrader 5 installation with at least one broker account logged in and history downloaded for the symbols under test.

Start by confirming the CLI can find the terminal:

```powershell
mt5-backtester-pp-cli config
```

If no terminal is detected, set `MT5_PATH` or add a named profile:

```powershell
mt5-backtester-pp-cli profile add --name broker1 --terminal "C:\Program Files\MetaTrader 5\terminal64.exe"
```

Core flows:

```powershell
mt5-backtester-pp-cli compile C:\MT5\MQL5\Experts\MyEA.mq5     # .mq5 -> .ex5
mt5-backtester-pp-cli run --ea MyEA --symbol EURUSD --period H1 --from 2023.01.01 --to 2024.01.01
mt5-backtester-pp-cli report <path-to-report.htm> -o json      # parse an existing report
mt5-backtester-pp-cli template && mt5-backtester-pp-cli batch --file batch.json
```

Use `-o json` or `-o csv` for machine-readable output when scripting.

## Operational cautions

- A backtest run launches the full MT5 terminal in shutdown-after-test mode. Runs take seconds to minutes depending on the date range and model; do not kill the process mid-run — the report only lands on clean exit.
- `run` writes a generated `.ini` into the terminal's config area and the terminal writes an HTML report; both are per-run artifacts, safe to regenerate.
- This tool never sends orders and never touches a live account; it only drives the Strategy Tester. For live trading and account analytics, use the sibling `library/other/mt5/` CLI.

For install, examples, batch schema, and .set-file semantics, read `README.md` and `SKILL.md`.

## Local Customizations

If you modify this CLI, record each customization in `.printing-press-patches.json` at this CLI's root (shape documented in the repo-root AGENTS.md): a short `id`, one-sentence `summary`, one-or-two-sentence `reason`, the touched `files`, and optionally a `validated_outcome`. Diffs live in git; the manifest is the index that survives tree replacement.
