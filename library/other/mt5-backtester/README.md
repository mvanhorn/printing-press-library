# mt5-backtester-pp-cli · MT5 Backtester CLI

> Printed by Printing Press — printingpress.dev

Run MetaTrader 5 Expert Advisor backtests from the command line.
Compile MQL5 source files. Parse and compare reports. Batch test across symbols and timeframes.

---

## Install

### Windows (PowerShell) — recommended

```powershell
# 1. Install Go if not present
winget install GoLang.Go

# 2. Clone and build
git clone https://github.com/mvanhorn/printing-press-library
cd printing-press-library\library\finance\mt5-backtester
go install ./cmd/mt5-backtester-pp-cli

# 3. Set your MT5 path once
$env:MT5_PATH = "C:\Program Files\MetaTrader 5"
[System.Environment]::SetEnvironmentVariable("MT5_PATH","C:\Program Files\MetaTrader 5","User")
```

### Build from source

```powershell
git clone https://github.com/mvanhorn/printing-press-library
cd printing-press-library\library\finance\mt5-backtester
go build -o mt5-backtester-pp-cli.exe ./cmd/mt5-backtester-pp-cli
```

---

## Requirements

- **Windows** (MetaTrader 5 only runs on Windows)
- **MetaTrader 5** installed with at least one broker account logged in
- **Go 1.22+** (only needed to build)
- MT5 terminal must have history downloaded for the symbols you test

---

## Quick Start

```powershell
# Check that mt5-backtester-pp-cli finds your terminal
mt5-backtester-pp-cli config

# Run a backtest on the built-in MACD Sample EA
mt5-backtester-pp-cli run --ea "MACD Sample" --symbol EURUSD --period H1 --from 2023.01.01 --to 2024.01.01

# Compile your EA first, then run it
mt5-backtester-pp-cli compile C:\MT5\MQL5\Experts\MyEA.mq5
mt5-backtester-pp-cli run --ea MyEA --symbol EURUSD --period M15 --model 0

# Parse a report file you already have
mt5-backtester-pp-cli report C:\Users\You\AppData\Roaming\MetaQuotes\Terminal\<hash>\tester\MyReport.htm

# Batch test across multiple symbols
mt5-backtester-pp-cli template                  # creates batch.json
mt5-backtester-pp-cli batch --file batch.json
```

---

## Commands

### `mt5-backtester-pp-cli run` — Run a backtest

```
mt5-backtester-pp-cli run [flags]

Flags:
  --ea string          EA name or path (required)
                       e.g. "MACD Sample", "MyEA", "Experts\MyEA.ex5"
  --symbol string      Trading symbol (default: EURUSD)
  --period string      Timeframe: M1 M5 M15 M30 H1 H4 D1 W1 (default: H1)
  --from string        Start date: YYYY.MM.DD (default: 1 year ago)
  --to string          End date:   YYYY.MM.DD (default: today)
  --model int          Tick model:
                         0 = Every Tick      (most accurate, slowest)
                         1 = 1 Minute OHLC   (good balance, default)
                         2 = Open Prices Only (fastest)
                         4 = Real Ticks       (most realistic)
  --deposit float      Initial deposit (default: 10000)
  --currency string    Deposit currency (default: USD)
  --leverage int       Account leverage (default: 100)
  --input key=value    EA input parameters (repeatable, override set file)
  --set string         .set file path or bare filename (see "Set files" below)
  --timeout duration   Max wait time (default: 4h)
  --no-shutdown        Keep MT5 open after test finishes
  -o, --output string  Output format: text, json, csv (default: text)
```

**Examples:**
```powershell
# Basic
mt5-backtester-pp-cli run --ea "MACD Sample" --symbol EURUSD --period H1 --from 2022.01.01 --to 2024.01.01

# Every tick model, custom deposit
mt5-backtester-pp-cli run --ea MyScalper --symbol GBPUSD --period M5 --model 0 --deposit 50000

# With EA inputs
mt5-backtester-pp-cli run --ea MyEA --symbol EURUSD --period H4 --input StopLoss=50 --input TakeProfit=150

# Drive inputs from a .set file (saved out of the MT5 strategy tester)
mt5-backtester-pp-cli run --ea MyEA --symbol EURUSD --set MyEA.set

# Set file + per-run override (override wins; rest of set file still applied)
mt5-backtester-pp-cli run --ea MyEA --set MyEA.set --input MagicNumber=99002

# JSON output for scripting
mt5-backtester-pp-cli run --ea "MACD Sample" --symbol EURUSD --period H1 --from 2023.01.01 --to 2024.01.01 -o json
```

---

### `mt5-backtester-pp-cli compile` — Compile MQL5 source

```
mt5-backtester-pp-cli compile <file.mq5> [flags]

Flags:
  --inc string   Additional include directory (repeatable)
```

**Examples:**
```powershell
mt5-backtester-pp-cli compile MyEA.mq5
mt5-backtester-pp-cli compile "C:\MT5\MQL5\Experts\MyEA.mq5" --inc "C:\MT5\MQL5\Include\MyLib"
```

---

### `mt5-backtester-pp-cli report` — Parse a report

```
mt5-backtester-pp-cli report [file.htm] [flags]

Flags:
  --dir string   Directory to search for most recent report
  -o string      Output format: text, json, csv
```

**Examples:**
```powershell
mt5-backtester-pp-cli report C:\path\to\report.htm
mt5-backtester-pp-cli report --dir "C:\Users\You\AppData\Roaming\MetaQuotes\Terminal\<hash>\tester"
mt5-backtester-pp-cli report report.htm -o json
mt5-backtester-pp-cli report report.htm -o csv >> results.csv
```

---

### `mt5-backtester-pp-cli batch` — Run multiple backtests

```
mt5-backtester-pp-cli batch --file batch.json [flags]

Flags:
  --file string      Path to batch JSON file (required)
  --timeout duration Timeout per job (default: 4h)
  -o string          Output format: text, json, csv
```

**batch.json format:**
```json
{
  "jobs": [
    {
      "ea": "MACD Sample",
      "symbol": "EURUSD",
      "period": "H1",
      "from": "2023.01.01",
      "to": "2024.01.01",
      "model": 1,
      "deposit": 10000
    },
    {
      "ea": "MACD Sample",
      "symbol": "GBPUSD",
      "period": "H1",
      "from": "2023.01.01",
      "to": "2024.01.01"
    },
    {
      "ea": "MyEA",
      "symbol": "EURUSD",
      "period": "M15",
      "model": 0,
      "deposit": 50000,
      "inputs": {
        "StopLoss": "50",
        "TakeProfit": "100",
        "MagicNumber": "12345"
      }
    },
    {
      "ea": "MyEA",
      "symbol": "EURUSD",
      "set_file": "MyEA.set"
    },
    {
      "ea": "MyEA",
      "symbol": "GBPUSD",
      "set_file": "MyEA.set",
      "inputs": { "MagicNumber": "99002" }
    }
  ]
}
```

Per-job fields:

| Field      | Purpose                                                                 |
|------------|-------------------------------------------------------------------------|
| `set_file` | Resolve a `.set` file (see "Set files" below) and emit `[TesterInputs]` |
| `inputs`   | Override individual params; preserves optimization ranges from the file |

Either field can also go in `defaults` to apply across all jobs.

**Examples:**
```powershell
# Generate a template
mt5-backtester-pp-cli template

# Run the batch
mt5-backtester-pp-cli batch --file batch.json

# Save results as CSV
mt5-backtester-pp-cli batch --file batch.json -o csv > results.csv

# JSON for further processing
mt5-backtester-pp-cli batch --file batch.json -o json | python -m json.tool
```

---

### `mt5-backtester-pp-cli config` — Show detected paths

```powershell
mt5-backtester-pp-cli config
```

---

### `mt5-backtester-pp-cli setfile` — Inspect, generate, and list `.set` files

```
mt5-backtester-pp-cli setfile show <file.set>      # pretty-print params and optimization ranges
mt5-backtester-pp-cli setfile export --output FILE --input KEY=VAL [--input ...]
mt5-backtester-pp-cli setfile list                 # all .set files under MQL5\Profiles\Tester\
```

**Examples:**
```powershell
# Inspect a set file from MQL5\Profiles\Tester\
mt5-backtester-pp-cli setfile show MyEA.set

# Or by absolute path
mt5-backtester-pp-cli setfile show "C:\Users\You\AppData\Roaming\MetaQuotes\Terminal\<hash>\MQL5\Profiles\Tester\MyEA.set"

# Generate a starter .set from inputs (no optimization ranges)
mt5-backtester-pp-cli setfile export --output MyEA.set --input StopLoss=50 --input TakeProfit=100 --input MagicNumber=99001

# List every .set file MT5 can see
mt5-backtester-pp-cli setfile list
```

---

## Set files

A MetaTrader 5 `.set` file is the persisted form of the strategy tester's "Inputs" tab — one parameter per line, in MT5's own pipe-delimited format:

```
; lines starting with ';' are comments
StopLoss=50||50||1||200||N
TakeProfit=100||100||1||500||Y
MagicNumber=99001
UseFilter=true
```

| Field         | Meaning                                       |
|---------------|-----------------------------------------------|
| `value`       | The value used during a normal backtest run.  |
| `opt_start`   | Optimization sweep start.                     |
| `opt_step`    | Optimization sweep step.                      |
| `opt_stop`    | Optimization sweep stop.                      |
| `Y` / `N`     | Whether this param is included in the sweep.  |

Lines without `||` are simple values (no optimization range). Boolean literals (`true` / `false`) are passed through verbatim. Blank lines and `;` comments are skipped.

### How `mt5-backtester-pp-cli` uses set files

- `--set FILE` on `run` or `set_file` on a batch job parses the file and emits a proper `[TesterInputs]` block in the generated ini — pipes and Y/N flags preserved.
- `--input KEY=VALUE` (or per-job `inputs`) **overrides** values from the set file by name. Optimization ranges on the original line are preserved; only the value changes.
- If the path is not absolute, `mt5-backtester-pp-cli` resolves bare filenames against `MQL5\Profiles\Tester\` under every MT5 data folder on the host. With `--portable`, the terminal's own directory is used instead.

### Workflow

```powershell
# 1. In MT5 Strategy Tester: configure inputs, click "Save" → MyEA.set
# 2. Inspect
mt5-backtester-pp-cli setfile show MyEA.set

# 3. Run with the saved inputs
mt5-backtester-pp-cli run --ea MyEA --symbol EURUSD --set MyEA.set

# 4. Batch the same file across symbols, with one override per job
mt5-backtester-pp-cli batch --file batch.json
```

---

## How it works

MT5 supports a headless CLI mode via `.ini` configuration files:

```powershell
terminal64.exe /config:backtest.ini
```

`mt5-backtester-pp-cli` generates these ini files, launches the terminal, waits for completion (MT5 exits when `ShutdownTerminal=1`), then finds and parses the HTML report MT5 writes.

The generated ini looks like this:
```ini
[Tester]
Expert=Experts\MyEA.ex5
Symbol=EURUSD
Period=H1
Model=1
FromDate=2023.01.01
ToDate=2024.01.01
Deposit=10000.00
Currency=USD
Leverage=100
ShutdownTerminal=1
Report=MyEA_EURUSD_H1_20241201
ReplaceReport=1
```

---

## Environment variables

| Variable    | Description                                     |
|-------------|--------------------------------------------------|
| `MT5_PATH`  | MT5 install directory (e.g. `C:\Program Files\MetaTrader 5`) |

---

## Tips

- MT5 must be logged into a broker account for backtests to run (demo accounts work)
- Download history first: open charts for all symbols you want to test
- Every Tick model (`--model 0`) requires more history; start with 1 Minute OHLC (`--model 1`)
- The terminal's log files are in `%APPDATA%\MetaQuotes\Terminal\<hash>\logs\`
- Reports land in `%APPDATA%\MetaQuotes\Terminal\<hash>\tester\`

---

## Roadmap

- [ ] Auto-discover terminal data path (parse `origin.txt`)
- [ ] Optimization mode support with parameter ranges
- [ ] Watch mode (stream log lines during test)
- [ ] SQLite results store (compare runs over time)
- [ ] Walk-forward analysis helper
- [ ] MCP server surface

---

Printed by [Printing Press](https://printingpress.dev) — the agent-designed CLI factory.
