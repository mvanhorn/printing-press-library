---
name: pp-yahoo-finance
description: "Every Yahoo Finance endpoint plus a SQLite portfolio, covered-call screener Trigger phrases: `quote AAPL`, `options on TSLA`, `what's my portfolio doing`, `dividend income this year`, `screen for value stocks`, `daily market briefing`, `use yahoo-finance`, `run yahoo-finance`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - yahoo-finance-pp-cli
    install:
      - kind: go
        bins: [yahoo-finance-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/yahoo-finance/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Yahoo Finance — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `yahoo-finance-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install yahoo-finance --cli-only
   ```
2. Verify: `yahoo-finance-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an agent needs Yahoo Finance market data with deterministic JSON output, a local store that compounds across calls (portfolio, watchlist, history cache), or a working session from a rate-limited host. It is the only Yahoo Finance tool that joins your cost basis with live data and survives a 429 via a Chrome cookie fallback.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`portfolio perf`** — Track YTD / 1Y / all-time returns on your holdings with cost basis, dividends, and current price baked in.

  _Reach for this when an agent needs to report 'how is my user's portfolio doing this year' — no single Yahoo Finance endpoint encodes cost basis._

  ```bash
  yahoo-finance-pp-cli portfolio perf --agent
  ```
- **`digest`** — One command: overnight news + biggest movers + earnings today + ex-div dates for everything on your watchlist.

  _Use as a single agent call for a 'daily market briefing' across a named set of tickers — replaces 4+ separate endpoint calls._

  ```bash
  yahoo-finance-pp-cli digest --watchlist tech --agent
  ```
- **`portfolio dividends`** — See total dividend income for the year, per-holding breakdown, and yield on cost.

  _Reach for this when the user asks 'how much have I earned in dividends this year' or 'what's my real yield on my JNJ position'._

  ```bash
  yahoo-finance-pp-cli portfolio dividends --year 2026 --agent
  ```
- **`insiders-net-buying`** — Surface companies where insiders are net buyers in the last N days, filtered to your watchlist.

  _Use when an agent needs a 'who's insiders actually buying' signal layer across the user's watchlist._

  ```bash
  yahoo-finance-pp-cli insiders-net-buying --recent 30d --watchlist tech --agent
  ```

### Agent-native compute
- **`options-chain`** — Filter an options chain to ATM/OTM/ITM contracts within a days-to-expiration window.

  _Use when an agent needs to surface 'OTM weekly puts on AAPL near earnings' without parsing a thousand-row chain._

  ```bash
  yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --agent
  ```
- **`screen-local`** — Run arbitrary P/E, P/B, yield, margin, and growth filters against the data you've synced locally.

  _Use to compose custom value/growth screens an agent can apply to a synced universe, far beyond Yahoo's 12 canned screens._

  ```bash
  yahoo-finance-pp-cli screen-local --pe-max 15 --roe-min 0.15 --agent
  ```
- **`compare`** — Multi-symbol price-plus-dividend total return ranked over a range.

  _Use when comparing actual holding-period returns rather than price-only deltas — the only Yahoo CLI that does this offline._

  ```bash
  yahoo-finance-pp-cli compare AAPL MSFT NVDA --range 1y --include-divs --agent
  ```
- **`options-covered-calls`** — Scan your holdings for covered-call candidates by annualized yield and DTE.

  _Use to surface 'wheel-strategy' covered-call candidates from the user's actual stock positions._

  ```bash
  yahoo-finance-pp-cli options-covered-calls --min-yield-annualized 0.10 --max-dte 45 --agent
  ```
- **`watchlist correlate`** — Pairwise Pearson correlation across the symbols in a named watchlist over a date range.

  _Use when an agent needs to flag concentration risk inside a watchlist — 'is this 'tech' watchlist actually 80% AAPL exposure'._

  ```bash
  yahoo-finance-pp-cli watchlist correlate tech --range 6m --agent
  ```

### Reachability mitigation
- **`auth login --chrome`** — Import your live Chrome session cookies when Yahoo's crumb handshake is blocked from your IP.

  _Reach for this when an agent's `doctor` reports 429 from the host's IP — Chrome's session cookies unblock the crumb handshake._

  ```bash
  yahoo-finance-pp-cli auth login --chrome
  ```

## Command Reference

**autocomplete** — Legacy autocomplete (faster than search)

- `yahoo-finance-pp-cli autocomplete` — Autocomplete symbols and company names

**chart** — Historical OHLCV price data

- `yahoo-finance-pp-cli chart <symbol>` — Historical price chart data for a symbol

**fundamentals** — Time series of fundamentals (quarterly/annual)

- `yahoo-finance-pp-cli fundamentals <symbol>` — Fundamentals time series (EPS, revenue, margin, cash flow, etc.)

**insights** — Company insights, valuation, and technical events

- `yahoo-finance-pp-cli insights` — Insights for a symbol: technical events, valuation, research reports

**lookup** — Symbol search and lookup

- `yahoo-finance-pp-cli lookup` — Search for symbols, news, and funds matching a query

**options** — Options chains for equities and ETFs

- `yahoo-finance-pp-cli options <symbol>` — Options chain for a symbol (calls and puts)

**quote** — Real-time quotes and quote summaries

- `yahoo-finance-pp-cli quote list` — Get current quotes for one or more symbols
- `yahoo-finance-pp-cli quote summary` — Deep quote summary including price, fundamentals, ownership, and filings

**recommendations** — Symbols related by analyst recommendation

- `yahoo-finance-pp-cli recommendations <symbol>` — Symbols that share recommendations with the given symbol

**screener** — Predefined and custom stock screeners

- `yahoo-finance-pp-cli screener` — Run a predefined screener by ID

**trending** — Trending symbols by region

- `yahoo-finance-pp-cli trending <region>` — Top trending symbols in a region right now


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
yahoo-finance-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Daily market briefing across your watchlist

```bash
yahoo-finance-pp-cli digest --watchlist tech --agent
```

Single command returns overnight news, biggest movers, earnings today, and ex-div dates for the symbols in the named watchlist.

### Year-to-date portfolio performance

```bash
yahoo-finance-pp-cli portfolio perf --agent --select symbol,unrealized_pl,total_return_pct
```

Joins cost basis with live quotes and reinvested dividends. The `--select` narrows the payload so an agent doesn't burn context on every row.

### Find OTM puts on AAPL near 30 DTE

```bash
yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --type puts --agent
```

Filters the full chain client-side by moneyness and DTE; Yahoo's endpoint doesn't filter.

### Custom SQL screen for cheap large caps

```bash
yahoo-finance-pp-cli screen-local --pe-max 15 --roe-min 0.15 --market-cap-min 10000000000 --agent
```

Runs against locally-synced fundamentals; Yahoo's remote screener has only 12 predefined IDs.

### Raw SQL over the local store

```bash
yahoo-finance-pp-cli sql "SELECT symbol, ROUND(AVG(close), 2) AS avg_close FROM history WHERE date > date('now','-30 days') GROUP BY symbol ORDER BY avg_close DESC LIMIT 10"
```

Demonstrates the SQLite path; useful when an agent needs aggregations that no canned command covers.

## Auth Setup

Yahoo Finance has no API key — it requires a crumb+cookie handshake. The CLI auto-fetches the crumb on first call and persists cookies to `~/.config/yahoo-finance-pp-cli/`. If your IP is rate-limited (cloud hosts and many international IPs are), run `auth login --chrome` to import a logged-in Chrome session so the crumb dance succeeds.

Run `yahoo-finance-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  yahoo-finance-pp-cli autocomplete --query example-value --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
yahoo-finance-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
yahoo-finance-pp-cli feedback --stdin < notes.txt
yahoo-finance-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/yahoo-finance-pp-cli/feedback.jsonl`. They are never POSTed unless `YAHOO_FINANCE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `YAHOO_FINANCE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
yahoo-finance-pp-cli profile save briefing --json
yahoo-finance-pp-cli --profile briefing autocomplete --query example-value
yahoo-finance-pp-cli profile list --json
yahoo-finance-pp-cli profile show briefing
yahoo-finance-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `yahoo-finance-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add yahoo-finance-pp-mcp -- yahoo-finance-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which yahoo-finance-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   yahoo-finance-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `yahoo-finance-pp-cli <command> --help`.
