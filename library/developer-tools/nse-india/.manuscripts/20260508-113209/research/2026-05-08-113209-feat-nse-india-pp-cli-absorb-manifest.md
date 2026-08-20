# NSE India CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Live equity quote (LTP, change, 52w H/L) | stock-nse-india, nsetools, nsecli | `quote <symbol>` — rich table + `--json` | Offline-capable after sync; --select for agents |
| 2 | Trade info (order book depth, bid/ask) | nse-api-package `tradeInfo()` | `depth <symbol>` — bid/ask levels, totalBuyQty/SellQty | Structured JSON; agents can parse spread |
| 3 | Delivery statistics (delivery%, deliveryQty) | nselib `price_volume_and_deliverable_position_data()` | Included in `depth <symbol>` | Works offline from cache |
| 4 | VaR margin, extremeLossMargin, adhocMargin | nselib `var_margins()` | Included in `depth <symbol>` | Aggregatable across portfolio |
| 5 | Index constituents with live prices | stock-nse-india, nsetools, nselib | `index <name>` — all stocks with price, pChange, 1Y/30D% | Offline-capable; joinable in SQLite |
| 6 | List all NSE indices | nsetools `get_index_list()`, nselib | `index list` | Cached locally |
| 7 | Market status (open/closed per segment) | NseIndiaApi, nse-api-package | `market` — all segments with current NIFTY | Real-time |
| 8 | Symbol search / autocomplete | stock-nse-india `market.lookup()`, nsepython | `search <query>` — online + FTS offline | Offline search from synced symbol list |
| 9 | Corporate announcements | NseIndiaApi, nse-bse-mcp | `filings <symbol> [--type results|board|agm|dividend]` | 1413+ filings for ADANIPORTS; text FTS |
| 10 | Annual reports with PDF links | NseIndiaApi (partial), nse-bse-mcp | `reports <symbol>` — company annual reports with direct PDF URLs | Direct download-ready links |
| 11 | Pre-open market data (IEP, pre-open bids) | nse-api-package `preOpenMarket()` | Included in `quote` output + `pre-open <symbol>` | Stores IEP for iep-drift analysis |
| 12 | Most active securities by volume/value | nsetools, stock-market-india | `movers [--by volume|value]` | --json + --limit |
| 13 | Top gainers / top losers | nsetools `get_top_gainers/losers()`, stock-market-india | `gainers`, `losers` — from index or market-wide | Offline from cached index data |
| 14 | 52-week high/low stocks | nsetools `get_52_week_high/low()` | `52w [--mode high|low]` — stocks at extremes | Cross-tabulated with sector and volume |
| 15 | Advances/declines ratio | NseIndiaApi, nsetools, stock-market-india | Included in `sector-breadth` output | Per-index decomposition |
| 16 | Bulk deals | nselib `bulk_deals()` | `deals --type bulk` — symbol, quantity, price, client | Cacheable |
| 17 | Block deals | nselib `block_deals()` | `deals --type block` | Timestamp filterable |
| 18 | India VIX data | nselib `india_vix_data()` | `vix [--days 30]` — volatility index historical | Trend analysis in SQLite |
| 19 | Trading holidays | nselib `trading_holiday_calendar()`, nse-bse-mcp | `holidays [--year 2026]` | Cached; agent-queryable |
| 20 | Sector/index PE ratio | nselib `pe_ratio_for_equity()`, nsepython | Included in `quote` (pdSectorPe, pdSymbolPe) | Joinable for outlier detection |
| 21 | IPO data (current, upcoming, past) | nse-bse-mcp | `ipo [--status current|upcoming|past]` | JSON + filtering |
| 22 | Options chain data | nselib, nsepython, nse-bse-mcp, stock-nse-india | `options <symbol> [--expiry <date>]` | Requires session cookie; stub if not reachable |
| 23 | PCR ratio (Put-Call Ratio) | nse-bse-mcp | `pcr <symbol>` — derived from options chain | Sentiment indicator |
| 24 | Index-level option chain (NIFTY/BANKNIFTY) | nse-api-package `indexOptionChain()` | `options --index NIFTY` | Same options endpoint |
| 25 | Equity bhavcopy (end-of-day OHLCV file) | NseIndiaApi, nselib | `bhavcopy [--date 08-05-2026]` — download + parse | CSV with ISIN join |
| 26 | Sync to local SQLite | (no competitor has this) | `sync [--symbols <list>] [--full]` | Enables all transcendence features |
| 27 | SQL query interface | (no competitor has this) | `sql "<query>"` | Direct SQLite access |
| 28 | Offline symbol FTS | (no competitor has this) | `search --offline <query>` | After sync; instant results |
| 29 | Multiple output formats | stock-nse-india (JSON), nselib (CSV) | `--json`, `--csv`, `--select <fields>` | All commands |
| 30 | Doctor / health check | (none) | `doctor` | Connectivity + header test |

**Stubs:** Options chain (22-24) require a browser session cookie for `/api/option-chain-equities` — current probes return empty JSON. Will stub with `(stub — requires browser session cookie, use 'auth login --chrome')` unless browser-sniff resolves. All other 28 features are ship-scope with confirmed working endpoints.

## Transcendence (only possible with our SQLite+sync approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Delivery spike detector | `delivery-spike [--threshold 2.0]` | 10 | Rolling 20-session avg delivery% join; single API call gives one day with no baseline |
| 2 | Pre-market IEP vs actual open drift | `iep-drift [--lookback 30]` | 10 | Requires pre-market IEP + actual open stored in same table across sessions |
| 3 | Announcement flood detector | `announcement-flood [--window 7d]` | 9 | Compares current filing cadence against each company's own historical baseline |
| 4 | Sector breadth analyzer | `sector-breadth [--sector IT]` | 9 | Two-table join: index_constituents × daily_quotes; impossible from single API call |
| 5 | 52-week range scanner | `52w-scanner [--mode approaching-high]` | 9 | Tracks proximity trajectory over multiple synced days; approach persistence = conviction |
| 6 | Index basket contributor | `index-driver --index NIFTY50` | 9 | Weight × pChange attribution joins index weights with constituent quotes |
| 7 | Portfolio P&L tracker | `portfolio pnl --holdings holdings.csv` | 9 | Holdings × quote history join; reconstructs portfolio value curve from SQLite |
| 8 | Portfolio margin health | `portfolio margin-health --holdings holdings.csv` | 9 | SUM(var_margin × qty × price) across all holdings; requires multi-symbol join |
| 9 | Correlated movers finder | `correlated-movers --symbol HDFCBANK` | 8 | CORR(pChange_a, pChange_b) over 30d time series; needs cross-symbol history in SQLite |
| 10 | Corporate action proximity alert | `action-proximity [--days-ahead 14]` | 8 | Announcements × portfolio × action-type classifier; three-table join |
| 11 | VaR margin spike tracker | `var-spike [--threshold 20]` | 8 | VaR time series: today vs 5d ago; requires temporal join in quotes history |
| 12 | Delivery-price divergence scanner | `delivery-divergence [--lookback 10]` | 8 | CORR(pChange, delivery_pct) per symbol over 10 sessions; distribution vs accumulation signal |
| 13 | Sector PE outlier detector | `sector-pe-outlier [--sector AUTO]` | 7 | Z-score of stock PE vs sector distribution; requires constituents × quotes × PE join |

