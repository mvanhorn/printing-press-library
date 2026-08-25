# Screener.in Browser-Sniff Discovery Report

## 1. User Goal Flow
- Goal: Search for any Indian company and retrieve everything Screener.in provides about it (key metrics, analysis, financial statements, shareholding, peers, chart, documents), plus screen/market/IPO/market-pulse features.
- Steps completed:
  1. Loaded feed (/) — observed Core Watchlist feed structure, sublist id 10753465, mark-pos POST
  2. Loaded company page (/company/INFY/consolidated/) — captured chart + peers XHR, full section DOM
  3. Triggered search box — captured /api/company/search/?q= endpoint JSON
  4. Loaded explore (/explore/) — captured screen categories, sector tree
  5. Loaded screen page (/screens/1/the-bull-cartel/) — captured result table structure
  6. Loaded IPO (/ipo/) — captured IPO table
  7. Loaded results (/results/latest/) — authenticated, captured latest-results cards with YOY
  8. Loaded insider trades (/trades/insiders/) — authenticated, captured trade table + filters
  9. Probed filings/full-text-search, pagination, sort params via curl
- Coverage: 9/9 planned steps completed

## 2. Pages & Interactions
- https://www.screener.in/ — feed page (logged in)
- https://www.screener.in/company/INFY/consolidated/ — typed search "Tata", chart/peers XHR fired
- https://www.screener.in/explore/ — screen categories, sector browse links
- https://www.screener.in/screens/1/the-bull-cartel/ — screen result table (25 rows, 39 pages)
- https://www.screener.in/ipo/ — IPO table (upcoming IPOs)
- https://www.screener.in/results/latest/ — latest quarterly results cards (3815 results, 153 pages)
- https://www.screener.in/trades/insiders/ — insider trade rows (126696 summaries, 5068 pages)
- https://www.screener.in/filings/ — market pulse hub (bulk/block/SAST/insider links)

## 3. Browser-Sniff Configuration
- Backend: chrome-devtools MCP (user's running Chrome, logged-in session) + curl validation
- Pacing: ~1 req/s
- Proxy pattern: not detected (standard REST-ish routes under /api/ and HTML pages)

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | /api/company/search/?q=&v=5&fts=1 | 200 | application/json | public |
| GET | /api/company/{id}/chart/?q=&days=&consolidated= | 200 | application/json | public |
| GET | /api/company/{id}/peers/ | 200 | text/html | public |
| GET | /company/{slug}/consolidated/ | 200 | text/html | public |
| GET | /company/{slug}/standalone/ | 200 | text/html | public |
| GET | /screens/{id}/{slug}/?page=N | 200 | text/html | public |
| GET | /explore/ | 200 | text/html | public |
| GET | /market/{sector}/... | 200 | text/html | public |
| GET | /ipo/ | 200 | text/html | public |
| GET | /results/latest/?p=N&sectors=&mcap=&sort=&sme=&watchlist= | 200 | text/html | auth-required |
| GET | /trades/insiders/?p=N&trade_type=&transaction=&person_type=&watchlist= | 200 | text/html | auth-required |
| GET | /filings/ | 200 | text/html | auth-required |
| GET | /full-text-search/?q= | 200 | text/html | auth-required |
| POST | /api/mark-pos/sublist-10753465/ | 200 | text/html | auth-required |

## 5. Traffic Analysis
- Protocols: rest_json (search, chart), html_server_rendered (company, screens, market, ipo, results, trades, filings)
- Auth: cookie session (sessionid + csrftoken); auth-required endpoints redirect anonymous users to /register/
- Parameter evidence: search q; chart q/days/consolidated; screen page; results p/sectors/mcap/sort/sme/watchlist; trades p/reporting_date__year/o/trade_type/transaction/person_type/watchlist
- Protection signals: none (standard_http)
- Generation hints: cookie auth for results/trades/filings/full-text-search; html extraction for tables

## 6. Coverage Analysis
- Exercised: companies (search, profile, chart, peers), screens, market sectors, IPO, results, insider trades, filings
- Likely missed: company corporate-actions/insights (premium-gated), saved-screens, alerts, notebook, AI chat

## 7. Response Samples
- search: JSON array [{id, name, url}]
- chart: {"datasets": [{"metric","label","values":[[date,val],...]}], "meta": {"is_weekly": bool}}
- peers/screen/market: HTML data-table (S.No, Name, CMP, P/E, Mar Cap, Div Yld, NP Qtr, Qtr Profit Var, Sales Qtr, Qtr Sales Var, ROCE)
- results: HTML cards (Price, M.Cap, PE, YOY Sales/EBIDT/Net profit/EPS vs quarter-ago and year-ago)
- trades: HTML rows (Company, Name, Date, Type, Value)
- ipo: HTML table (Name, Period, Price, Listing Date, M.Cap, Subscription, PE, ROCE, Category, Shares, Bids, Subscribed)

## 8. Rate Limiting Events
- None observed. Public endpoints cached (chart max-age=300, search max-age=43200).

## 9. Authentication Context
- Authenticated session used via chrome-devtools MCP (user's logged-in Chrome). Cookie auth validated: /results/latest/ and /trades/insiders/ return 200 with session cookie; anonymous curl gets 302 → /register/. sessionid is HttpOnly. Auth header scheme: cookie-based (no Authorization header).
