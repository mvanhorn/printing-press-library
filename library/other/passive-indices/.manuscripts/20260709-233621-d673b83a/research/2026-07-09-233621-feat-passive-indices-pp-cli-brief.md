# Passive Indices CLI Brief

## API Identity
- Domain: Indian passive-investing data — NSE index reference/levels/history (niftyindices) + the ETFs and index funds that track those indices (indiapassivefunds). Complementary layers: indices on one side, the linked passive products on the other.
- Users: Indian retail/DIY passive investors, quant/researchers, fintech builders, advisors comparing ETFs vs their benchmark indices.
- Data profile: index constituents & weights, live/EOD index levels, historical index series + TRI, P/E-P/B-DivYield valuation series; on the funds side: ETF/index-fund master, NAV & AUM time series, NFOs, screeners, fund-vs-fund and fund-vs-index compare, market rankings.

## Source Priority
- Primary: **niftyindices.com** — no official spec; partial no-auth HTTP surface confirmed, remainder needs browser-sniff — auth: free
- Secondary (complementary): **indiapassivefunds.com** — no official spec; clean JSON API at data.indiapassivefunds.com, Bearer-token gated (token minted credential-less by frontend) — auth: free-but-token
- Tertiary (deprioritized by user): **bseindices.com** — reachable; deferred this run
- **Economics:** All three are free/public data. No paid key. indiapassivefunds needs a runtime-minted Bearer token, not a user secret.
- **Inversion risk:** indiapassivefunds has the cleaner, fully-enumerated JSON API (14 endpoints w/ exact params extracted from its JS bundle); niftyindices is a mix of static CSV + JSON blob + ASP.NET PageMethods. Do NOT let indiapassivefunds' cleaner surface demote niftyindices — niftyindices leads per user.

## Reachability Risk
- Low. All three hosts return 200 to stdlib HTTP (probe-reachability: standard_http, confidence 0.95). No Cloudflare/WAF challenge.
- **niftyindices: fully resolved, no auth required anywhere.** Live-watch JSON + constituent CSVs confirmed working. Historical index price series, Total Return Index (TRI), and P/E-P/B-Dividend-Yield series are ALL reachable via `POST https://www.niftyindices.com/BackPage/{getHistoricaldatatabletoString | getTotalReturnIndexString | getpepbHistoricaldataDBtoString}` — note: the path is `/BackPage/` (capital P, no `.aspx` extension). The `.aspx` variant (`/Backpage.aspx/...`) IS gated behind Sitefinity auth (302 + `WWW-Authenticate: Bearer`) and must NOT be used — it appears to be a legacy/deprecated route. All three confirmed live via direct HTTP POST with a `cinfo` JSON-string body (`{"cinfo":"{\"name\":\"<Index>\",\"startDate\":\"DD-Mon-YYYY\",\"endDate\":\"DD-Mon-YYYY\",\"indexName\":\"<Index>\"}"}`).
- **indiapassivefunds: fully resolved.** Token mint is `POST https://www.indiapassivefunds.com/pages/api/login` with an empty JSON body `{}` — no user credentials, works over plain HTTP. Returns a short-lived JWT (`role: view`) replayed as `Authorization: Bearer <token>` against `data.indiapassivefunds.com`. Confirmed live against `dashboard`, `screeners/filters`, `nfo`, `symbollookup` (param is `searchTerm`, not `searchText`). Remaining endpoints (`marketrankings`, `funddetail`, `timeseries`, `navtimeseries`, `screeners`, `fundcompare`) have exact query-param contracts extracted from the JS bundle but weren't all live-replayed — low risk since the pattern is uniform and mint+dashboard+nfo already prove the auth flow.

## Top Workflows
1. Look up an NSE index (e.g. NIFTY 50): current level + constituents + weights.
2. Pull historical index series / TRI / valuation (P/E, P/B, div yield) for backtesting.
3. Find and compare ETFs/index funds tracking a given index — NAV, AUM, expense, tracking data.
4. Screen passive funds by AMC / underlying index / asset type / AUM / returns.
5. Cross-layer: given an index, list the passive products tracking it and compare their fidelity/cost.

## Table Stakes (from nsepython / jugaad-data / nsetools / indiapassivefunds UI)
- Index constituents + weights; live index levels; historical index data + TRI; PE/PB/DivYield history.
- ETF/index-fund master + search/symbol lookup; NAV & AUM time series; NFO list; screeners + filters; fund compare; market rankings.

## Data Layer
- Primary entities: `index`, `index_constituent`, `index_history` (level/TRI/valuation), `fund` (ETF/index fund), `fund_nav_series`, `fund_aum_series`, `nfo`, `market_ranking`.
- Sync cursor: date-range for history/time-series; snapshot for constituents/live-watch/rankings.
- FTS/search: index names + fund names/symbols/AMCs.

## Codebase Intelligence
- niftyindices surfaces (confirmed via direct probes; see discovery/discovery-notes.md for full detail):
  - GET https://iislliveblob.niftyindices.com/jsonfiles/LiveIndicesWatch.json — no auth, full index snapshot (level, %chg, 52w hi/lo, open/high/low, TRI subtype).
  - GET https://www.niftyindices.com/IndexConstituent/ind_<index>list.csv — no auth, constituent CSV.
  - POST https://www.niftyindices.com/BackPage/getHistoricaldatatabletoString — no auth, historical OHLC by date range.
  - POST https://www.niftyindices.com/BackPage/getTotalReturnIndexString — no auth, TRI series by date range.
  - POST https://www.niftyindices.com/BackPage/getpepbHistoricaldataDBtoString — no auth, PE/PB/DivYield series by date range.
  - IMPORTANT: use `/BackPage/` (capital P, no `.aspx`) — the `/Backpage.aspx/` variant is auth-gated (legacy route, do not use).
- indiapassivefunds (base https://data.indiapassivefunds.com/api/v1/etf/), exact params extracted from JS bundle and live-verified:
  - Auth: `POST /pages/api/login` (body `{}`, no creds) → `{response:{token, expiration}}` JWT → `Authorization: Bearer <token>`.
  - Endpoints: marketrankings, nfo, funddetail, screeners, screeners/filters, timeseries, navtimeseries, symbollookup (param `searchTerm`), dashboard, historicaldata, fundcompare, cmsdata, searchcms, web/news/stories.
  - List-endpoint responses use field-coded rows (`f_29`, `f_36`...) + a `columns[]` metadata array mapping codes to human `displayName`. CLI must flatten for human/agent output.

## Product Thesis
- Name: passive-indices (binary passive-indices-pp-cli)
- Why it should exist: no tool unifies the NSE index layer with the passive-products layer that tracks it. nsepython/jugaad-data cover raw NSE data but not the ETF/index-fund tracking layer; indiapassivefunds' data is UI-only. A single offline-capable, agent-native CLI that answers "what tracks NIFTY 50 and how well" is novel.

## Build Priorities
1. niftyindices index layer: constituents (CSV), live-watch (JSON), historical OHLC + TRI + PE/PB/DivYield (POST /BackPage/*).
2. indiapassivefunds fund layer: token mint + Bearer client, fund master/search, NAV/AUM series, screeners, compare, rankings, NFO.
3. Cross-layer transcendence: index→tracking-funds join, tracking-fidelity/cost comparison (now with real historical index data), rolling tracking-error, offline SQLite over both layers.

## Known Gaps
None currently identified for the covered scope (niftyindices + indiapassivefunds). bseindices.com remains out of scope per user instruction (not a gap — a deliberate exclusion).
