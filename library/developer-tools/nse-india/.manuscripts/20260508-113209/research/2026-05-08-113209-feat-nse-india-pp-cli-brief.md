# NSE India CLI Brief

## API Identity
- Domain: Indian equity market data — National Stock Exchange of India (NSE)
- Users: Indian retail investors, quants, algo traders, financial analysts, portfolio managers, developers building fintech tools
- Data profile: Real-time equity quotes, OHLCV, order book depth, delivery statistics, corporate announcements, annual reports, index constituents, market status, pre-open auction data, volatility/margin metrics

## Reachability Risk
- **Low** — Live probes confirm APIs return 200 with browser-like User-Agent + Referer headers. No login, no cookie session, no Cloudflare challenge for public equity endpoints.
- Tested working: `/api/quote-equity`, `/api/equity-stockIndices`, `/api/marketStatus`, `/api/search/autocomplete`, `/api/corporate-announcements`, `/api/annual-reports`, `/api/index-names`
- Some endpoints (historical OHLC, option chain) returned 503/empty — likely require a browser session cookie. Scope to confirmed-working endpoints.
- Rate limiting: ~3 req/sec; adaptive backoff required.
- No official API, no SLA. Endpoints are reverse-engineered from the website frontend.

## Top Workflows
1. **Quick stock quote** — `nse-india-pp-cli quote ADANIPORTS` — last price, change%, 52w H/L, sector PE, pre-market IEP
2. **Order book depth** — `nse-india-pp-cli depth ADANIPORTS` — bid/ask levels, delivery%, VaR margin, total traded value
3. **Index dashboard** — `nse-india-pp-cli index "NIFTY 50"` — all 50 constituents with live prices + 1Y/30D % change
4. **Corporate filings** — `nse-india-pp-cli filings ADANIPORTS` — latest announcements, board meetings, results, AGM filings
5. **Market status** — `nse-india-pp-cli market` — open/closed for CM, Currency, F&O, WDM segments with current NIFTY

## Table Stakes (from competitors)
- Symbol search/autocomplete (nsepython, nselib, stock-nse-india all support)
- Real-time quote: LTP, change, 52w H/L, volume (all competitors support)
- Index constituents with prices (stock-nse-india, nsepython)
- Corporate announcements (BennyThadikaran/NseIndiaApi)
- Annual reports with direct PDF links (BennyThadikaran/NseIndiaApi)
- Market status check (all competitors)
- Bulk/block deals (nselib)
- Multiple symbol watch (nsepython)
- JSON output (stock-nse-india, nsepython)

## Data Layer
- Primary entities: `equity_quotes`, `index_data`, `corporate_announcements`, `market_status`
- Sync cursor: symbol list + last_sync timestamp per entity
- FTS/search: symbol name + company name FTS5 for offline symbol lookup; announcement text search

## Codebase Intelligence
- Best reference: [BennyThadikaran/NseIndiaApi](https://github.com/BennyThadikaran/NseIndiaApi) (Python, comprehensive)
- [hi-imcodeman/stock-nse-india](https://github.com/hi-imcodeman/stock-nse-india) (Node.js, Swagger docs)
- Auth: No auth — just `User-Agent: Mozilla/5.0 ...Chrome/120...` + `Referer: https://www.nseindia.com/`
- Data model: symbol → quote (price, metadata, securityInfo, tradeInfo, priceInfo, industryInfo, preOpenMarket)
- Rate limiting: ~3 req/sec enforced; exponential backoff on 429/503
- Architecture: REST JSON APIs served from `www.nseindia.com/api/` — undocumented, website-frontend-backing endpoints

## Product Thesis
- Name: NSE India CLI (`nse-india-pp-cli`)
- Why it should exist: No Go-native CLI for NSE India data exists. Python/JS tools require runtime deps; this ships as a single binary with local SQLite store, offline symbol search, agent-native JSON, and MCP exposure for financial AI workflows. The only tool that lets `jq`, cron, and Claude Desktop reach Indian equity data without Python.

## Build Priorities
1. `quote <symbol>` — full equity quote with rich default table + `--json` for agents
2. `depth <symbol>` — order book + delivery statistics + VaR margins
3. `index <name>` — index constituents with live prices (NIFTY 50, NIFTY BANK, etc.)
4. `market` — real-time market status for all segments
5. `search <query>` — symbol autocomplete (online) + FTS offline search
6. `filings <symbol>` — corporate announcements with type filtering (results, AGM, board meeting)
7. `reports <symbol>` — annual reports with PDF URLs
8. `watch <symbol...>` — multi-symbol price dashboard (refresh loop)
9. `sync` — cache quotes + announcements to SQLite for offline use
10. Transcendence: portfolio P&L tracker, sector heat map, 52-week scanner, delivery spike detector, index basket comparator
