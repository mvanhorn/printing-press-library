# Alpha Vantage CLI

**Every Alpha Vantage function, plus a local SQLite cache and quota-aware planning no other AV tool has.**

Wraps all 106 Alpha Vantage functions (stocks, news sentiment, fundamentals, FX, crypto, commodities, economics, 45 technical indicators) and adds the local store + cross-call workflows wrappers cannot. The 25/day free-tier quota is the design constraint, not an afterthought: every call logs to local quota_log, `quota plan` previews call-cost before execution, and `pulse us` hydrates a full daily snapshot in two API calls or fewer.

## Install

The recommended path installs both the `alphavantage-pp-cli` binary and the `pp-alphavantage` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install alphavantage
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install alphavantage --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/alphavantage-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-alphavantage --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-alphavantage --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-alphavantage skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-alphavantage. The skill defines how its required CLI can be installed.
```

## Authentication

Set `ALPHAVANTAGE_API_KEY` (or alias `ALPHA_VANTAGE_API_KEY`) in your env. Free key from https://www.alphavantage.co/support/#api-key. Free tier is 25 requests/day, but the CLI is built around that: one good `sync news` burn hydrates the local store and the next 100 reads cost zero quota. Use `doctor` to verify the key works and see remaining budget.

## Quick Start

```bash
# Confirm your API key works and see how many of today's 25 calls you've used.
alphavantage-pp-cli doctor


# Preview a command's quota cost BEFORE running it — critical on the free tier.
alphavantage-pp-cli quota plan news sweep --tickers NVDA,AAPL,MSFT


# Pull NEWS_SENTIMENT for 3 tickers in one shot. Articles + ticker_sentiment[] arrays land in local SQLite.
alphavantage-pp-cli news sweep --tickers NVDA,AAPL,MSFT --json


# Now query the local store for 30-day sentiment trend. Zero API cost.
alphavantage-pp-cli news timeline NVDA --days 30 --json


# FTS5 search across pulled articles. Still zero API cost.
alphavantage-pp-cli news search "AI agent" --tickers NVDA --json


# Today's top gainers with their 7d sentiment z-score from local data.
alphavantage-pp-cli movers brief --side gainers --enrich sentiment --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Quota discipline
- **`quota status`** — See exactly how many of your 25/day Alpha Vantage calls you have left, and preview what a command would burn before running it.

  _Reach for this before any deep-dive or watchlist sweep — burning your 25/day in one fan-out leaves the rest of the session blind. The CLI tracks every call locally, so agents can plan with confidence._

  ```bash
  alphavantage-pp-cli quota status --json
  ```
- **`pulse us`** — Designed for /market-pulse: top movers + macro snapshot + watchlist sentiment delta in <=2 API calls.

  _Run /market-pulse every weekday morning without exhausting the 25/day quota by lunch. The quota strategy IS the feature._

  ```bash
  alphavantage-pp-cli pulse us --json
  ```

### News + sentiment
- **`news sweep`** — Pull NEWS_SENTIMENT for a watchlist in one command; persist articles AND the per-ticker sentiment array every other wrapper drops.

  _When users ask 'which of my watchlist tickers has the strongest negative swing this week,' this is the only tool that can answer locally — without re-burning quota or losing the per-ticker breakdown._

  ```bash
  alphavantage-pp-cli news sweep --watchlist us-core --json
  ```
- **`news timeline`** — Day-by-day mean sentiment + article count for a ticker, computed from local data with zero API calls.

  _Use this in /stock-deep-dive to answer 'how has sentiment evolved for this ticker' without burning a single API call on already-pulled data._

  ```bash
  alphavantage-pp-cli news timeline NVDA --days 30 --json
  ```
- **`news search`** — Full-text search across all locally pulled news articles with boolean AND/OR, prefix matching, ranked results.

  _Find the article that drove a sentiment spike weeks later without burning any quota. Especially useful for research and journalism workflows._

  ```bash
  alphavantage-pp-cli news search "tariff AND TSMC" --from 2026-02-01 --json
  ```

### Cross-source briefings
- **`briefing earnings`** — One-shot pre-earnings card: upcoming earnings date, recent earnings-topic news, last call transcript highlights, current quote.

  _Replaces 5 separate API calls + manual joining with one query. Ideal pre-print research for portfolio decisions._

  ```bash
  alphavantage-pp-cli briefing earnings AAPL --json
  ```
- **`movers brief`** — Today's top gainers/losers with each ticker's 7d sentiment z-score from the local store.

  _Pre-market intel: distinguish 'top gainer with strong recent positive sentiment' from 'top gainer with negative sentiment' (potential reversal)._

  ```bash
  alphavantage-pp-cli movers brief --side gainers --enrich sentiment --json
  ```
- **`macro snapshot`** — Current CPI, Fed Funds Rate, Treasury Yield curve, Unemployment, Nonfarm Payroll in one card.

  _Daily macro context for /market-pulse in two API calls or fewer. Saves quota for ticker-level work._

  ```bash
  alphavantage-pp-cli macro snapshot --json
  ```

### Watchlist intelligence
- **`watchlist sentiment`** — Per-ticker mean sentiment for a named watchlist, with delta-since-last-scan.

  _Weekly Sunday-night ritual: 'which of my 12 names shifted sentiment most this week' in one read._

  ```bash
  alphavantage-pp-cli watchlist sentiment --name us-core --json
  ```
- **`screen`** — Filter tickers by multi-predicate (min sentiment, earnings within N days, recent insider net-buy) across local tables.

  _Surface 'high-conviction setups' in a watchlist without external screeners. Uses only local data after sync._

  ```bash
  alphavantage-pp-cli screen --watchlist us-core --sentiment-min 0.2 --has-earnings-in 14d --insider-net-buy --json
  ```

### Sync infrastructure
- **`sync news`** — Pull-once-query-many: hydrate the local store with news, movers, or earnings calendar; incremental cursor avoids re-fetching.

  _One burn of N calls hydrates the local store; the next 100 reads cost zero quota. The backbone for every other novel feature._

  ```bash
  alphavantage-pp-cli sync news --watchlist us-core --since last --json
  ```

## Usage

Run `alphavantage-pp-cli --help` for the full command reference and flag list.

## Commands

### calendar

Earnings and IPO calendars

- **`alphavantage-pp-cli calendar earnings`** - Earnings calendar for next 3/6/12 months (EARNINGS_CALENDAR, CSV)
- **`alphavantage-pp-cli calendar ipo`** - IPO calendar for the next 3 months (IPO_CALENDAR, CSV)

### commodities

Commodity prices (WTI, Brent, gas, metals, agriculture)

- **`alphavantage-pp-cli commodities all`** - All commodities at once (ALL_COMMODITIES)
- **`alphavantage-pp-cli commodities aluminum`** - Global aluminum price
- **`alphavantage-pp-cli commodities brent`** - Brent crude oil
- **`alphavantage-pp-cli commodities coffee`** - Global coffee price
- **`alphavantage-pp-cli commodities copper`** - Global copper price
- **`alphavantage-pp-cli commodities corn`** - Global corn price
- **`alphavantage-pp-cli commodities cotton`** - Global cotton price
- **`alphavantage-pp-cli commodities gold-silver-history`** - Historical gold and silver prices (GOLD_SILVER_HISTORY)
- **`alphavantage-pp-cli commodities gold-silver-spot`** - Live gold and silver spot prices (GOLD_SILVER_SPOT)
- **`alphavantage-pp-cli commodities natural-gas`** - Henry Hub natural gas spot
- **`alphavantage-pp-cli commodities sugar`** - Global sugar price
- **`alphavantage-pp-cli commodities wheat`** - Global wheat price
- **`alphavantage-pp-cli commodities wti`** - West Texas Intermediate crude oil (WTI)

### crypto

Cryptocurrency exchange rates and time series

- **`alphavantage-pp-cli crypto daily`** - Daily crypto time series (DIGITAL_CURRENCY_DAILY)
- **`alphavantage-pp-cli crypto intraday`** - Intraday crypto time series (DIGITAL_CURRENCY_INTRADAY)
- **`alphavantage-pp-cli crypto monthly`** - Monthly crypto time series (DIGITAL_CURRENCY_MONTHLY)
- **`alphavantage-pp-cli crypto rate`** - Realtime exchange rate between two currencies, crypto or fiat (CURRENCY_EXCHANGE_RATE)
- **`alphavantage-pp-cli crypto weekly`** - Weekly crypto time series (DIGITAL_CURRENCY_WEEKLY)

### earnings

Earnings call transcripts (EARNINGS_CALL_TRANSCRIPT)

- **`alphavantage-pp-cli earnings transcript`** - Earnings call transcript with LLM-generated speaker sentiment

### econ

Economic indicators (GDP, CPI, Fed Funds, Treasury, unemployment)

- **`alphavantage-pp-cli econ cpi`** - Consumer Price Index (CPI)
- **`alphavantage-pp-cli econ durables`** - US durable goods orders (DURABLES, monthly)
- **`alphavantage-pp-cli econ federal-funds-rate`** - Federal funds rate (FEDERAL_FUNDS_RATE)
- **`alphavantage-pp-cli econ inflation`** - US inflation rate (INFLATION, annual)
- **`alphavantage-pp-cli econ nonfarm-payroll`** - US nonfarm payroll (NONFARM_PAYROLL, monthly)
- **`alphavantage-pp-cli econ real-gdp`** - Real Gross Domestic Product (REAL_GDP)
- **`alphavantage-pp-cli econ real-gdp-per-capita`** - Real GDP per capita (REAL_GDP_PER_CAPITA)
- **`alphavantage-pp-cli econ retail-sales`** - US retail sales (RETAIL_SALES, monthly)
- **`alphavantage-pp-cli econ treasury-yield`** - US Treasury yield rates (TREASURY_YIELD)
- **`alphavantage-pp-cli econ unemployment`** - US unemployment rate (UNEMPLOYMENT, monthly)

### fundamentals

Company fundamentals (overview, statements, dividends, splits, ETFs)

- **`alphavantage-pp-cli fundamentals balance`** - Annual and quarterly balance sheets (BALANCE_SHEET)
- **`alphavantage-pp-cli fundamentals cashflow`** - Annual and quarterly cash flow statements (CASH_FLOW)
- **`alphavantage-pp-cli fundamentals dividends`** - Historical dividend payments (DIVIDENDS)
- **`alphavantage-pp-cli fundamentals earnings`** - Historical earnings (EPS) data (EARNINGS)
- **`alphavantage-pp-cli fundamentals earnings-estimates`** - Projected earnings estimates (EARNINGS_ESTIMATES)
- **`alphavantage-pp-cli fundamentals etf`** - ETF profile and holdings (ETF_PROFILE)
- **`alphavantage-pp-cli fundamentals income`** - Annual and quarterly income statements (INCOME_STATEMENT)
- **`alphavantage-pp-cli fundamentals listings`** - Listing/delisting status snapshot (LISTING_STATUS, CSV)
- **`alphavantage-pp-cli fundamentals overview`** - Company profile, ratios, and metrics (COMPANY_OVERVIEW)
- **`alphavantage-pp-cli fundamentals shares`** - Outstanding share count (SHARES_OUTSTANDING)
- **`alphavantage-pp-cli fundamentals splits`** - Stock split history (SPLITS)

### fx

Foreign exchange rates

- **`alphavantage-pp-cli fx daily`** - Daily FX rates (FX_DAILY)
- **`alphavantage-pp-cli fx intraday`** - Intraday FX (FX_INTRADAY, premium for full history)
- **`alphavantage-pp-cli fx monthly`** - Monthly FX rates (FX_MONTHLY)
- **`alphavantage-pp-cli fx weekly`** - Weekly FX rates (FX_WEEKLY)

### indicator

Technical indicators (SMA, EMA, RSI, MACD, BBANDS, 40+ more)

- **`alphavantage-pp-cli indicator ad`** - Chaikin A/D Line (AD)
- **`alphavantage-pp-cli indicator adosc`** - Chaikin A/D Oscillator (ADOSC)
- **`alphavantage-pp-cli indicator adx`** - ADX technical indicator
- **`alphavantage-pp-cli indicator adxr`** - ADXR technical indicator
- **`alphavantage-pp-cli indicator apo`** - APO technical indicator
- **`alphavantage-pp-cli indicator aroon`** - AROON technical indicator
- **`alphavantage-pp-cli indicator aroonosc`** - AROONOSC technical indicator
- **`alphavantage-pp-cli indicator atr`** - ATR technical indicator
- **`alphavantage-pp-cli indicator bbands`** - Bollinger Bands (BBANDS)
- **`alphavantage-pp-cli indicator bop`** - BOP technical indicator
- **`alphavantage-pp-cli indicator cci`** - CCI technical indicator
- **`alphavantage-pp-cli indicator cmo`** - CMO technical indicator
- **`alphavantage-pp-cli indicator dema`** - DEMA technical indicator
- **`alphavantage-pp-cli indicator dx`** - DX technical indicator
- **`alphavantage-pp-cli indicator ema`** - EMA technical indicator
- **`alphavantage-pp-cli indicator ht-dcperiod`** - HT_DCPERIOD Hilbert transform indicator
- **`alphavantage-pp-cli indicator ht-dcphase`** - HT_DCPHASE Hilbert transform indicator
- **`alphavantage-pp-cli indicator ht-phasor`** - HT_PHASOR Hilbert transform indicator
- **`alphavantage-pp-cli indicator ht-sine`** - HT_SINE Hilbert transform indicator
- **`alphavantage-pp-cli indicator ht-trendline`** - HT_TRENDLINE Hilbert transform indicator
- **`alphavantage-pp-cli indicator ht-trendmode`** - HT_TRENDMODE Hilbert transform indicator
- **`alphavantage-pp-cli indicator kama`** - KAMA technical indicator
- **`alphavantage-pp-cli indicator macd`** - Moving Average Convergence/Divergence (MACD)
- **`alphavantage-pp-cli indicator macdext`** - MACD with controllable MA types (MACDEXT)
- **`alphavantage-pp-cli indicator mama`** - MESA Adaptive Moving Average (MAMA)
- **`alphavantage-pp-cli indicator mfi`** - MFI technical indicator
- **`alphavantage-pp-cli indicator midpoint`** - MIDPOINT technical indicator
- **`alphavantage-pp-cli indicator midprice`** - MIDPRICE technical indicator
- **`alphavantage-pp-cli indicator minus-di`** - MINUS_DI technical indicator
- **`alphavantage-pp-cli indicator minus-dm`** - MINUS_DM technical indicator
- **`alphavantage-pp-cli indicator mom`** - MOM technical indicator
- **`alphavantage-pp-cli indicator natr`** - NATR technical indicator
- **`alphavantage-pp-cli indicator obv`** - On Balance Volume (OBV)
- **`alphavantage-pp-cli indicator plus-di`** - PLUS_DI technical indicator
- **`alphavantage-pp-cli indicator plus-dm`** - PLUS_DM technical indicator
- **`alphavantage-pp-cli indicator ppo`** - PPO technical indicator
- **`alphavantage-pp-cli indicator roc`** - ROC technical indicator
- **`alphavantage-pp-cli indicator rocr`** - ROCR technical indicator
- **`alphavantage-pp-cli indicator rsi`** - RSI technical indicator
- **`alphavantage-pp-cli indicator sar`** - Parabolic SAR (SAR)
- **`alphavantage-pp-cli indicator sma`** - SMA technical indicator
- **`alphavantage-pp-cli indicator stoch`** - Stochastic oscillator (STOCH)
- **`alphavantage-pp-cli indicator stochf`** - Stochastic fast (STOCHF)
- **`alphavantage-pp-cli indicator stochrsi`** - Stochastic RSI (STOCHRSI)
- **`alphavantage-pp-cli indicator t3`** - T3 technical indicator
- **`alphavantage-pp-cli indicator tema`** - TEMA technical indicator
- **`alphavantage-pp-cli indicator trange`** - TRANGE technical indicator
- **`alphavantage-pp-cli indicator trima`** - TRIMA technical indicator
- **`alphavantage-pp-cli indicator trix`** - TRIX technical indicator
- **`alphavantage-pp-cli indicator ultosc`** - Ultimate Oscillator (ULTOSC)
- **`alphavantage-pp-cli indicator vwap`** - Volume Weighted Average Price (VWAP, intraday only)
- **`alphavantage-pp-cli indicator willr`** - WILLR technical indicator
- **`alphavantage-pp-cli indicator wma`** - WMA technical indicator

### insider

Insider transactions (Form 4 filings)

- **`alphavantage-pp-cli insider transactions`** - Latest and historical insider transactions for a ticker (INSIDER_TRANSACTIONS)

### institutional

Institutional ownership (13F)

- **`alphavantage-pp-cli institutional holdings`** - Institutional holdings for a ticker (INSTITUTIONAL_HOLDINGS)

### market

Global market status

- **`alphavantage-pp-cli market status`** - Open/close status for major global equity markets (MARKET_STATUS)

### movers

Top gainers, losers, and most active (TOP_GAINERS_LOSERS)

- **`alphavantage-pp-cli movers top`** - Top 20 gainers, losers, and most actively traded US tickers

### news

News & sentiment (NEWS_SENTIMENT)

- **`alphavantage-pp-cli news sentiment`** - Live and historical market news with per-ticker sentiment scores (NEWS_SENTIMENT)

### options

Options chain data (premium for realtime; HISTORICAL_OPTIONS free with limits)

- **`alphavantage-pp-cli options historical`** - Historical options chain (HISTORICAL_OPTIONS, 15+ years)
- **`alphavantage-pp-cli options realtime`** - Realtime US options chain with Greeks and IV (REALTIME_OPTIONS, premium)

### query

Raw query escape hatch: call any Alpha Vantage function directly

- **`alphavantage-pp-cli query raw`** - Pass-through to any Alpha Vantage function with arbitrary params

### quote

Latest stock quotes and bulk realtime quotes

- **`alphavantage-pp-cli quote bulk`** - Realtime quotes for up to 100 US symbols (premium tier)
- **`alphavantage-pp-cli quote get`** - Latest price and volume for a single ticker (GLOBAL_QUOTE)

### series

Stock time series (intraday, daily, weekly, monthly)

- **`alphavantage-pp-cli series daily`** - Daily OHLCV time series (20+ years; --adjusted requires premium)
- **`alphavantage-pp-cli series intraday`** - Intraday OHLCV time series at 1/5/15/30/60-min intervals (premium for full 20y history)
- **`alphavantage-pp-cli series monthly`** - Monthly OHLCV time series (--adjusted adds dividend events)
- **`alphavantage-pp-cli series weekly`** - Weekly OHLCV time series (--adjusted adds dividend events)

### tickers

Ticker symbol search (SYMBOL_SEARCH)

- **`alphavantage-pp-cli tickers lookup`** - Search for tickers by keywords with match scoring (SYMBOL_SEARCH)

### windows

Advanced analytics over fixed or sliding windows (ANALYTICS_FIXED_WINDOW / ANALYTICS_SLIDING_WINDOW)

- **`alphavantage-pp-cli windows fixed`** - Fixed-window analytics: MEAN/STDDEV/CORRELATION across multiple tickers (ANALYTICS_FIXED_WINDOW)
- **`alphavantage-pp-cli windows sliding`** - Sliding-window analytics across multiple tickers (ANALYTICS_SLIDING_WINDOW)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
alphavantage-pp-cli quote get mock-value

# JSON for scripting and agents
alphavantage-pp-cli quote get mock-value --json

# Filter to specific fields
alphavantage-pp-cli quote get mock-value --json --select id,name,status

# Dry run — show the request without sending
alphavantage-pp-cli quote get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
alphavantage-pp-cli quote get mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-alphavantage -g
```

Then invoke `/pp-alphavantage <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add alphavantage alphavantage-pp-mcp -e ALPHAVANTAGE_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/alphavantage-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ALPHAVANTAGE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "alphavantage": {
      "command": "alphavantage-pp-mcp",
      "env": {
        "ALPHAVANTAGE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
alphavantage-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/alphavantage-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ALPHAVANTAGE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `alphavantage-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ALPHAVANTAGE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`{"Information": "...rate limit..."}` or `{"Note": "..."}` in response** — You hit the 25/day cap. Run `alphavantage-pp-cli quota status --json` to confirm. Wait for the reset (00:00 UTC) or query the local store instead — `news timeline`, `news search`, `screen`, `watchlist sentiment` all read locally.
- **Commands return empty data without error** — Likely the silent rate-limit case — older wrapper bug. Run `alphavantage-pp-cli doctor --json` to verify and force-surface the response shape.
- **API key in `ALPHA_VANTAGE_API_KEY` not picked up** — The CLI reads `ALPHAVANTAGE_API_KEY` first; `ALPHA_VANTAGE_API_KEY` is the alias. Either works; check `alphavantage-pp-cli doctor` for which one resolved.
- **Premium-only endpoint returns nothing on free tier** — REALTIME_OPTIONS, REALTIME_BULK_QUOTES, TIME_SERIES_INTRADAY require premium. Use `query function=PREMIUM_FUNCTION ...` to confirm AV's exact response.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**alpha_vantage_mcp**](https://github.com/alphavantage/alpha_vantage_mcp) — Python
- [**portfoliotree/alphavantage**](https://github.com/portfoliotree/alphavantage) — Go
- [**RomelTorres/alpha_vantage**](https://github.com/RomelTorres/alpha_vantage) — Python
- [**ashleydavis/alpha-vantage-cli**](https://github.com/ashleydavis/alpha-vantage-cli) — JavaScript
- [**berlinbra/alpha-vantage-mcp**](https://github.com/berlinbra/alpha-vantage-mcp) — Python
- [**matteoantoci/mcp-alphavantage**](https://github.com/matteoantoci/mcp-alphavantage) — TypeScript
- [**calvernaz/alphavantage**](https://github.com/calvernaz/alphavantage) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
