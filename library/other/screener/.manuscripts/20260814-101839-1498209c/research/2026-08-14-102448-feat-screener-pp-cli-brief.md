# Screener.in CLI Brief

## API Identity
- Domain: Indian stock market fundamental analysis & screening
- Users: Retail investors, value investors, stock screeners, LLM agents doing equity research
- Data profile: 2500+ listed Indian companies; per-company financials (quarterly/annual P&L, balance sheet, cash flow, ratios, shareholding, pros/cons, peers), price history, insider trades, IPO data, screening screens

## Reachability Risk
- **None.** All public endpoints returned HTTP 200 with real content via plain curl (no bot protection observed). Chart endpoint returns JSON; peers/company/screen/market/IPO return HTML data-tables. Authenticated endpoints (results, trades, feed, full-text-search) require a session cookie (302 → /register/ when anonymous). Cookie replay works via the sessionid cookie (HttpOnly, must be captured via browser).
- Probe-safe endpoint: GET /api/company/search/?q=Tata&v=5&fts=1

## Top Workflows
1. Search for a company by name/ticker and get its complete fundamental profile (key metrics, analysis/pros-cons, quarterly results, financials, shareholding, peers)
2. Screen the market: run a screen (Bull Cartel, Magic Formula, etc.) or browse a sector and get the ranked result table (CMP, P/E, Mar Cap, Div Yld, NP Qtr, Qtr Profit Var, Sales Qtr, Qtr Sales Var, ROCE)
3. Monitor market pulse: latest quarterly results with YOY growth, insider trades, IPO calendar
4. Compare peers for a company (valuation + performance columns)
5. Pull price/technical chart history (Price, DMA50, DMA200, Volume) for a company

## Table Stakes
- Company profile sections: quarterly-results, profit-loss, balance-sheet, cash-flow, ratios, shareholding, pros-cons, about, key-metrics, peer-comparison (screenercli parity)
- Search company by name/ticker with clean JSON
- Screen results with the standard table columns
- Consistent JSON output, `--view consolidated|standalone`, error handling

## Data Layer
- Primary entities: companies (slug, id, name, url), company_financials (section tables), peers, screens, market_sectors, trades, ipo, price_history
- Sync cursor: none (HTML snapshot data); cache per-company pages with TTL
- FTS/search: company name/ticker search via API endpoint; local SQLite for cached company profiles and screen results

## Product Thesis
- Name: screener-pp-cli
- Why it should exist: Screener.in is the go-to free Indian stock research site but has no official API, no offline access, and its competitors are thin Python scrapers. A Go CLI with a local SQLite mirror, agent-native JSON, screen result caching, and market-pulse monitoring beats every existing tool — works offline, scriptable, composable with jq.

## Build Priorities
1. Company search + full profile fetch (key metrics, analysis, quarterly, P&L, balance sheet, cash flow, ratios, shareholding, pros/cons, peers) with --view and --json
2. Screen results (by slug/id) + screen list + explore/sector browse with the standard table
3. Chart data (Price, DMA, Volume)
4. Market pulse: latest results (auth), insider trades (auth), IPO calendar
5. Local SQLite mirror + offline search + novel cross-company commands
