# Yahoo Finance GOAT — Absorb Manifest

## Tools Cataloged

| Tool | Type | Stars | Source |
|------|------|-------|--------|
| yfinance (ranaroussi) | Python library | ~14k | https://github.com/ranaroussi/yfinance |
| yahoo-finance2 (gadicc) | Node.js library | ~2.8k | https://github.com/gadicc/yahoo-finance2 |
| Alex2Yang97/yahoo-finance-mcp | MCP server | 262 | https://github.com/Alex2Yang97/yahoo-finance-mcp |
| AgentX-ai/yahoo-finance-server | MCP server | — | https://github.com/AgentX-ai/yahoo-finance-server |
| kanishka-namdeo/yfnhanced-mcp | MCP server (caching+circuit breaker) | — | https://github.com/kanishka-namdeo/yfnhanced-mcp |
| BillGatesCat/yf | Go CLI | — | https://github.com/BillGatesCat/yf |
| tabrindle/yahoo-finance-cli | Node CLI (CSV-API) | — | https://github.com/tabrindle/yahoo-finance-cli |
| scottjbarr/yahoofinance | Go library+CLI | — | https://github.com/scottjbarr/yahoofinance |
| Scarvy/yahoo-finance-api-collection | Endpoint collection (Bruno) | — | https://github.com/Scarvy/yahoo-finance-api-collection |
| ghulette/stock-quote | CLI | — | https://github.com/ghulette/stock-quote |
| boyank/yoc | Options scraper CLI | — | https://github.com/boyank/yoc |
| dpguthrie/yahooquery | Python library | ~1k | https://yahooquery.dpguthrie.com |

## Absorbed (match or beat everything that exists)

### Real-time and near-real-time data
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 1 | Current quote (single or many symbols) | yahoo-finance2 `quote()`, yfinance `fast_info` | `quote AAPL MSFT NVDA` | `--json`, `--csv`, `--compact`, `--select`; supports 100+ symbols in one call with chunking |
| 2 | Watchlist quotes | MCP `get_stock_info` | `watchlist show tech` | Query local SQLite watchlist, dedupe symbols, format as table/json |
| 3 | Pre/post-market price | yfinance | `quote AAPL --extended-hours` | Single flag toggles the `includePrePost` endpoint param |
| 4 | Currency-normalized quote | scottjbarr `yahoofinance` | `quote SONY --in-currency USD` | Auto-fetch FX pair and convert |

### Historical data (chart)
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 5 | Historical OHLCV | yfinance `history()`, yahoo-finance2 `chart()` | `history AAPL --range 1y` | Auto-caches to SQLite, dedup on upsert; `--interval` supports 1m-3mo |
| 6 | Dividend history | yfinance `dividends` | `dividends AAPL` | SQL-composable: `SELECT sum(amount) FROM dividends WHERE symbol='AAPL' AND ex_date > date('now','-1 year')` |
| 7 | Split history | yfinance `splits` | `splits AAPL` | Same offline query path |
| 8 | Corporate actions (combined) | yfinance `actions` | `actions AAPL` | Unified view of divs + splits + capital gains |
| 9 | Historical download (bulk) | yfinance `download()` | `sync history --symbols file:watchlist.txt` | Concurrent fetches with rate-limit-aware pool |
| 10 | Capital gains | yfinance `capital_gains` | `history AAPL --events capitalgains` | Single endpoint param |

### Fundamentals / financials
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 11 | Income statement (annual) | yfinance `income_stmt`, yahoo-finance2 `quoteSummary.incomeStatementHistory` | `financials AAPL --statement income` | Human-readable table; SQL-queryable; `--period annual|quarterly|ttm` |
| 12 | Balance sheet | yfinance `balance_sheet` | `financials AAPL --statement balance-sheet` | Same |
| 13 | Cash flow statement | yfinance `cashflow` | `financials AAPL --statement cashflow` | Same |
| 14 | Quarterly income | yfinance `quarterly_income_stmt` | `financials AAPL --statement income --period quarterly` | Same |
| 15 | TTM income | yfinance `ttm_income_stmt` | `financials AAPL --statement income --period ttm` | Same |
| 16 | Key statistics | yahoo-finance2 `defaultKeyStatistics` | `stats AAPL` | P/E, EPS, market cap, forward P/E, PEG, 52w range, institutional % etc. |
| 17 | Asset/company profile | yahoo-finance2 `assetProfile` | `profile AAPL` | Description, sector, industry, officers, address |
| 18 | Earnings history and estimates | yfinance `earnings_estimate`, yahoo-finance2 `earnings` | `earnings AAPL --history` | Shows actual vs estimate per quarter |
| 19 | Earnings calendar | yahoo-finance2 `calendarEvents` | `calendar AAPL` | Next earnings date, ex-div dates |
| 20 | SEC filings | yahoo-finance2 `secFilings` | `filings AAPL` | Recent 10-K, 10-Q, 8-K links |
| 21 | Time-series fundamentals | yahoo-finance2 `fundamentalsTimeSeries` | `fundamentals AAPL --keys revenue,eps --period annual` | Direct hits to `/ws/fundamentals/v1/finance/timeseries/` |

### Ownership and insider data
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 22 | Institutional holders | yfinance `institutional_holders` | `holders AAPL --type institutional` | Table with shares, value, pct, date |
| 23 | Mutual fund holders | yfinance `mutualfund_holders` | `holders AAPL --type fund` | Same |
| 24 | Major holders breakdown | yahoo-finance2 `majorHoldersBreakdown` | `holders AAPL --type major` | Insider % vs institution % |
| 25 | Insider transactions | yfinance `insider_purchases`, yahoo-finance2 `insiderTransactions` | `insiders AAPL` | Recent buy/sell by officers |
| 26 | Insider holders | yahoo-finance2 `insiderHolders` | `insiders AAPL --type holders` | Current insider holdings |
| 27 | Net share purchase activity | yahoo-finance2 `netSharePurchaseActivity` | `insiders AAPL --activity` | Net buys vs sells trend |

### Analyst data
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 28 | Analyst recommendations | yfinance `recommendations` | `analysts AAPL` | Recent upgrades/downgrades, firm, action |
| 29 | Recommendation summary | yfinance `recommendations_summary` | `analysts AAPL --summary` | Buy/hold/sell counts |
| 30 | Price targets | yfinance `analyst_price_targets` | `analysts AAPL --targets` | Low/mean/high, consensus |
| 31 | Upgrade/downgrade history | yahoo-finance2 `upgradeDowngradeHistory` | `analysts AAPL --history` | Full history |
| 32 | Recommendations by symbol (peers) | yahoo-finance2 `recommendationsBySymbol` | `peers AAPL` | Symbols analysts recommend alongside this one |

### Options
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 33 | Expirations list | yahoo-finance2 `options` | `options AAPL --expirations` | Simple enumeration |
| 34 | Options chain | yfinance `option_chain` | `options AAPL --expiry 2026-04-18` | Calls + puts with IV, OI, volume |
| 35 | Calls only | yfinance `option_chain().calls` | `options AAPL --calls` | Filter |
| 36 | Puts only | yfinance `option_chain().puts` | `options AAPL --puts` | Filter |
| 37 | Filter by strike range | boyank/yoc | `options AAPL --min-strike 150 --max-strike 200` | |
| 38 | ATM contracts | (none direct) | `options AAPL --moneyness atm` | Computed relative to spot |

### News
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 39 | Per-symbol news | yfinance `news`, yahoo-finance2 via search | `news AAPL` | Title, publisher, date, URL |
| 40 | Multi-symbol news digest | (none) | `news --watchlist tech` | Aggregates across watchlist, dedupes |
| 41 | Offline news search | — | `news search "earnings beat"` | FTS5 on locally synced news |

### Discovery
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 42 | Symbol search | yahoo-finance2 `search()`, yfinance | `search apple` | Fuzzy match tickers + companies |
| 43 | Autocomplete | yahoo-finance2 `autoc()` | `search --autocomplete app` | Faster prefix match |
| 44 | Trending symbols | yahoo-finance2 `trendingSymbols` | `trending [region]` | Top symbols by region |
| 45 | Day gainers | yahoo-finance2 `dailyGainers` | `screen day-gainers` | Predefined screener wrapper |
| 46 | Day losers | yahoo-finance2 `dailyLosers` | `screen day-losers` | |
| 47 | Most actives | Predefined screener | `screen most-actives` | |
| 48 | Undervalued large caps | Predefined screener | `screen undervalued-large-caps` | |
| 49 | Growth tech stocks | Predefined screener | `screen growth-tech` | |
| 50 | All predefined screeners (12+) | — | `screen list` | Enumerate available |
| 51 | Insights / research | yahoo-finance2 `insights()` | `insights AAPL` | Technical events, valuation |

### Misc
| # | Feature | Best Source | Our Command | Added Value |
|---|---------|------------|-------------|-------------|
| 52 | ISIN lookup | yfinance `isin` | `profile AAPL --isin` | Derived or queried |
| 53 | Currency conversion | ivanrad/yahoofx | `fx USD EUR --amount 100` | Uses `EURUSD=X` chart |
| 54 | FX rates snapshot | — | `fx rates` | Reads a curated list of pairs |

---

## Transcendence (only possible with our approach)

### User-first feature discovery — personas and rituals

(full personas + Pass-1 customer model in `2026-05-24-221229-novel-features-brainstorm.md`)

- **Priya, retail dividend investor** — Sunday morning roll-up of dividends received that week + ex-div for next 30 days, across 28 holdings. Pain: no tool tracks her cost basis, so every yield is yield-on-market not yield-on-cost.
- **Marcus, options-selling swing trader** — Sunday night scan of 40 tickers for next-Friday / 30-45 DTE OTM puts; cross-checks against earnings dates. Pain: yfinance returns the full chain; he has to compute moneyness and DTE in pandas.
- **Lena, value-screening hobbyist quant** — Saturday afternoon custom P/E + ROE + debt/equity screens across the S&P 500, then cross-references insider net-buying. Pain: Yahoo's screener is 12 predefined IDs only.
- **Sam, autonomous agent author** — 7am Claude agent run on a Hetzner VPS, constantly 429'd. Pain: yfinance returns Python objects not JSON; nobody offers a `--chrome` fallback for blocked cloud IPs.

### Transcendence commands

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|------------------------|
| T1 | Portfolio performance tracker | `portfolio perf` | hand-code | 9/10 | Joins local `portfolio_lots` × cached `history` × `dividends`. No public tool maintains cost-basis state between calls. |
| T2 | Dividend income + yield-on-cost | `portfolio dividends --year 2026` | hand-code | 8/10 | Per-lot dividend roll-up + YoC vs cost_basis; Priya's killer feature. |
| T3 | Morning digest across watchlist | `digest --watchlist tech` | hand-code | 9/10 | Fans out quote / news / calendarEvents / dailyGainers, filtered by local watchlist. Sam + Priya weekly ritual. |
| T4 | Options moneyness + DTE filter | `options AAPL --moneyness otm --max-dte 45` | hand-code | 8/10 | Local compute against spot quote + expiration window; no API filter offers this. Marcus's killer command. |
| T5 | Local SQL screener over synced fundamentals | `screen-local --pe-max 15 --roe-min 0.15` | hand-code | 8/10 | Parametrized SQL over local fundamentals; Yahoo's remote screener is 12 IDs only. Lena's killer command. |
| T6 | Chrome cookie import fallback | `auth login --chrome` | hand-code | 9/10 | Reads Chrome's Cookies SQLite, extracts A1/A3/B1 for `.yahoo.com`, persists to session file. No competitor offers this fallback. |
| T7 | Insider net-buying signal screener | `insiders --recent 30d --net-buying --watchlist tech` | hand-code | 7/10 | Joins recent insider transactions × watchlist with buy/sell ratio aggregation. |
| T8 | Total-return comparator | `compare AAPL MSFT NVDA --range 1y --include-divs` | hand-code | 7/10 | Joins cached `history` × `dividends` for total return (price + reinvested div); no library exposes this single-shot. |
| T9 | Covered-call screener over holdings | `options --covered-calls --min-yield-annualized 0.10 --max-dte 45` | hand-code | 7/10 | Joins `portfolio_lots` (shares >=100) × cached options chains × spot, computes annualized yield. |
| T10 | Watchlist correlation matrix | `watchlist correlate tech --range 6m` | hand-code | 6/10 | Pairwise Pearson over cached daily returns; portfolio-theory staple. |

**Dropped from prior planning (reprint reconciliation):** Earnings calendar `--watchlist` reframed (folds into digest, where the data already converges); see brainstorm doc for full kill table.

**Total features: 54 absorbed + 10 transcendence = 64.**

---

---

## Build Priorities

- **P0 (foundation):** spec-driven resources, data layer (quotes, history, dividends, splits, options_chains, financials, recommendations, holders, news, watchlists, portfolio_lots, symbols FTS), sync infra, crumb+cookie client, `auth login --chrome`, rate-limit handler
- **P1 (absorb):** all 54 absorbed commands — `quote`, `quote-summary`/`summary`, `history`, `dividends`, `splits`, `actions`, `options`, `search`, `autocomplete`, `trending`, `screen` (with all 12 predefined IDs), `insights`, `fundamentals`, `financials`, `stats`, `profile`, `earnings`, `calendar`, `filings`, `holders` (all 4 types), `insiders`, `analysts` (recommendations + summary + targets + history), `peers`, `news`, `fx`
- **P2 (transcend):** T1-T16 — all 16 commands above
- **P3 (polish):** enriched flag help text, sparkline fidelity, README cookbook, csv output for all tables, `--select` for every command

## Auth & Session Handling Note
- Yahoo Finance has no official auth but REQUIRES a crumb+cookie handshake on every request
- Client must: GET `https://fc.yahoo.com/` → persist A1/B1 cookies → GET `/v1/test/getcrumb` → include `crumb=...` on every data call
- Hand-implemented in Phase 3 since the generator's standard auth types don't cover session handshakes
- `auth login --chrome` as a fallback when user's residential/cloud IP is 429-blocked
