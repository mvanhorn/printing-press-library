# SEC EDGAR Absorb Manifest

## Sources surveyed

- `edgartools` (dgunning/edgartools) — Python, 24+ form types, XBRL parsing, MCP integration
- `secedgar` (sec-edgar/sec-edgar) — Python + CLI, bulk filing downloads, index walking
- `bellingcat/EDGAR` — Python CLI, keyword/form search → CSV
- `sec-edgar-mcp` (stefanoamorelli) — MCP server, CIK lookup, 10-K/10-Q/8-K retrieval, XBRL financials, Form 3/4/5
- `sec-edgar-toolkit` (stefanoamorelli) — TS/JS + Python SDKs, real-time retrieval
- `sec-api` (janlukasschroeder/sec-api, paid) — 150+ form types, query API, streaming, 13F, exec comp, exhibits
- `tumarkin/edgar` — Local index + filing download CLI
- Official SEC `data.sec.gov` + `efts.sec.gov` + `www.sec.gov` endpoints (confirmed live-probed)

## Absorbed (match or beat everything that exists in free SEC surfaces)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | CIK lookup by ticker | edgartools, sec-edgar-mcp | `companies lookup <ticker>` | Local SQLite of company_tickers.json; reverse map (CIK→tickers) too; --json, --select |
| 2 | Company search by name | edgartools | `companies search "<name>"` | FTS5 over company name with fuzzy match; --limit, --json |
| 3 | Filing history per CIK | edgartools, secedgar, sec-edgar-mcp | `filings list --cik <cik>` | Synced submissions JSON in SQLite; --form, --since, --until, --json all filter offline |
| 4 | Filing detail by accession | edgartools | `filings get <accession>` | Resolves index.json + lists primary docs; --download writes archives |
| 5 | Filing exhibits download | secedgar | `filings exhibits <accession>` | Lists+downloads Archives/edgar/data tree; --exhibit-type filter |
| 6 | 10-K/10-Q/8-K section extraction | sec-edgar-mcp, sec-api | `filings sections <accession>` | HTML parser into structured items; --section flag |
| 7 | XBRL company facts (all concepts) | edgartools, sec-edgar-mcp | `facts company --cik <cik>` | Full companyfacts.json mirror cached locally; --tag, --unit filters |
| 8 | XBRL single concept time series | edgartools | `facts get --cik <cik> --tag <tag>` | One concept across all filings; --unit, --form filter |
| 9 | XBRL frames (cross-company) | sec-api | `facts frame --tag <tag> --unit USD --period CY2024Q1I` | One concept across all filers per period; cached locally for offline pivot |
| 10 | Balance sheet / income / cash flow | edgartools, sec-edgar-mcp | `facts statement --cik <cik> --kind balance` | Standard us-gaap tag groups; --periods last4 |
| 11 | Form 4 by insider | sec-api, sec-edgar-mcp | `insider by-insider --insider-cik <cik>` | Parses Form 4 XML; --code P/S filter |
| 12 | Form 4 by issuer | sec-api, sec-edgar-mcp | `insider by-issuer --issuer-cik <cik>` | Cross-Form-4 view per company |
| 13 | 13F institutional holdings | edgartools, sec-api | `holdings 13f --cik <cik> --period 2024Q4` | INFORMATION TABLE XML parsed; --top N |
| 14 | N-PORT fund holdings | edgartools, sec-api | `holdings nport --cik <cik>` | Mutual fund / ETF portfolio |
| 15 | Schedule 13D/G beneficial owners | sec-api | `ownership 13dg --issuer-cik <cik>` | 5%+ owner disclosures |
| 16 | Form D private offerings | sec-api | `offerings form-d` | Regulation D private placements |
| 17 | Form 144 restricted-stock notices | edgartools, sec-api | `offerings form-144` | Insider proposed sales |
| 18 | N-PX proxy voting | edgartools | `holdings npx --cik <cik>` | Fund proxy voting records |
| 19 | DEF 14A proxy parsing | edgartools | `filings get --form "DEF 14A"` + section extractor | Exec comp tables, board info, voting items |
| 20 | Full-text search of filings | EFTS, sec-api, edgartools | `search "<phrase>" --form 10-K --start <date> --end <date>` | NDJSON output, --cik filter, --location filter |
| 21 | Latest filings (point query) | edgartools, sec-edgar-mcp | `feed latest --count <n>` | Parses Atom getcurrent; --form filter |
| 22 | Daily / quarterly index walking | secedgar, tumarkin/edgar | `index daily --date YYYY-MM-DD` / `index quarterly --year YYYY --quarter N` | Bulk-mode pagination |
| 23 | EDGAR operational status | sec-edgar-mcp | `status` | Operating-hours probe |
| 24 | SIC industry lookup | edgartools, sec-api | `sic show <code>` | Local SIC reference table |
| 25 | Filings by SIC code | sec-api | `filings by-sic <code> --form <form>` | Filter the local filings table by SIC |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Coverage-list 8-K item pivot | `watchlist items --since 7d --item 2.05,5.02,4.02 --cik-in @watchlist` | 9/10 | Joins synced `filings` (form='8-K', items parsed from submissions) against an in-CLI watchlist file. SELECT cik, accession, items WHERE form='8-K' AND items REGEXP item-list AND filed >= cursor. No single endpoint does cross-CIK + Item-code pivot. |
| 2 | Insider cluster detector | `insider-cluster --within 5d --min-insiders 3 --code S --since 90d` | 9/10 | SQL self-join on rolling date window grouped by issuer_cik HAVING COUNT(DISTINCT insider_cik) ≥ N. Requires local `insider_txns` table; the Form 4 endpoints give the rows, the cluster is the JOIN. |
| 3 | Live filing watch with filters | `watch --form 8-K --item 2.05 --cik-in @watchlist --keyword <re>` | 8/10 | Polls Atom getcurrent every N seconds, applies multi-dim filter (form ∧ item ∧ cik ∧ keyword), emits NDJSON. sec-api offers paid raw stream; no free tool combines multi-dim filtering with structured output. Verify-safe via `cliutil.IsVerifyEnv()`. |
| 4 | XBRL peer-group bench | `industry-bench --tag us-gaap:Revenues --period CY2024Q4 --sic 7372 --stat p50,p90` | 8/10 | JOIN xbrl_facts ON companies.sic; compute percentiles in-process. XBRL frames feeds the rows but offers no SIC grouping. Competitors stop at the raw frame call. |
| 5 | Cross-section concept across companies | `cross-section --tag us-gaap:Revenues --cik AAPL,MSFT,GOOGL --periods last8` | 7/10 | SELECT cik, period, value FROM xbrl_facts WHERE tag=? AND cik IN(?) ORDER BY cik, period DESC LIMIT N per cik. Pivot output as wide CSV/JSON. edgartools covers one company × all periods; frames covers all companies × one period; neither does both axes constrained. |
| 6 | Restatement / non-reliance scanner | `restatements --since 90d` | 7/10 | SELECT WHERE (form='8-K' AND items LIKE '%4.02%') OR form IN ('10-K/A','10-Q/A','20-F/A') AND filed >= cursor. 8-K Item 4.02 is the textbook accounting-irregularity signal; no competitor exposes a one-shot scanner. |
| 7 | Late filer scanner | `late-filers --since 90d --form 10-K` | 6/10 | SELECT WHERE form IN ('NT 10-K','NT 10-Q','NT 20-F') AND filed >= cursor; JOIN companies for name+SIC. NT-form is the SEC's "missed deadline" signal; bellingcat/EDGAR can search but no one surfaces it as first-class. |
| 8 | 13F holdings delta | `holdings-delta --filer-cik <cik> --period 2024Q4 --vs 2024Q3` | 7/10 | LEFT JOIN holdings_13f current vs prior on (filer, issuer); categorize ADD / EXIT / INCREASE / DECREASE. sec-api/edgartools require hand-rolled diff in Python. |
