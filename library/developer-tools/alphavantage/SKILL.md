---
name: pp-alphavantage
description: "Every Alpha Vantage function, plus a local SQLite cache and quota-aware planning no other AV tool has. Trigger phrases: `alphavantage`, `alpha vantage`, `news sentiment X`, `sentiment for X`, `AV news`, `top movers`, `top gainers losers`, `X 情感分析`, `X 新闻情感`, `市场情绪`, `use alphavantage`."
author: "lokisbo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - alphavantage-pp-cli
---

# Alpha Vantage — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `alphavantage-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install alphavantage --cli-only
   ```
2. Verify: `alphavantage-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Wraps all 106 Alpha Vantage functions (stocks, news sentiment, fundamentals, FX, crypto, commodities, economics, 45 technical indicators) and adds the local store + cross-call workflows wrappers cannot. The 25/day free-tier quota is the design constraint, not an afterthought: every call logs to local quota_log, `quota plan` previews call-cost before execution, and `pulse us` hydrates a full daily snapshot in two API calls or fewer.

## When to Use This CLI

Use this CLI any time the workflow involves Alpha Vantage's news sentiment, top movers, earnings calendar, or technical indicators. The killer use case is sentiment-aware research: combine news sweep with the local FTS5 store and you can answer 'how did sentiment on NVDA evolve before its last earnings spike?' without re-burning quota. Pair with `/stock-deep-dive` and `/market-pulse` from the broader stock-toolkit; the `pulse us` command is purpose-built for the morning briefing.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**calendar** — Earnings and IPO calendars

- `alphavantage-pp-cli calendar earnings` — Earnings calendar for next 3/6/12 months (EARNINGS_CALENDAR, CSV)
- `alphavantage-pp-cli calendar ipo` — IPO calendar for the next 3 months (IPO_CALENDAR, CSV)

**commodities** — Commodity prices (WTI, Brent, gas, metals, agriculture)

- `alphavantage-pp-cli commodities all` — All commodities at once (ALL_COMMODITIES)
- `alphavantage-pp-cli commodities aluminum` — Global aluminum price
- `alphavantage-pp-cli commodities brent` — Brent crude oil
- `alphavantage-pp-cli commodities coffee` — Global coffee price
- `alphavantage-pp-cli commodities copper` — Global copper price
- `alphavantage-pp-cli commodities corn` — Global corn price
- `alphavantage-pp-cli commodities cotton` — Global cotton price
- `alphavantage-pp-cli commodities gold-silver-history` — Historical gold and silver prices (GOLD_SILVER_HISTORY)
- `alphavantage-pp-cli commodities gold-silver-spot` — Live gold and silver spot prices (GOLD_SILVER_SPOT)
- `alphavantage-pp-cli commodities natural-gas` — Henry Hub natural gas spot
- `alphavantage-pp-cli commodities sugar` — Global sugar price
- `alphavantage-pp-cli commodities wheat` — Global wheat price
- `alphavantage-pp-cli commodities wti` — West Texas Intermediate crude oil (WTI)

**crypto** — Cryptocurrency exchange rates and time series

- `alphavantage-pp-cli crypto daily` — Daily crypto time series (DIGITAL_CURRENCY_DAILY)
- `alphavantage-pp-cli crypto intraday` — Intraday crypto time series (DIGITAL_CURRENCY_INTRADAY)
- `alphavantage-pp-cli crypto monthly` — Monthly crypto time series (DIGITAL_CURRENCY_MONTHLY)
- `alphavantage-pp-cli crypto rate` — Realtime exchange rate between two currencies, crypto or fiat (CURRENCY_EXCHANGE_RATE)
- `alphavantage-pp-cli crypto weekly` — Weekly crypto time series (DIGITAL_CURRENCY_WEEKLY)

**earnings** — Earnings call transcripts (EARNINGS_CALL_TRANSCRIPT)

- `alphavantage-pp-cli earnings <symbol>` — Earnings call transcript with LLM-generated speaker sentiment

**econ** — Economic indicators (GDP, CPI, Fed Funds, Treasury, unemployment)

- `alphavantage-pp-cli econ cpi` — Consumer Price Index (CPI)
- `alphavantage-pp-cli econ durables` — US durable goods orders (DURABLES, monthly)
- `alphavantage-pp-cli econ federal-funds-rate` — Federal funds rate (FEDERAL_FUNDS_RATE)
- `alphavantage-pp-cli econ inflation` — US inflation rate (INFLATION, annual)
- `alphavantage-pp-cli econ nonfarm-payroll` — US nonfarm payroll (NONFARM_PAYROLL, monthly)
- `alphavantage-pp-cli econ real-gdp` — Real Gross Domestic Product (REAL_GDP)
- `alphavantage-pp-cli econ real-gdp-per-capita` — Real GDP per capita (REAL_GDP_PER_CAPITA)
- `alphavantage-pp-cli econ retail-sales` — US retail sales (RETAIL_SALES, monthly)
- `alphavantage-pp-cli econ treasury-yield` — US Treasury yield rates (TREASURY_YIELD)
- `alphavantage-pp-cli econ unemployment` — US unemployment rate (UNEMPLOYMENT, monthly)

**fundamentals** — Company fundamentals (overview, statements, dividends, splits, ETFs)

- `alphavantage-pp-cli fundamentals balance` — Annual and quarterly balance sheets (BALANCE_SHEET)
- `alphavantage-pp-cli fundamentals cashflow` — Annual and quarterly cash flow statements (CASH_FLOW)
- `alphavantage-pp-cli fundamentals dividends` — Historical dividend payments (DIVIDENDS)
- `alphavantage-pp-cli fundamentals earnings` — Historical earnings (EPS) data (EARNINGS)
- `alphavantage-pp-cli fundamentals earnings-estimates` — Projected earnings estimates (EARNINGS_ESTIMATES)
- `alphavantage-pp-cli fundamentals etf` — ETF profile and holdings (ETF_PROFILE)
- `alphavantage-pp-cli fundamentals income` — Annual and quarterly income statements (INCOME_STATEMENT)
- `alphavantage-pp-cli fundamentals listings` — Listing/delisting status snapshot (LISTING_STATUS, CSV)
- `alphavantage-pp-cli fundamentals overview` — Company profile, ratios, and metrics (COMPANY_OVERVIEW)
- `alphavantage-pp-cli fundamentals shares` — Outstanding share count (SHARES_OUTSTANDING)
- `alphavantage-pp-cli fundamentals splits` — Stock split history (SPLITS)

**fx** — Foreign exchange rates

- `alphavantage-pp-cli fx daily` — Daily FX rates (FX_DAILY)
- `alphavantage-pp-cli fx intraday` — Intraday FX (FX_INTRADAY, premium for full history)
- `alphavantage-pp-cli fx monthly` — Monthly FX rates (FX_MONTHLY)
- `alphavantage-pp-cli fx weekly` — Weekly FX rates (FX_WEEKLY)

**indicator** — Technical indicators (SMA, EMA, RSI, MACD, BBANDS, 40+ more)

- `alphavantage-pp-cli indicator ad` — Chaikin A/D Line (AD)
- `alphavantage-pp-cli indicator adosc` — Chaikin A/D Oscillator (ADOSC)
- `alphavantage-pp-cli indicator adx` — ADX technical indicator
- `alphavantage-pp-cli indicator adxr` — ADXR technical indicator
- `alphavantage-pp-cli indicator apo` — APO technical indicator
- `alphavantage-pp-cli indicator aroon` — AROON technical indicator
- `alphavantage-pp-cli indicator aroonosc` — AROONOSC technical indicator
- `alphavantage-pp-cli indicator atr` — ATR technical indicator
- `alphavantage-pp-cli indicator bbands` — Bollinger Bands (BBANDS)
- `alphavantage-pp-cli indicator bop` — BOP technical indicator
- `alphavantage-pp-cli indicator cci` — CCI technical indicator
- `alphavantage-pp-cli indicator cmo` — CMO technical indicator
- `alphavantage-pp-cli indicator dema` — DEMA technical indicator
- `alphavantage-pp-cli indicator dx` — DX technical indicator
- `alphavantage-pp-cli indicator ema` — EMA technical indicator
- `alphavantage-pp-cli indicator ht-dcperiod` — HT_DCPERIOD Hilbert transform indicator
- `alphavantage-pp-cli indicator ht-dcphase` — HT_DCPHASE Hilbert transform indicator
- `alphavantage-pp-cli indicator ht-phasor` — HT_PHASOR Hilbert transform indicator
- `alphavantage-pp-cli indicator ht-sine` — HT_SINE Hilbert transform indicator
- `alphavantage-pp-cli indicator ht-trendline` — HT_TRENDLINE Hilbert transform indicator
- `alphavantage-pp-cli indicator ht-trendmode` — HT_TRENDMODE Hilbert transform indicator
- `alphavantage-pp-cli indicator kama` — KAMA technical indicator
- `alphavantage-pp-cli indicator macd` — Moving Average Convergence/Divergence (MACD)
- `alphavantage-pp-cli indicator macdext` — MACD with controllable MA types (MACDEXT)
- `alphavantage-pp-cli indicator mama` — MESA Adaptive Moving Average (MAMA)
- `alphavantage-pp-cli indicator mfi` — MFI technical indicator
- `alphavantage-pp-cli indicator midpoint` — MIDPOINT technical indicator
- `alphavantage-pp-cli indicator midprice` — MIDPRICE technical indicator
- `alphavantage-pp-cli indicator minus-di` — MINUS_DI technical indicator
- `alphavantage-pp-cli indicator minus-dm` — MINUS_DM technical indicator
- `alphavantage-pp-cli indicator mom` — MOM technical indicator
- `alphavantage-pp-cli indicator natr` — NATR technical indicator
- `alphavantage-pp-cli indicator obv` — On Balance Volume (OBV)
- `alphavantage-pp-cli indicator plus-di` — PLUS_DI technical indicator
- `alphavantage-pp-cli indicator plus-dm` — PLUS_DM technical indicator
- `alphavantage-pp-cli indicator ppo` — PPO technical indicator
- `alphavantage-pp-cli indicator roc` — ROC technical indicator
- `alphavantage-pp-cli indicator rocr` — ROCR technical indicator
- `alphavantage-pp-cli indicator rsi` — RSI technical indicator
- `alphavantage-pp-cli indicator sar` — Parabolic SAR (SAR)
- `alphavantage-pp-cli indicator sma` — SMA technical indicator
- `alphavantage-pp-cli indicator stoch` — Stochastic oscillator (STOCH)
- `alphavantage-pp-cli indicator stochf` — Stochastic fast (STOCHF)
- `alphavantage-pp-cli indicator stochrsi` — Stochastic RSI (STOCHRSI)
- `alphavantage-pp-cli indicator t3` — T3 technical indicator
- `alphavantage-pp-cli indicator tema` — TEMA technical indicator
- `alphavantage-pp-cli indicator trange` — TRANGE technical indicator
- `alphavantage-pp-cli indicator trima` — TRIMA technical indicator
- `alphavantage-pp-cli indicator trix` — TRIX technical indicator
- `alphavantage-pp-cli indicator ultosc` — Ultimate Oscillator (ULTOSC)
- `alphavantage-pp-cli indicator vwap` — Volume Weighted Average Price (VWAP, intraday only)
- `alphavantage-pp-cli indicator willr` — WILLR technical indicator
- `alphavantage-pp-cli indicator wma` — WMA technical indicator

**insider** — Insider transactions (Form 4 filings)

- `alphavantage-pp-cli insider <symbol>` — Latest and historical insider transactions for a ticker (INSIDER_TRANSACTIONS)

**institutional** — Institutional ownership (13F)

- `alphavantage-pp-cli institutional <symbol>` — Institutional holdings for a ticker (INSTITUTIONAL_HOLDINGS)

**market** — Global market status

- `alphavantage-pp-cli market` — Open/close status for major global equity markets (MARKET_STATUS)

**movers** — Top gainers, losers, and most active (TOP_GAINERS_LOSERS)

- `alphavantage-pp-cli movers` — Top 20 gainers, losers, and most actively traded US tickers

**news** — News & sentiment (NEWS_SENTIMENT)

- `alphavantage-pp-cli news` — Live and historical market news with per-ticker sentiment scores (NEWS_SENTIMENT)

**options** — Options chain data (premium for realtime; HISTORICAL_OPTIONS free with limits)

- `alphavantage-pp-cli options historical` — Historical options chain (HISTORICAL_OPTIONS, 15+ years)
- `alphavantage-pp-cli options realtime` — Realtime US options chain with Greeks and IV (REALTIME_OPTIONS, premium)

**query** — Raw query escape hatch: call any Alpha Vantage function directly

- `alphavantage-pp-cli query` — Pass-through to any Alpha Vantage function with arbitrary params

**quote** — Latest stock quotes and bulk realtime quotes

- `alphavantage-pp-cli quote bulk` — Realtime quotes for up to 100 US symbols (premium tier)
- `alphavantage-pp-cli quote get` — Latest price and volume for a single ticker (GLOBAL_QUOTE)

**series** — Stock time series (intraday, daily, weekly, monthly)

- `alphavantage-pp-cli series daily` — Daily OHLCV time series (20+ years; --adjusted requires premium)
- `alphavantage-pp-cli series intraday` — Intraday OHLCV time series at 1/5/15/30/60-min intervals (premium for full 20y history)
- `alphavantage-pp-cli series monthly` — Monthly OHLCV time series (--adjusted adds dividend events)
- `alphavantage-pp-cli series weekly` — Weekly OHLCV time series (--adjusted adds dividend events)

**tickers** — Ticker symbol search (SYMBOL_SEARCH)

- `alphavantage-pp-cli tickers <keywords>` — Search for tickers by keywords with match scoring (SYMBOL_SEARCH)

**windows** — Advanced analytics over fixed or sliding windows (ANALYTICS_FIXED_WINDOW / ANALYTICS_SLIDING_WINDOW)

- `alphavantage-pp-cli windows fixed` — Fixed-window analytics: MEAN/STDDEV/CORRELATION across multiple tickers (ANALYTICS_FIXED_WINDOW)
- `alphavantage-pp-cli windows sliding` — Sliding-window analytics across multiple tickers (ANALYTICS_SLIDING_WINDOW)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
alphavantage-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pre-earnings sentiment + transcript check

```bash
alphavantage-pp-cli briefing earnings AAPL --json --select symbol,upcoming_earnings_date,recent_articles_count,sentiment_7d_mean,last_transcript_highlights
```

One command joins EARNINGS_CALENDAR, NEWS_SENTIMENT, and EARNINGS_CALL_TRANSCRIPT for a ticker. Use --select to narrow the response.

### Top gainers minus low-sentiment names

```bash
alphavantage-pp-cli movers brief --side gainers --enrich sentiment --agent --select results.ticker,results.change_percentage,results.sentiment_7d_z_score
```

Pre-market intel: today's gainers ranked alongside their local sentiment z-score. Drop names with negative sentiment z (potential reversal).

### Watchlist sentiment delta since last scan

```bash
alphavantage-pp-cli watchlist sentiment --name us-core --agent
```

Reads named watchlist + recent ticker_sentiment history; reports per-ticker mean + change vs prior scan. Sunday-night ritual.

### Compound screen: high-sentiment with upcoming earnings

```bash
alphavantage-pp-cli screen --watchlist us-core --sentiment-min 0.2 --has-earnings-in 14d --agent
```

Multi-predicate SQL across local tables. Surface high-conviction setups without external screeners.

### Local FTS5 search for theme exposure

```bash
alphavantage-pp-cli news search '"data center" OR cloud' --tickers NVDA,AMD --from 2026-04-01 --agent --select results.title,results.url,results.ticker_sentiment
```

Find articles mentioning a theme across pulled sentiment data. FTS5 phrase queries use double-quoted phrases (`"data center"`) and boolean `AND` / `OR`. Zero API cost.

## Auth Setup

Set `ALPHAVANTAGE_API_KEY` (or alias `ALPHA_VANTAGE_API_KEY`) in your env. Free key from https://www.alphavantage.co/support/#api-key. Free tier is 25 requests/day, but the CLI is built around that: one good `sync news` burn hydrates the local store and the next 100 reads cost zero quota. Use `doctor` to verify the key works and see remaining budget.

Run `alphavantage-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  alphavantage-pp-cli quote get mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
alphavantage-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
alphavantage-pp-cli feedback --stdin < notes.txt
alphavantage-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.alphavantage-pp-cli/feedback.jsonl`. They are never POSTed unless `ALPHAVANTAGE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ALPHAVANTAGE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
alphavantage-pp-cli profile save briefing --json
alphavantage-pp-cli --profile briefing quote get mock-value
alphavantage-pp-cli profile list --json
alphavantage-pp-cli profile show briefing
alphavantage-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `alphavantage-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add alphavantage-pp-mcp -- alphavantage-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which alphavantage-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   alphavantage-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `alphavantage-pp-cli <command> --help`.
