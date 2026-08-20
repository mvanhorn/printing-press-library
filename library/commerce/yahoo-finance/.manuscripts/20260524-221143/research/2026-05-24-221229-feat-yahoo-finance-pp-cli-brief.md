# Yahoo Finance CLI Brief

## API Identity
- **Domain:** finance.yahoo.com — market data, quotes, charts, fundamentals, options, news, screeners
- **Status:** Yahoo shut down the official Finance API in 2017. A thriving unofficial ecosystem exists around reverse-engineered endpoints at `query1.finance.yahoo.com` and `query2.finance.yahoo.com`.
- **Users:** Retail investors, traders, quants, financial analysts, portfolio managers, indie hedge funds, finance bloggers, educators
- **Data profile:** Real-time quotes (15-min delayed on free tier), OHLCV history (80+ years), financial statements (quarterly/annual), analyst recommendations, options chains, institutional holders, insider transactions, news, screeners, trending symbols, earnings calendar

## Reachability Risk
**HIGH.** Multiple signals:
- yfinance issues #2289, #2411, #2422 (2024-2025) all report recurring 429 rate limits
- GitHub discussion ranaroussi/yfinance#2431 describes widespread blocking in 2025
- **Live probe from this machine: HTTP 429 on every endpoint including crumb bootstrap.** This IP is currently rate-limited.
- Yahoo requires a crumb+cookie handshake: visit `fc.yahoo.com` → extract A1/B1 cookies → GET `/v1/test/getcrumb` → pass `crumb` query param and cookies on every subsequent call
- Cloud provider IPs and Tor exits get blocked fast. Residential IPs mostly work if crumb/session is handled correctly.

**Mitigation baked into build:** P0 feature is a robust session client that (a) auto-fetches the crumb, (b) persists cookies to disk, (c) auto-retries with backoff on 429, (d) supports `auth login --chrome` to import a real browser session for cases where the crumb dance fails.

## Top Workflows
1. **"What's AAPL doing right now?"** — current quote with % change, volume, day range, market cap
2. **"Chart me 1Y of TSLA daily"** — historical OHLCV, renders sparkline in terminal or JSON for piping
3. **"Show me the financials on NVDA"** — income statement, balance sheet, cash flow (quarterly/annual)
4. **"What are analysts saying about META?"** — recommendations summary, price targets, upgrades/downgrades
5. **"Options chain on SPY for next Friday"** — calls and puts by expiration, filterable by strike/moneyness
6. **"News on my watchlist"** — recent articles across multiple symbols
7. **"Screen for large-cap growth stocks"** — predefined screeners (day_gainers, most_actives, etc.) and custom filters
8. **"What's trending today?"** — trending symbols per region
9. **"Compare my portfolio returns"** — aggregate performance across tickers held
10. **"Earnings this week"** — calendar of upcoming earnings releases

## Table Stakes (must match every competitor)
From yfinance (ranaroussi, ~14k stars): quote, history, fast_info, dividends, splits, actions, capital_gains, income_stmt (annual/quarterly/ttm), balance_sheet, cashflow, recommendations, recommendations_summary, analyst_price_targets, earnings_estimate, insider_purchases, major_holders, institutional_holders, mutualfund_holders, news, options, option_chain, sustainability, isin, info, get_shares_full, calendar

From yahoo-finance2 (gadicc): quote, quoteSummary (13 submodules: assetProfile, balanceSheetHistory, balanceSheetHistoryQuarterly, calendarEvents, cashflowStatementHistory, defaultKeyStatistics, earnings, financialData, incomeStatementHistory, price, secFilings, summaryDetail, summaryProfile), chart, historical, search, screener, options, insights, recommendationsBySymbol, trendingSymbols, dailyGainers, dailyLosers, autoc, fundamentalsTimeSeries

From competing CLIs (yf, yahoo-finance-cli, scottjbarr/yahoofinance): quote printing, CSV output, CSV-API legacy endpoint, currency conversion, JSON output

From MCPs (Alex2Yang97/yahoo-finance-mcp 262★, AgentX-ai, kanishka-namdeo with caching+circuit breaker): same core + bundled tools with structured responses

## Data Layer
- **Primary entities:**
  - `quotes` — symbol, price snapshot, market cap, 52w range, volume (high churn, TTL cache)
  - `history` — symbol, date, open/high/low/close/adj_close/volume (append-only, partitioned by symbol+interval)
  - `dividends` — symbol, ex_date, amount
  - `splits` — symbol, date, numerator, denominator
  - `options_chains` — symbol, expiration, type (call/put), strike, bid/ask/last/IV/volume/OI (snapshot table, TTL cache)
  - `financials` — symbol, period, statement_type (income/balance/cashflow), line_items (JSON)
  - `recommendations` — symbol, firm, from_grade, to_grade, action, date
  - `holders` — symbol, holder_name, holder_type (major/institutional/fund/insider), shares, pct, date
  - `news` — id, symbol (or list), title, publisher, published_at, url, summary
  - `watchlists` — local concept: named groups of symbols
  - `portfolio_lots` — local concept: symbol, purchase_date, shares, cost_basis (user-entered, for returns calc)
- **Sync cursor:** per-symbol `last_synced_at` in meta. History syncs forward from last bar.
- **FTS/search:** symbols table with name+ticker FTS5 index for offline fuzzy lookup. News title FTS.
- **SQL composability:** all above stored normalized so users can `SELECT avg(close) FROM history WHERE symbol='AAPL' AND date > '2026-01-01'`.

## User Vision
(none provided — user selected "No, let's go")

## Product Thesis
- **Name:** Yahoo Finance GOAT — `yahoo-finance-pp-cli`
- **Why it should exist:**
  - Every existing CLI is tiny (quote-only) or requires a RapidAPI paid key
  - Every library (yfinance, yahoo-finance2) is a library, not a CLI — no agent-native interface, no local store, no offline
  - MCPs are great for Claude Desktop but not for shell pipelines, scripts, cron jobs, or composable workflows
  - Nobody has built a portfolio-tracking CLI backed by local SQLite that can answer "what's my return YTD?" offline
  - Nobody bakes the crumb/session handling in a way that gracefully falls back to browser cookies via `auth login --chrome`
- **Differentiators:**
  1. Every feature from yfinance + yahoo-finance2 + every MCP, with `--json`, `--csv`, `--select`, `--dry-run`, typed exit codes
  2. Robust crumb+cookie session client that matches yahoo-finance2's proven implementation + `auth login --chrome` for stuck IPs
  3. SQLite-backed watchlists, portfolio lots, and historical cache — query them with raw SQL
  4. Transcendence commands that need local joins: portfolio performance, watchlist digest, earnings calendar filtered to holdings, cost-basis gain/loss, dividend income YTD
  5. Terminal charts (sparkline + ASCII candlestick) for at-a-glance reads
  6. Bulk operations: fetch history for 100 symbols concurrently, respecting rate limits

## Build Priorities
1. **P0** — Crumb/cookie session client + retry-with-backoff + `auth login --chrome`. SQLite store for quotes, history, options, financials, holders, news, watchlists, portfolio_lots. FTS5 symbol search. Sync infrastructure.
2. **P1** — Absorb every feature from the manifest below: quote, chart, history, quoteSummary (all 13 modules), options, screener, search, trending, dailyGainers/Losers, recommendations, holders, news, financials (3 statements × 3 periods), earnings calendar, insider transactions. All with `--json`, `--csv`, `--select`, `--compact`, `--dry-run`.
3. **P2** — Transcendence: watchlist management, portfolio tracking, `perf` (YTD/1Y/all), `dividend-income`, `earnings-this-week --holdings`, `digest` (news+movers across watchlist), terminal sparkline, options `moneyness` command (filter to ATM/OTM/ITM), `screen-local` (SQL-based screener on cached fundamentals).
