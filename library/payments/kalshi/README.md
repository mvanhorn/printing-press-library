# Kalshi CLI

Trade prediction markets, track portfolios, and analyze odds on Kalshi from the command line

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/payments/kalshi/cmd/kalshi-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/kalshi-current).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from [Kalshi API settings](https://kalshi.com/api). You will need an API key UUID and a private key file.

```bash
export KALSHI_API_KEY="your-api-key-uuid"
export KALSHI_PRIVATE_KEY_PATH="~/.kalshi/private_key.pem"
```

You can also persist this in your config file at `~/.config/kalshi-pp-cli/config.toml`.

### 3. Verify Setup

```bash
kalshi-pp-cli doctor
```

This checks your configuration, credentials, and API connectivity.

### 4. Try Your First Commands

```bash
# Sync market and portfolio data locally
kalshi-pp-cli sync

# Browse open events
kalshi-pp-cli events --status open

# Check your portfolio positions
kalshi-pp-cli portfolio

# See your P&L breakdown by category
kalshi-pp-cli portfolio attribution --by category
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`portfolio attribution`** -- See your P&L broken down by market category and series over any time period
- **`portfolio winrate`** -- Calculate your win/loss ratio, expected value, and ROI across all settled positions
- **`portfolio calendar`** -- See upcoming settlements with your positions, expected payouts, and category breakdown
- **`portfolio exposure`** -- Analyze portfolio risk by category and concentration
- **`portfolio stale`** -- Find positions in markets approaching expiry where you haven't acted recently
- **`markets movers`** -- Find markets with the biggest price swings since your last sync
- **`markets correlate`** -- Compare price histories of two markets to discover correlated events
- **`markets heatmap`** -- Show market activity by category (volume, open interest, avg price)

## Commands

### Trading & Portfolio

- **`kalshi-pp-cli portfolio`** -- Get positions (shortcut for get-positions)
- **`kalshi-pp-cli portfolio create-order`** -- Create an order
- **`kalshi-pp-cli portfolio amend-order`** -- Amend an existing order
- **`kalshi-pp-cli portfolio cancel-order`** -- Cancel an order
- **`kalshi-pp-cli portfolio decrease-order`** -- Decrease an order
- **`kalshi-pp-cli portfolio batch-create-orders`** -- Batch create orders
- **`kalshi-pp-cli portfolio batch-cancel-orders`** -- Batch cancel orders
- **`kalshi-pp-cli portfolio get-balance`** -- Get account balance
- **`kalshi-pp-cli portfolio get-orders`** -- Get open orders
- **`kalshi-pp-cli portfolio get-fills`** -- Get order fills
- **`kalshi-pp-cli portfolio get-settlements`** -- Get settlements

### Portfolio Analytics (Transcendence)

- **`kalshi-pp-cli portfolio attribution`** -- P&L by category, series, or event
- **`kalshi-pp-cli portfolio winrate`** -- Win rate and ROI analysis
- **`kalshi-pp-cli portfolio calendar`** -- Upcoming settlements
- **`kalshi-pp-cli portfolio exposure`** -- Risk concentration analysis
- **`kalshi-pp-cli portfolio stale`** -- Expiring positions needing attention

### Markets

- **`kalshi-pp-cli markets`** -- Get market orderbooks
- **`kalshi-pp-cli markets get`** -- List markets with filters
- **`kalshi-pp-cli markets get-ticker`** -- Get a single market by ticker
- **`kalshi-pp-cli markets get-trades`** -- Get recent trades
- **`kalshi-pp-cli markets batch-get-candlesticks`** -- Batch get candlestick data
- **`kalshi-pp-cli markets movers`** -- Biggest price changes since last sync
- **`kalshi-pp-cli markets heatmap`** -- Activity by category
- **`kalshi-pp-cli markets correlate`** -- Compare two market price histories

### Events & Series

- **`kalshi-pp-cli events`** -- List events
- **`kalshi-pp-cli events get-eventticker`** -- Get a specific event
- **`kalshi-pp-cli events get-multivariate`** -- Get multivariate events
- **`kalshi-pp-cli series`** -- List series
- **`kalshi-pp-cli series get`** -- Get a specific series
- **`kalshi-pp-cli series get-fee-changes`** -- Get fee change history

### Historical Data

- **`kalshi-pp-cli historical`** -- Get historical trades
- **`kalshi-pp-cli historical get-market`** -- Get historical market data
- **`kalshi-pp-cli historical get-markets`** -- Get historical markets
- **`kalshi-pp-cli historical get-fills`** -- Get historical fills
- **`kalshi-pp-cli historical get-orders`** -- Get historical orders
- **`kalshi-pp-cli historical get-market-candlesticks`** -- Get historical candlesticks
- **`kalshi-pp-cli historical get-cutoff`** -- Get cutoff timestamps

### Exchange & Account

- **`kalshi-pp-cli account`** -- Get API limits
- **`kalshi-pp-cli exchange get-status`** -- Get exchange status
- **`kalshi-pp-cli exchange get-schedule`** -- Get exchange schedule
- **`kalshi-pp-cli exchange get-user-data-timestamp`** -- Get user data timestamp
- **`kalshi-pp-cli incentive-programs`** -- Get active incentive programs

### Communications (RFQ)

- **`kalshi-pp-cli communications create-rfq`** -- Create a request for quote
- **`kalshi-pp-cli communications create-quote`** -- Create a quote
- **`kalshi-pp-cli communications accept-quote`** -- Accept a quote
- **`kalshi-pp-cli communications get-rfqs`** -- List RFQs
- **`kalshi-pp-cli communications get-quotes`** -- List quotes

### Utilities

- **`kalshi-pp-cli doctor`** -- Check CLI health, auth, and API connectivity
- **`kalshi-pp-cli sync`** -- Sync API data to local SQLite
- **`kalshi-pp-cli search`** -- Full-text search across synced data
- **`kalshi-pp-cli analytics`** -- Count, group-by, and summarize synced data
- **`kalshi-pp-cli export`** -- Export data to JSONL or JSON
- **`kalshi-pp-cli import`** -- Import data from JSONL via API
- **`kalshi-pp-cli tail`** -- Stream live changes via polling
- **`kalshi-pp-cli auth`** -- Manage authentication credentials
- **`kalshi-pp-cli workflow archive`** -- Sync all resources for offline access
- **`kalshi-pp-cli workflow status`** -- Show local archive status

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
kalshi-pp-cli portfolio

# JSON for scripting and agents
kalshi-pp-cli portfolio --json

# Filter to specific fields
kalshi-pp-cli portfolio --json --select ticker,market_result,total_traded

# CSV for spreadsheets
kalshi-pp-cli markets get --status open --csv

# Compact output for agents (fewer fields, smaller tokens)
kalshi-pp-cli markets get --compact

# Dry run -- show the request without sending
kalshi-pp-cli portfolio create-order --ticker INXD-26APR25-B5525 --action buy --side yes --count 10 --yes-price 65 --dry-run

# Agent mode -- JSON + compact + no prompts in one flag
kalshi-pp-cli portfolio --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** -- never prompts, every input is a flag
- **Pipeable** -- `--json` output to stdout, errors to stderr
- **Filterable** -- `--select ticker,status` returns only fields you need
- **Previewable** -- `--dry-run` shows the request without sending
- **Retryable** -- creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** -- `--yes` for explicit confirmation of destructive actions
- **Piped input** -- `echo '{"ticker":"INXD-26APR25-B5525","action":"buy"}' | kalshi-pp-cli portfolio create-order --stdin`
- **Cacheable** -- GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** -- no colors or formatting unless `--human-friendly` is set
- **Progress events** -- paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add kalshi-trade-manual kalshi-trade-manual-pp-mcp -e KALSHI_API_KEY=<your-key> -e KALSHI_PRIVATE_KEY_PATH=<path-to-pem>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kalshi-trade-manual": {
      "command": "kalshi-trade-manual-pp-mcp",
      "env": {
        "KALSHI_API_KEY": "<your-key>",
        "KALSHI_PRIVATE_KEY_PATH": "<path-to-pem>"
      }
    }
  }
}
```

## Cookbook

Common workflows and recipes:

```bash
# Check your current positions
kalshi-pp-cli portfolio --json --select ticker,market_result,total_traded

# Browse open markets for a series
kalshi-pp-cli markets get --status open --series-ticker INXD --json

# Look up a specific market's orderbook
kalshi-pp-cli markets get-market INXD-26APR25-B5525

# Place a Yes buy order (preview first with --dry-run)
kalshi-pp-cli portfolio create-order \
  --ticker INXD-26APR25-B5525 --action buy --side yes \
  --count 10 --yes-price 65 --dry-run

# Sync everything locally for offline analysis
kalshi-pp-cli sync

# Search synced data for specific markets
kalshi-pp-cli search "bitcoin"

# See P&L by category over the last 30 days
kalshi-pp-cli portfolio attribution --by category --period 30d

# Find markets with biggest recent price swings
kalshi-pp-cli markets movers --json

# Compare two correlated markets
kalshi-pp-cli markets correlate INXD-26APR25-B5525 INXD-26APR25-B5550

# Check positions expiring soon
kalshi-pp-cli portfolio stale --days 3

# Win rate across all settled positions
kalshi-pp-cli portfolio winrate --by category

# Export historical trades for analysis
kalshi-pp-cli export historical --format jsonl > trades.jsonl

# Group synced data by resource type
kalshi-pp-cli analytics --type markets --group-by status
```

## Health Check

```bash
kalshi-pp-cli doctor
```

```
  OK Config: ok
  FAIL Auth: not configured
  OK API: reachable
  config_path: ~/.config/kalshi-pp-cli/config.toml
  base_url: https://api.elections.kalshi.com/trade-api/v2
  version: 3.13.0
  hint: export KALSHI_API_KEY=<your-key> KALSHI_PRIVATE_KEY_PATH=<path>
```

## Configuration

Config file: `~/.config/kalshi-pp-cli/config.toml`

Environment variables:
- `KALSHI_API_KEY` -- Your Kalshi API key UUID
- `KALSHI_PRIVATE_KEY_PATH` -- Path to your RSA private key PEM file
- `KALSHI_PRIVATE_KEY` -- Inline PEM private key (alternative to file path)
- `KALSHI_BASE_URL` -- Override API base URL (default: `https://api.elections.kalshi.com/trade-api/v2`)
- `KALSHI_ENV` -- Set to `demo` to use the sandbox environment (`https://demo-api.kalshi.co/trade-api/v2`)
- `KALSHI_CONFIG` -- Override config file path

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `kalshi-pp-cli doctor` to check credentials
- Verify environment variables are set: `echo $KALSHI_API_KEY`
- Ensure your private key file exists: `ls -la $KALSHI_PRIVATE_KEY_PATH`
- Kalshi requires both an API key and a private key for authentication

**Not found errors (exit code 3)**
- Check the market ticker or resource ID is correct
- Use `kalshi-pp-cli markets get --tickers TICKER` to verify a ticker exists

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Use `--rate-limit 2` to throttle requests to 2 per second
- If persistent, wait a few minutes and try again

**Demo/sandbox environment**
- Set `export KALSHI_ENV=demo` to use the Kalshi demo environment
- Demo credentials are separate from production

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**OctagonAI/kalshi-deep-trading-bot**](https://github.com/OctagonAI/kalshi-deep-trading-bot) -- TypeScript (168 stars)
- [**austron24/kalshi-cli**](https://github.com/austron24/kalshi-cli) -- Python (14 stars)
- [**newyorkcompute/kalshi**](https://github.com/newyorkcompute/kalshi) -- TypeScript (3 stars)
- [**fsctl/go-kalshi**](https://github.com/fsctl/go-kalshi) -- Go (2 stars)
- [**JThomasDevs/kalshi-cli**](https://github.com/JThomasDevs/kalshi-cli) -- Python (1 stars)
- [**yakub268/kalshi-mcp**](https://github.com/yakub268/kalshi-mcp) -- TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `kalshi-pp-cli profile save <name> --<flag> <value>`, then `kalshi-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.kalshi-pp-cli/feedback.jsonl`. When `KALSHI_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `KALSHI_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
