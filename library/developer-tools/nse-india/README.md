# NSE India CLI

**Every NSE equity data point in a single Go binary — live quotes, order book, filings, and a local SQLite store no other tool has.**

nse-india-pp-cli fetches real-time NSE equity quotes, index constituents, corporate filings, and market status with just browser headers — no API key, no Python runtime, no setup friction. After sync, the local SQLite store powers cross-symbol analysis that is invisible in any single API call: delivery spikes, sector breadth, portfolio margin health, and index attribution.

## Install

The recommended path installs both the `nse-india-pp-cli` binary and the `pp-NSE India` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install NSE India
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install NSE India --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/cmd/nse-india-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/NSE India-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-NSE India --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-NSE India --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-NSE India skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-NSE India. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required. The CLI sends browser-compatible headers (User-Agent + Referer) that mirror what NSE's website uses. Options chain data may require a browser session cookie — use 'nse-india-pp-cli auth login --chrome' if those commands return empty data.

## Quick Start

```bash
# Check if NSE is open before fetching data
nse-india-pp-cli market


# Live price, 52w H/L, sector PE, pre-market IEP
nse-india-pp-cli quote ADANIPORTS


# Order book, delivery%, VaR margin
nse-india-pp-cli depth RELIANCE


# Find today's biggest Nifty losers
nse-india-pp-cli index "NIFTY 50" --json | jq '.data[] | select(.pChange < -3)'


# Cache IT sector for offline analysis
nse-india-pp-cli sync --symbols INFY,TCS,WIPRO,HCLTECH


# Detect institutional accumulation signals
nse-india-pp-cli delivery-spike --threshold 2.0 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`delivery-spike`** — Flags stocks where today's delivery-to-traded ratio is significantly above their 20-session rolling average — early signal of institutional accumulation.

  _Use when you want to detect institutional accumulation before price follows — the delivery spike precedes moves 2-3 sessions earlier than price action alone._

  ```bash
  nse-india-pp-cli delivery-spike --threshold 2.0 --agent
  ```
- **`iep-drift`** — Tracks how accurately each stock's pre-market IEP (Indicative Equilibrium Price) predicts the actual opening price over 30 sessions.

  _Use in pre-market to know which IEP signals are reliable for that day's gap-open strategy._

  ```bash
  nse-india-pp-cli iep-drift --lookback 30 --min-gap 1.5 --agent
  ```
- **`announcement-flood`** — Surfaces companies whose filing cadence has spiked above their own historical baseline — a leading indicator of imminent corporate actions like rights issues or mergers.

  _Use when monitoring for surprise corporate actions — a flood of exchange filings reliably precedes announcements by 3-7 days._

  ```bash
  nse-india-pp-cli announcement-flood --window 7d --threshold 3 --agent
  ```
- **`portfolio pnl`** — Tracks unrealized P&L, daily delta, and drawdown for a personal holdings file against the synced quote history.

  _Use when an agent needs to report portfolio performance or trigger rebalancing alerts without a brokerage integration._

  ```bash
  nse-india-pp-cli portfolio pnl --holdings holdings.csv --agent
  ```
- **`portfolio margin-health`** — Aggregates VaR margin, extreme loss margin, and adhoc margin across a full portfolio — shows total margin-at-risk and which holdings are the biggest margin consumers.

  _Use before market open to check whether overnight VaR changes have pushed margin utilization above a safe threshold._

  ```bash
  nse-india-pp-cli portfolio margin-health --holdings holdings.csv --agent
  ```
- **`delivery-divergence`** — Detects when price is rising but delivery % is falling (distribution signal) or price falling while delivery is rising (accumulation signal) — separates smart money from retail.

  _Use when evaluating whether a trend is sustainable — delivery divergence at a peak is the single clearest distribution signal available from public data._

  ```bash
  nse-india-pp-cli delivery-divergence --lookback 10 --agent
  ```

### Agent-native plumbing
- **`sector-breadth`** — Computes advance/decline ratio, median pChange, and delivery breadth for every constituent of a named index — far richer than the index headline number.

  _Use when deciding whether an index move is broad-based or driven by one or two large caps — changes the conviction of a trade._

  ```bash
  nse-india-pp-cli sector-breadth --sector IT --agent
  ```
- **`index-driver`** — Decomposes any day's index move into per-stock point contributions — identifies the 3-5 stocks driving 80%+ of the index change.

  _Use when the index move looks misleading — tells an agent whether a rally is concentrated (single-stock risk) or genuinely broad._

  ```bash
  nse-india-pp-cli index-driver --index "NIFTY 50" --agent
  ```

## Usage

Run `nse-india-pp-cli --help` for the full command reference and flag list.

## Commands

### corporate

Corporate actions, filings, and financial data

- **`nse-india-pp-cli corporate actions`** - Corporate actions history — dividends, bonuses, splits, rights with ex-dates and record dates
- **`nse-india-pp-cli corporate announcements`** - Exchange filings — board meetings, results, AGM, investor meets, disclosure intimations
- **`nse-india-pp-cli corporate annual_reports`** - Annual reports with direct PDF download URLs, going back multiple years
- **`nse-india-pp-cli corporate financial_results`** - Quarterly and annual financial result filings with XBRL links
- **`nse-india-pp-cli corporate insider_trading`** - SEBI PIT (Prohibition of Insider Trading) disclosures — promoter/director buy/sell with quantities, values, pre/post holding %

### equity

Comprehensive equity quote data for NSE-listed stocks

- **`nse-india-pp-cli equity derivatives`** - F&O data — futures (3 expiries with OI, volume, turnover) and options contracts (CE/PE by strike and expiry)
- **`nse-india-pp-cli equity quote`** - Full equity quote — last price, 52w H/L, sector PE, order book (5 bid/ask levels), delivery%, VaR margin, index memberships, pre-open IEP

### indices

NSE index data and constituents

- **`nse-india-pp-cli indices constituents`** - Live prices for all constituent stocks in an index with 52w range, 1Y/30D% change
- **`nse-india-pp-cli indices list`** - List all available NSE indices with short and long names

### market

Market status and operational information

- **`nse-india-pp-cli market status`** - Real-time market status for all segments (Capital Market, Currency, F&O, WDM) with current NIFTY level

### movers

Market activity rankings

- **`nse-india-pp-cli movers active`** - Most active intraday securities ranked by volume or traded value

### symbol_lookup

Symbol search and autocomplete

- **`nse-india-pp-cli symbol_lookup autocomplete`** - Search symbols by company name or ticker — returns equity, MF, index matches


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
nse-india-pp-cli indices list

# JSON for scripting and agents
nse-india-pp-cli indices list --json

# Filter to specific fields
nse-india-pp-cli indices list --json --select id,name,status

# Dry run — show the request without sending
nse-india-pp-cli indices list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
nse-india-pp-cli indices list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-NSE India -g
```

Then invoke `/pp-NSE India <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/cmd/nse-india-pp-mcp@latest
```

Then register it:

```bash
claude mcp add NSE India nse-india-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/NSE India-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/cmd/nse-india-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "NSE India": {
      "command": "nse-india-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
nse-india-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **options command returns empty JSON** — Run 'nse-india-pp-cli auth login --chrome' to capture browser session cookie; options chain requires NSE session state
- **429 Too Many Requests** — The CLI auto-backs off at 3 req/sec; add '--rate-limit slow' for batch operations or wait 30 seconds
- **delivery-spike shows no results after sync** — Need 20+ sessions of data: run 'nse-india-pp-cli sync --full' daily for 4 weeks before running delivery-spike

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**aeron7/nsepython**](https://github.com/aeron7/nsepython) — Python (500 stars)
- [**hi-imcodeman/stock-nse-india**](https://github.com/hi-imcodeman/stock-nse-india) — JavaScript (400 stars)
- [**vsjha18/nsetools**](https://github.com/vsjha18/nsetools) — Python (300 stars)
- [**BennyThadikaran/NseIndiaApi**](https://github.com/BennyThadikaran/NseIndiaApi) — Python (250 stars)
- [**bshada/nse-bse-mcp**](https://github.com/bshada/nse-bse-mcp) — Python (150 stars)
- [**maanavshah/stock-market-india**](https://github.com/maanavshah/stock-market-india) — JavaScript (120 stars)
- [**RAKESHKRAJEEV90/nse-api-package**](https://github.com/RAKESHKRAJEEV90/nse-api-package) — JavaScript (80 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
