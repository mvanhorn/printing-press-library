# Yahoo Finance CLI

**Look up market data, track local portfolios and watchlists, filter options chains, and recover from Yahoo rate limits with a browser-session fallback.**

Yahoo Finance CLI wraps Yahoo's reverse-engineered finance endpoints in a shell-friendly, agent-friendly interface. It covers quotes, charts, fundamentals, screeners, recommendations, trending symbols, options chains, and symbol search, then adds a local SQLite layer for watchlists, portfolio lots, ad-hoc SQL, and workflows like morning digest and side-by-side comparison.

The important difference is not just endpoint coverage. It is that this CLI turns Yahoo Finance into a usable tool for terminal workflows, scripts, cron jobs, and agents.

## Why this CLI?

Most Yahoo Finance tools fall into one of three buckets:

| Tool type | Typical strength | Typical gap |
| --- | --- | --- |
| Library wrappers | Good endpoint coverage | Not a CLI, weak shell ergonomics, no persistent local state |
| Thin CLIs | Easy quote lookups | Usually quote-only, little or no options/fundamentals depth |
| MCP servers | Good for chat tools | Less useful for shell pipelines and recurring local workflows |

This CLI combines:

- broad Yahoo endpoint coverage
- agent-friendly output flags on every command
- local watchlists and portfolio lots in SQLite
- derived workflows like `digest`, `compare`, `sparkline`, `fx`, and filtered `options-chain`
- a Chrome-session import fallback when Yahoo blocks the automatic crumb bootstrap from your IP

## Install

### Go

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yahoo-finance-current).

## Session Bootstrap

Yahoo Finance has no official API key. The CLI bootstraps a Yahoo session automatically by:

1. visiting `fc.yahoo.com`
2. collecting the Yahoo cookies it needs
3. fetching a crumb from `/v1/test/getcrumb`
4. persisting that session to disk

For most users, this happens automatically on first live request.

### When to use `auth login-chrome`

If Yahoo is returning HTTP 429 from your IP and the automatic bootstrap cannot get a usable crumb, import a browser session instead:

```bash
# 1. Open finance.yahoo.com in Chrome and accept cookies.
# 2. Export cookies for *.yahoo.com as JSON.
# 3. Get a crumb in DevTools on finance.yahoo.com:
#    fetch('/v1/test/getcrumb').then(r => r.text()).then(console.log)
yahoo-finance-pp-cli auth login-chrome --cookies ~/yahoo-cookies.json --crumb abc123
```

Check the current state with:

```bash
yahoo-finance-pp-cli auth status
yahoo-finance-pp-cli doctor
```

## Quick Start

```bash
# Verify config, cached session status, and the live Yahoo handshake
yahoo-finance-pp-cli doctor

# Current quotes
yahoo-finance-pp-cli quote --symbols AAPL,MSFT,NVDA

# Deep summary for one symbol
yahoo-finance-pp-cli quote summary AAPL

# Build a watchlist and check it daily
yahoo-finance-pp-cli watchlist create tech
yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG
yahoo-finance-pp-cli digest --watchlist tech

# Track a portfolio with cost basis
yahoo-finance-pp-cli portfolio add AAPL 50 185.50 --purchased 2024-06-15
yahoo-finance-pp-cli portfolio perf

# Filter the options chain to what you actually care about
yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --type calls
```

## Unique Features

These are the workflows that differentiate this CLI from a thin Yahoo wrapper.

### `watchlist`

Named watchlists live in the local SQLite database and can be reused across commands.

```bash
yahoo-finance-pp-cli watchlist create tech
yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG
yahoo-finance-pp-cli watchlist show tech
```

### `portfolio`

Track purchase lots locally and compute live unrealized performance from Yahoo quotes.

```bash
yahoo-finance-pp-cli portfolio add AAPL 50 185.50 --purchased 2024-06-15
yahoo-finance-pp-cli portfolio perf
yahoo-finance-pp-cli portfolio gains
```

### `digest`

Turn a saved watchlist into a morning market snapshot with top gainers and losers.

```bash
yahoo-finance-pp-cli digest --watchlist tech
```

### `compare`

Compare multiple symbols in one normalized table instead of hopping across quote pages.

```bash
yahoo-finance-pp-cli compare AAPL MSFT NVDA GOOG
```

### `sparkline`

Render a compact terminal sparkline for recent price action.

```bash
yahoo-finance-pp-cli sparkline AAPL --range 3mo
```

### `sql`

Run raw SQL against the local SQLite database for custom analysis.

```bash
yahoo-finance-pp-cli sql "SELECT watchlist, COUNT(*) AS members FROM watchlist_members GROUP BY watchlist"
```

### `fx`

Use Yahoo's FX pairs as a simple currency converter.

```bash
yahoo-finance-pp-cli fx USD EUR --amount 100
```

### `options-chain`

Filter a raw options chain by moneyness, DTE, and strike range.

```bash
yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --type calls
```

### `auth login-chrome`

Import a live browser session if Yahoo blocks the automatic crumb bootstrap from the current IP.

```bash
yahoo-finance-pp-cli auth login-chrome --cookies ~/yahoo-cookies.json --crumb abc123
```

## Commands

### Core market data

| Command | Description |
| --- | --- |
| `quote --symbols AAPL,MSFT` | Current quotes for one or more symbols |
| `quote summary AAPL` | Deep quote summary with modules like price and financial data |
| `chart AAPL` | Historical chart data |
| `fundamentals AAPL` | Fundamentals time series |
| `insights --symbol AAPL` | Technical events, valuation, and research reports |
| `options AAPL` | Raw options chain |
| `recommendations AAPL` | Symbols with shared analyst recommendations |
| `screener --scr-ids day_gainers` | Predefined Yahoo screener |
| `trending US` | Trending symbols for a region |
| `search "apple"` | Yahoo symbol/news/fund search |
| `autocomplete --query appl` | Fast symbol/company autocomplete |

### Local-state workflows

| Command | Description |
| --- | --- |
| `watchlist create|add|remove|list|show|delete` | Local named ticker collections |
| `portfolio add|list|remove|perf|gains` | Local lot tracking and P&L |
| `digest` | Morning watchlist briefing |
| `compare` | Multi-symbol comparison |
| `sparkline` | Terminal sparkline |
| `sql` | Raw SQL over the local DB |
| `fx` | Currency conversion |
| `options-chain` | Filtered options chain |

### Data and utilities

| Command | Description |
| --- | --- |
| `sync` | Sync Yahoo data into local SQLite |
| `search <query> --data-source local` | Search synced local data |
| `workflow archive` | Sync all resources for offline access |
| `workflow status` | Show local archive status |
| `export` | Export API data to JSONL or JSON |
| `import` | Import JSONL via API create/upsert calls |
| `api` | Browse raw API interface coverage |
| `doctor` | Verify config, session state, and live Yahoo handshake |
| `auth status` | Show cached Yahoo session status |
| `auth logout` | Clear cached Yahoo session |
| `auth login-chrome` | Import a browser session |

## Output Formats

```bash
# Human-readable output in a terminal
yahoo-finance-pp-cli quote --symbols AAPL,MSFT,NVDA

# JSON for scripts and agents
yahoo-finance-pp-cli quote --symbols AAPL --json

# Select only the fields you want
yahoo-finance-pp-cli quote --symbols AAPL --json --select symbol,regularMarketPrice,regularMarketChangePercent

# CSV output
yahoo-finance-pp-cli quote --symbols AAPL,MSFT --csv

# Dry run
yahoo-finance-pp-cli quote --symbols AAPL --dry-run

# Agent mode
yahoo-finance-pp-cli quote --symbols AAPL --agent
```

## Agent Usage

This CLI behaves well in scripts and agent environments:

- `--json` for machine-readable output
- `--compact` for lower-token payloads
- `--select` to keep only required fields
- `--dry-run` to preview the request shape
- `--no-cache` to bypass the 5-minute GET cache
- `--data-source auto|live|local` to make source choice explicit
- `--agent` to bundle `--json --compact --no-input --no-color --yes`

Exit codes: `0` success, `2` usage error, `3` not found, `4` session/auth-style error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This project also ships a companion MCP server.

### Claude Code

```bash
claude mcp add yahoo-finance yahoo-finance-pp-mcp
```

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "yahoo-finance": {
      "command": "yahoo-finance-pp-mcp"
    }
  }
}
```

## Health Check

```bash
yahoo-finance-pp-cli doctor
```

`doctor` verifies:

- config loading
- whether a cached session file is present
- whether the live Yahoo crumb/session handshake succeeds from this machine
- version

## Configuration

Config file:

```text
~/.config/yahoo-finance-pp-cli/config.toml
```

Cached Yahoo session:

```text
~/.config/yahoo-finance-pp-cli/session.json
```

Environment variables:

- `YAHOO_FINANCE_CONFIG`
- `YAHOO_FINANCE_BASE_URL`

## Troubleshooting

**`doctor` reports `API: rate limited`**

- Yahoo is likely blocking this IP
- import a browser session with `auth login-chrome`
- re-run `doctor`

**`quote` says `required flag "symbols" not set`**

- use `quote --symbols AAPL,MSFT`, not positional symbols

**`search --data-source local` returns nothing**

- run `yahoo-finance-pp-cli sync` first
- confirm the local DB exists

**`auth status` says no session is cached**

- that is not automatically an error
- the CLI will try to bootstrap a Yahoo session on the next live request
- if that bootstrap fails with 429, use `auth login-chrome`

**You want to clear a bad cached session**

```bash
yahoo-finance-pp-cli auth logout
```

## Sources & Inspiration

- [yfinance](https://github.com/ranaroussi/yfinance)
- [yahoo-finance2](https://github.com/gadicc/yahoo-finance2)
- [yahooquery](https://github.com/dpguthrie/yahooquery)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `yahoo-finance-pp-cli profile save <name> --<flag> <value>`, then `yahoo-finance-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.yahoo-finance-pp-cli/feedback.jsonl`. When `YAHOO_FINANCE_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `YAHOO_FINANCE_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
