---
name: pp-tradingview
description: "Resolve any ticker and see its price in USD and EUR in one command — stocks, crypto, and forex, no API key. Trigger phrases: `quote AAPL`, `price of bitcoin in euros`, `what is TSLA worth in EUR`, `convert USD to EUR`, `use tradingview`, `run tradingview`."
author: "jbriaux"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tradingview-pp-cli
    install:
      - kind: go
        bins: [tradingview-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/tradingview/cmd/tradingview-pp-cli
---

# TradingView — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tradingview-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tradingview --cli-only
   ```
2. Verify: `tradingview-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/tradingview/cmd/tradingview-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

TradingView CLI turns a symbol into an answer: search resolves a query to a fully-qualified EXCHANGE:TICKER, quote returns the last price in its native currency plus USD and EUR, and convert does fiat conversion using TradingView's own forex rates. Terminal-first, agent-native --json, no data-API subscription required for the public endpoints. A local watchlist tracks symbols and snapshots their USD/EUR prices to SQLite for offline querying and drift.

## When to Use This CLI

Use this CLI when you need a fast, scriptable answer to 'what is <symbol> worth' in USD or EUR, for stocks, crypto, or forex, without a paid market-data API key. It is ideal for agents that must normalize a quote to a common currency.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for placing trades or accessing a brokerage account — it is read-only market data.
- Do not use it for historical OHLC backtesting datasets — it returns current quotes, not time series.
- Do not use it for authenticated TradingView features like private watchlists or alerts — it targets public endpoints only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Multi-currency pricing
- **`quote`** — See any ticker's last price in its native currency, USD, and EUR in a single command.

  _Reach for this when an agent needs a symbol's value normalized to USD or EUR without wiring up a separate FX API._

  ```bash
  tradingview-pp-cli quote NASDAQ:AAPL --agent --select symbol,price_usd,price_eur
  ```
- **`convert`** — Convert an amount between currencies using TradingView's own live forex rates.

  _Use this to turn any USD figure into EUR (or between other majors) from the same source that priced the instrument._

  ```bash
  tradingview-pp-cli convert 100 USD EUR --agent
  ```

### Local state that compounds
- **`watchlist`** — Track a list of symbols locally and snapshot their USD/EUR quotes to SQLite for offline querying.

  _Use this to keep a personal set of symbols and their USD/EUR prices without re-resolving them each call._

  ```bash
  tradingview-pp-cli watchlist add AAPL BINANCE:BTCUSDT
  ```
- **`watchlist changes`** — Show how each watched symbol moved in USD between its two most recent snapshots.

  _Reach for this to see what moved since your last sync without diffing raw API responses yourself._

  ```bash
  tradingview-pp-cli watchlist changes --agent
  ```

## Command Reference

**market** — Fetch prices and quotes

- `tradingview-pp-cli market` — Get last price and currency for a fully-qualified EXCHANGE:TICKER

**symbols** — Search and resolve TradingView symbols

- `tradingview-pp-cli symbols` — Search symbols by text (returns exchange, type, currency)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tradingview-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Price in euros

```bash
tradingview-pp-cli quote NASDAQ:MSFT --agent --select symbol,price_eur
```

Return only the EUR-converted price for scripting.

### Find then quote

```bash
tradingview-pp-cli search bitcoin --type crypto
```

Resolve a name to a fully-qualified crypto ticker, then quote it.

### Currency conversion

```bash
tradingview-pp-cli convert 250 USD EUR --agent
```

Convert 250 USD to EUR using TradingView's EURUSD forex rate.

### Track a watchlist

```bash
tradingview-pp-cli watchlist add AAPL BINANCE:BTCUSDT
```

Add symbols, then 'watchlist sync' and 'watchlist quotes' for offline USD/EUR prices.

## Auth Setup

No authentication required.

Run `tradingview-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tradingview-pp-cli symbols --text AAPL --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `TRADINGVIEW_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TRADINGVIEW_CONFIG_DIR`, `TRADINGVIEW_DATA_DIR`, `TRADINGVIEW_STATE_DIR`, `TRADINGVIEW_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TRADINGVIEW_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `tradingview-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "tradingview": {
        "command": "tradingview-pp-mcp",
        "env": {
          "TRADINGVIEW_HOME": "/srv/tradingview"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TRADINGVIEW_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TRADINGVIEW_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tradingview-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tradingview-pp-cli feedback --stdin < notes.txt
tradingview-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TRADINGVIEW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRADINGVIEW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
tradingview-pp-cli profile save briefing --json
tradingview-pp-cli --profile briefing symbols --text AAPL
tradingview-pp-cli profile list --json
tradingview-pp-cli profile show briefing
tradingview-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `tradingview-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/tradingview/cmd/tradingview-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tradingview-pp-mcp -- tradingview-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tradingview-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tradingview-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tradingview-pp-cli <command> --help`.
