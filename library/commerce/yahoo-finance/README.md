# Yahoo Finance CLI

**Every Yahoo Finance endpoint plus a SQLite portfolio, covered-call screener, and Chrome-cookie fallback the libraries don't ship.**

yahoo-finance-pp-cli matches yfinance and yahoo-finance2 for raw endpoint coverage, then adds the things only a local store + agent-shaped CLI can do: cost-basis-aware portfolio performance, dividend income with yield-on-cost, a SQL-backed fundamentals screener, options moneyness + DTE filtering, and an `auth login --chrome` cookie escape hatch when Yahoo blocks your IP.

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `yahoo-finance-pp-cli` binary and the `pp-yahoo-finance` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install yahoo-finance
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-finance --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-finance --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-finance --agent claude-code
npx -y @mvanhorn/printing-press-library install yahoo-finance --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yahoo-finance-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install yahoo-finance --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-yahoo-finance --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-yahoo-finance --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install yahoo-finance --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/yahoo-finance-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "yahoo-finance": {
      "command": "yahoo-finance-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Yahoo Finance has no API key — it requires a crumb+cookie handshake. The CLI auto-fetches the crumb on first call and persists cookies to `~/.config/yahoo-finance-pp-cli/`. If your IP is rate-limited (cloud hosts and many international IPs are), run `auth login --chrome` to import a logged-in Chrome session so the crumb dance succeeds.

## Quick Start

```bash
# Confirms the crumb handshake works from your IP.
yahoo-finance-pp-cli doctor

# Three live quotes, JSON-shaped, agent-narrow.
yahoo-finance-pp-cli quote list --symbols AAPL,MSFT,NVDA --json --select symbol,regularMarketPrice,regularMarketChangePercent

# Pulls daily chart for AAPL; auto-caches into the local SQLite store.
yahoo-finance-pp-cli chart AAPL --interval 1d --json

# Cross-endpoint daily briefing for a named watchlist — the killer transcendence command.
yahoo-finance-pp-cli digest --watchlist tech --agent

# Compose raw SQL against synced data — uniqueness of the local store.
yahoo-finance-pp-cli sql "SELECT symbol, MAX(close) FROM history GROUP BY symbol"

```

## Unique Features

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

## Usage

Run `yahoo-finance-pp-cli --help` for the full command reference and flag list.

## Commands

### autocomplete

Legacy autocomplete (faster than search)

- **`yahoo-finance-pp-cli autocomplete`** - Autocomplete symbols and company names

### chart

Historical OHLCV price data

- **`yahoo-finance-pp-cli chart <symbol>`** - Historical price chart data for a symbol

### fundamentals

Time series of fundamentals (quarterly/annual)

- **`yahoo-finance-pp-cli fundamentals <symbol>`** - Fundamentals time series (EPS, revenue, margin, cash flow, etc.)

### insights

Company insights, valuation, and technical events

- **`yahoo-finance-pp-cli insights`** - Insights for a symbol: technical events, valuation, research reports

### lookup

Symbol search and lookup

- **`yahoo-finance-pp-cli lookup`** - Search for symbols, news, and funds matching a query

### options

Options chains for equities and ETFs

- **`yahoo-finance-pp-cli options <symbol>`** - Options chain for a symbol (calls and puts)

### quote

Real-time quotes and quote summaries

- **`yahoo-finance-pp-cli quote list`** - Get current quotes for one or more symbols
- **`yahoo-finance-pp-cli quote summary`** - Deep quote summary including price, fundamentals, ownership, and filings

### recommendations

Symbols related by analyst recommendation

- **`yahoo-finance-pp-cli recommendations <symbol>`** - Symbols that share recommendations with the given symbol

### screener

Predefined and custom stock screeners

- **`yahoo-finance-pp-cli screener`** - Run a predefined screener by ID

### trending

Trending symbols by region

- **`yahoo-finance-pp-cli trending <region>`** - Top trending symbols in a region right now

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
yahoo-finance-pp-cli autocomplete --query example-value

# JSON for scripting and agents
yahoo-finance-pp-cli autocomplete --query example-value --json

# Filter to specific fields
yahoo-finance-pp-cli autocomplete --query example-value --json --select id,name,status

# Dry run — show the request without sending
yahoo-finance-pp-cli autocomplete --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
yahoo-finance-pp-cli autocomplete --query example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
yahoo-finance-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/yahoo-finance-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 from query1.finance.yahoo.com** — Run `yahoo-finance-pp-cli auth login --chrome` to import a logged-in Chrome session, then retry.
- **Crumb appears in error responses but request still fails** — Delete the cached session: `rm ~/.config/yahoo-finance-pp-cli/session.json` and let the next call re-bootstrap.
- **Quotes look stale** — Free tier is delayed up to 15 minutes; `--extended-hours` does not change that. Use a paid provider for sub-minute data.
- **`portfolio perf` shows zero positions** — Seed your lots first: `yahoo-finance-pp-cli portfolio add AAPL 100 --purchase-date 2025-01-15 --cost-basis 175.00`.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**yfinance**](https://github.com/ranaroussi/yfinance) — Python (14000 stars)
- [**yahoo-finance2**](https://github.com/gadicc/yahoo-finance2) — JavaScript (2800 stars)
- [**yahooquery**](https://github.com/dpguthrie/yahooquery) — Python (1000 stars)
- [**Alex2Yang97/yahoo-finance-mcp**](https://github.com/Alex2Yang97/yahoo-finance-mcp) — Python (262 stars)
- [**kanishka-namdeo/yfnhanced-mcp**](https://github.com/kanishka-namdeo/yfnhanced-mcp) — Python
- [**BillGatesCat/yf**](https://github.com/BillGatesCat/yf) — Go
- [**tabrindle/yahoo-finance-cli**](https://github.com/tabrindle/yahoo-finance-cli) — JavaScript
- [**scottjbarr/yahoofinance**](https://github.com/scottjbarr/yahoofinance) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
