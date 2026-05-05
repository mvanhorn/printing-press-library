---
name: pp-yahoo-finance
description: "Use Yahoo Finance CLI for stock and ETF quotes, charts, fundamentals, options chains, symbol search, trending tickers, local watchlists, portfolio lots, and market digests. Use when the user asks about a ticker, portfolio performance, option filtering, market movers, or wants Yahoo Finance data in a terminal or agent-friendly format."
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

# Yahoo Finance — Printing Press CLI

Yahoo Finance CLI wraps Yahoo market data in a terminal-first interface and adds a local SQLite layer for watchlists, portfolio lots, SQL queries, and derived workflows like `digest`, `compare`, `sparkline`, `fx`, and filtered `options-chain`.

It uses Yahoo's crumb/cookie session model automatically. No API key is required.

## When to Use This CLI

Use this CLI when the user asks about:

- stock, ETF, or fund quotes
- chart history or price ranges
- fundamentals or quote summary modules
- options chains or simple moneyness filtering
- trending symbols or predefined market screeners
- ticker search and autocomplete
- local watchlists
- portfolio cost basis and unrealized P&L

Do not use it when the user specifically needs:

- real-time streaming tick data
- exchange-grade paid data feeds
- broker/account actions like order entry

## Best Command Mapping

- "How is AAPL doing?" → `yahoo-finance-pp-cli quote --symbols AAPL --agent`
- "Give me a deeper view on Microsoft" → `yahoo-finance-pp-cli quote summary MSFT --agent`
- "Show me NVDA for the last year" → `yahoo-finance-pp-cli chart NVDA --range 1y --interval 1wk --agent`
- "What are the top gainers today?" → `yahoo-finance-pp-cli screener --scr-ids day_gainers --agent`
- "What is trending in the US?" → `yahoo-finance-pp-cli trending US --agent`
- "Track my portfolio" → `yahoo-finance-pp-cli portfolio perf --agent`
- "Compare AAPL, MSFT, and NVDA" → `yahoo-finance-pp-cli compare AAPL MSFT NVDA --agent`
- "Show me SPY options expiring soon" → `yahoo-finance-pp-cli options-chain SPY --max-dte 45 --agent`

## Unique Capabilities

### `watchlist`

Save named ticker groups locally for reuse across commands.

```bash
yahoo-finance-pp-cli watchlist create tech
yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG
```

### `portfolio`

Track local lots with purchase date and cost basis, then join them with live quotes.

```bash
yahoo-finance-pp-cli portfolio add AAPL 50 185.50 --purchased 2024-06-15
yahoo-finance-pp-cli portfolio perf --agent
yahoo-finance-pp-cli portfolio gains --agent
```

### `digest`

Summarize a watchlist into biggest gainers and losers.

```bash
yahoo-finance-pp-cli digest --watchlist tech --agent
```

### `compare`

Show a normalized multi-symbol comparison.

```bash
yahoo-finance-pp-cli compare AAPL MSFT GOOG NVDA --agent
```

### `sparkline`

Render a compact terminal sparkline from recent chart data.

```bash
yahoo-finance-pp-cli sparkline AAPL --range 3mo
```

### `sql`

Run SQL directly against the local Yahoo/watchlist/portfolio database.

```bash
yahoo-finance-pp-cli sql "SELECT watchlist, COUNT(*) FROM watchlist_members GROUP BY watchlist" --agent
```

### `fx`

Convert currencies without manually building Yahoo FX pair symbols.

```bash
yahoo-finance-pp-cli fx USD EUR --amount 100 --agent
```

### `options-chain`

Filter Yahoo's raw chain into a usable options view by moneyness and DTE.

```bash
yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --type calls --agent
```

### `auth login-chrome`

Import a browser session when Yahoo blocks the automatic crumb bootstrap from the current IP.

```bash
yahoo-finance-pp-cli auth login-chrome --cookies ~/yahoo-cookies.json --crumb abc123
```

## Command Reference

Market data:

- `yahoo-finance-pp-cli quote --symbols AAPL,MSFT`
- `yahoo-finance-pp-cli quote summary AAPL`
- `yahoo-finance-pp-cli chart AAPL --range 1mo --interval 1d`
- `yahoo-finance-pp-cli fundamentals AAPL --type annualTotalRevenue`
- `yahoo-finance-pp-cli insights --symbol AAPL`
- `yahoo-finance-pp-cli options AAPL`
- `yahoo-finance-pp-cli recommendations AAPL`
- `yahoo-finance-pp-cli screener --scr-ids day_gainers`
- `yahoo-finance-pp-cli trending US`
- `yahoo-finance-pp-cli search "apple"`
- `yahoo-finance-pp-cli autocomplete --query appl`

Local-state and derived workflows:

- `yahoo-finance-pp-cli watchlist create|add|remove|list|show|delete`
- `yahoo-finance-pp-cli portfolio add|list|remove|perf|gains`
- `yahoo-finance-pp-cli digest`
- `yahoo-finance-pp-cli compare`
- `yahoo-finance-pp-cli sparkline`
- `yahoo-finance-pp-cli sql`
- `yahoo-finance-pp-cli fx`
- `yahoo-finance-pp-cli options-chain`

Utilities:

- `yahoo-finance-pp-cli sync`
- `yahoo-finance-pp-cli workflow archive`
- `yahoo-finance-pp-cli workflow status`
- `yahoo-finance-pp-cli export`
- `yahoo-finance-pp-cli import`
- `yahoo-finance-pp-cli doctor`
- `yahoo-finance-pp-cli auth status`
- `yahoo-finance-pp-cli auth logout`
- `yahoo-finance-pp-cli auth login-chrome`

## Practical Recipes

### Morning briefing over a watchlist

```bash
yahoo-finance-pp-cli watchlist create tech
yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG META
yahoo-finance-pp-cli digest --watchlist tech --agent
```

### Track a real portfolio with cost basis

```bash
yahoo-finance-pp-cli portfolio add AAPL 50 185.50 --purchased 2024-06-15
yahoo-finance-pp-cli portfolio add MSFT 20 340.00 --purchased 2024-03-01
yahoo-finance-pp-cli portfolio perf --agent
yahoo-finance-pp-cli portfolio gains --agent
```

### Compare several large-cap names

```bash
yahoo-finance-pp-cli compare AAPL MSFT NVDA GOOG --agent
```

### Fallback when Yahoo blocks your IP

```bash
yahoo-finance-pp-cli auth login-chrome --cookies ~/yahoo-cookies.json --crumb abc123
yahoo-finance-pp-cli doctor
```

## Session Model

Yahoo Finance uses a crumb/cookie session model, not an API key model.

- first live request: the CLI tries to bootstrap the session automatically
- if Yahoo blocks that bootstrap with HTTP 429: use `auth login-chrome`
- inspect cached state: `auth status`
- clear a bad cached session: `auth logout`

## Agent Mode

Add `--agent` when you want machine-oriented output.

It expands to:

- `--json`
- `--compact`
- `--no-input`
- `--no-color`
- `--yes`

Useful companion flags:

- `--select <fields>`
- `--dry-run`
- `--no-cache`
- `--data-source auto|live|local`
- `--rate-limit <n>`

### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
yahoo-finance-pp-cli <command> --agent --select id,name
yahoo-finance-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.

## Exit Codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 2 | Usage error |
| 3 | Resource not found |
| 4 | Session/auth-style error |
| 5 | API error |
| 7 | Rate limited |
| 10 | Config error |

## Argument Parsing

Given `$ARGUMENTS`:

1. Empty, `help`, or `--help` → run `yahoo-finance-pp-cli --help`
2. `install` → install CLI
3. `install mcp` → install MCP server
4. Anything else → map the user request to the best command above and run it with `--agent`

## CLI Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-cli@main
yahoo-finance-pp-cli --version
```

## MCP Server Installation

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-mcp@latest

# If `@latest` installs a stale build (Go module proxy cache lag), install from main:
GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
  go install github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/cmd/yahoo-finance-pp-mcp@main
claude mcp add yahoo-finance yahoo-finance-pp-mcp
```

## Direct Use

1. Check whether `yahoo-finance-pp-cli` is installed.
2. If not installed, offer CLI installation.
3. Choose the command that matches the user's intent most directly.
4. Run with `--agent` unless the user explicitly wants human-formatted output.
5. If Yahoo is rate-limiting this machine, guide the user to `auth login-chrome`.

<!-- pr-218-features -->
## Agent Workflow Features

This CLI exposes three shared agent-workflow capabilities patched in from cli-printing-press PR #218.

### Named profiles

Persist a set of flags under a name and reuse them across invocations.

```bash
# Save the current non-default flags as a named profile
yahoo-finance-pp-cli profile save <name>

# Use a profile — overlays its values onto any flag you don't set explicitly
yahoo-finance-pp-cli --profile <name> <command>

# List / inspect / remove
yahoo-finance-pp-cli profile list
yahoo-finance-pp-cli profile show <name>
yahoo-finance-pp-cli profile delete <name> --yes
```

Flag precedence: explicit flag > env var > profile > default.

### --deliver

Route command output to a sink other than stdout. Useful when an agent needs to hand a result to a file, a webhook, or another process without plumbing.

```bash
yahoo-finance-pp-cli <command> --deliver file:/path/to/out.json
yahoo-finance-pp-cli <command> --deliver webhook:https://hooks.example/in
```

File sinks write atomically (tmp + rename). Webhook sinks POST `application/json` (or `application/x-ndjson` when `--compact` is set). Unknown schemes produce a structured refusal listing the supported set.

### feedback

Record in-band feedback about this CLI from the agent side of the loop. Local-only by default; safe to call without configuration.

```bash
yahoo-finance-pp-cli feedback "what surprised you or tripped you up"
yahoo-finance-pp-cli feedback list         # show local entries
yahoo-finance-pp-cli feedback clear --yes  # wipe
```

Entries append to `~/.yahoo-finance-pp-cli/feedback.jsonl` as JSON lines. When `YAHOO_FINANCE_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `YAHOO_FINANCE_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream (non-blocking — local write always succeeds).

