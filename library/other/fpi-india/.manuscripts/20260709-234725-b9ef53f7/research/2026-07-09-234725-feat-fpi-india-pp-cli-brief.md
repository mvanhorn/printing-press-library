# FPI India CLI Brief

## API Identity
- **Primary source:** NSDL FPI Monitor — `https://www.fpi.nsdl.co.in/web/Reports/` (ASP.NET, server-rendered HTML tables). Official NSDL portal publishing Foreign Portfolio Investor (FPI) investment data for India.
- **Fallback source:** CDSL — `https://www.cdslindia.com/Publications/ForeignPortInvestor.html` (static HTML page + published assets).
- **No official API exists** for either. Data is served through stable, query-param-driven ASPX report pages with the actual figures embedded directly in the HTML tables. Reachable via plain HTTP (curl 200), no auth, no bot protection, no clearance gate.
- **Domain:** Indian capital markets / macro flows. Users: equity/debt analysts, macro researchers, financial journalists, quant/algo builders, students.

## Reachability Risk
- **None.** Both hosts return HTTP 200 text/html over plain HTTPS with a standard Chrome UA. No 403/429, no Cloudflare/WAF, no interstitial. `probe-reachability` mode = `standard_http`.
- Transport: `response_format: html` extraction over replayable GET URLs. No browser sidecar, no cookie clearance.

## Phase 1.9 Reachability Gate
- Decision: PASS
- NSDL: GET Yearwise.aspx?RptType=5 -> 200 (90397 bytes)
- CDSL: GET ForeignPortInvestor.html -> 200 (189064 bytes)
- No auth, no 4xx tier hints applicable.

## The Headline Data (NSDL)
`Yearwise.aspx?RptType={5|6}&year={YYYY}&CurrVal={INR|USD}`
- `RptType=5` → **Financial Year** net investment series (1992-93 → present)
- `RptType=6` → **Calendar Year** net investment series
- `&year=YYYY` → drill into a single year's fortnightly/period detail
- `&CurrVal=` → INR crores (default) vs USD

Column schema of the FY/CY series (confirmed from live parse):
`Financial Year | Equity | Debt-General Limit | Debt-VRR | Debt-FAR | Hybrid | Mutual Funds | AIF | ... | Total for the period | Cumulative total`

Sample rows parsed live (INR crores): 1992-93 → 13; 1993-94 → 5127 (cum 5140); 1998-99 → 29973 equity / -147 debt (cum 61237). This single report answers the user's core ask for net flows back to 1992.

## NSDL Report Catalog (menu -> endpoint)
- `Yearwise.aspx?RptType=5` — FPI Investment Details (Financial Year)
- `Yearwise.aspx?RptType=6` — FPI Investment Details (Calendar Year)
- `QuarterlyWise.aspx` — Quarterly FPI Net Investment (Calendar Year)
- `Monthly.aspx` — monthly view
- `Latest.aspx` — latest fortnight snapshot
- `Archive.aspx` — archive of past reports
- `ReportDetail.aspx?RepID=N` — per-report detail pages. **Confirmed RepID -> label mapping (from live anchor text):**
  - RepID=1 — Debt Utilisation Status (of FPI/FII Investment)
  - RepID=2 — Bidding Details (of FPIs/FIIs in Debt Limits)
  - RepID=5 — Government debt limit
  - RepID=6 — Government debt long term
  - RepID=7 — Corporate debt limit
  - RepID=8 — Corporate debt long term infra
  - RepID=14 — FPI AUC Country-wise (top 10 countries) data
  - RepID=18 — FPI AUC category wise data
  - RepID=22 — Category wise AUC data for all clients of Custodians
  - RepID=28 — Value of Offshore Derivative Instrument(ODIs)/Participatory Notes(PNs)
- `RegisteredFIISAFPI.aspx` — registered FII/FPI list
- `ForeignInvestmentLimitMonitoringListing.aspx` — foreign investment limit monitoring

Data delivery: every page embeds `GridView`/`<table id="...">` HTML tables directly. Export is client-side JS (`ExportDIVtoExcel()`) only. A few pages carry an ASP.NET AJAX `webmethod`, but the primary series does not need it.

## Data Layer
- **Primary entities:** `net_investment` (period, asset class, net, cumulative, currency, source); `auc` (assets-under-custody: country-wise top-10, category-wise); `sector_flow` (fortnightly sector-wise); `trade_data` (trade-wise equity & debt); `fpi_registry` (list of FPIs, registration category counts, DDP pendency).
- **Sync cursor:** report period (year / fortnight). Historical rows immutable once published; incremental sync fetches current year + missing years.
- **FTS/search:** period labels, asset classes, sector names, country names, FPI names.

## Source Priority
- Primary: **nsdl-fpi** — no official spec; hand-authored internal YAML spec (`response_format: html`) + hand-coded table parsers. Auth: free/none. Full history 1992-93 → present, FY + CY + monthly-within-CY breakdown confirmed live.
- Fallback: **cdsl-fpi** — no official spec; static HTML page linking ~147 dated `latest_DDMMYYYY.xls` files (real MS-Excel `.vnd.ms-excel` binary, confirmed via live fetch). This is a **rolling recent-window daily snapshot feed** (a few months back), not deep history — CDSL does not expose a long historical archive on this page. Used for latest-daily cross-check against NSDL, not as a history source.
- **Economics:** both free, no key. **Inversion risk:** none — NSDL is unambiguously the richer canonical primary (full history since 1992); CDSL correctly stays secondary/cross-check given its shallow window.

## Top Workflows
1. Pull the full historical net-investment series (FY or CY), filter by year range + asset class, INR or USD.
2. "What did FPIs do this year/fortnight?" — latest-period equity vs debt net flow.
3. Cumulative flow trend + year-over-year comparison.
4. AUC snapshot: which countries/categories hold most Indian assets under custody.
5. Sector-wise fortnightly rotation.
6. Sync to local SQLite, then `sql`/`search` offline, pipe to jq.

## Table Stakes
- Historical net-investment series (equity/debt/hybrid/total/cumulative), FY + CY.
- Currency toggle INR/USD. Latest fortnight snapshot. Local persistence + offline query.

## Product Thesis
- **Name:** fpi-india
- **Why it should exist:** NSDL/CDSL publish authoritative FPI flow data only as clunky ASPX pages with client-side Excel export — no API, no historical time-series download, no scriptable access. `fpi-india` turns 30+ years of FPI equity/debt flows into a synced local SQLite database you can query, filter, aggregate, and pipe — agent-native (`--json`/`--select`), offline, with trend/YoY/cumulative commands no source offers.

## Build Priorities
1. Internal YAML spec (html endpoints) + generate scaffold; NSDL client + HTML table parser (`internal/nsdl/`).
2. `sync` historical net-investment series (FY+CY) -> SQLite `net_investment`; `search`/`sql`/list/get; INR/USD.
3. Latest-fortnight + AUC + sector + registry parsers.
4. Transcendence: trend/YoY/cumulative/flip-detection/net-flow-window; CDSL fallback + cross-check.

## User Vision
- User asked: "Need historical all data of FPI investments in india." Core deliverable = full historical net-investment series, synced locally and queryable. Confirmed source priority: NSDL primary, CDSL fallback.
