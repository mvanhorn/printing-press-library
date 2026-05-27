# BSE Filings CLI Brief

## API Identity
- Domain: BSE (Bombay Stock Exchange) corporate filings — announcements, results, board meetings, corp actions, concall transcript PDFs.
- Base: `https://api.bseindia.com/BseIndiaAPI/api` (JSON) + `https://www.bseindia.com/xml-data/corpfiling/AttachLive/<name>.pdf` (attachments).
- Users: Indian-equity investors, equity-research analysts, portfolio operators. Here: IMstockbox Council bots + operator terminal.
- Data profile: per-scrip time series of filings (8+ quarters). High-gravity entities: filings, holdings, concall chunks, results outcomes.

## Reachability Risk
- **Low.** Confirmed live 2026-05-27: `AnnSubCategoryGetData/w?strScrip=500325` → 200 with real announcements (latest `2026-05-26`). `Corpforthresults/w` → 200. PDF attachment → 200 `application/pdf` 1.4 MB.
- **Header requirement (must bake into client):** `Referer: https://www.bseindia.com/` + browser `User-Agent`. Without them → `301` redirect to members page (not a hard 403, but no JSON). This is transport-level, NOT user auth — no API key, no login.
- No-scrip "all announcements" query returns empty `{}`; per-scrip queries work. Sync iterates per holding, so this is not a blocker.

## Auth
- **None.** No credentials. The Referer/UA headers are a browser fingerprint, injected by the generated HTTP client transport. No `auth login`, no env var key.

## Endpoint surface (discovered via direct probe + BseIndiaApi SDK source)
| Path | Params | Powers |
|---|---|---|
| `/AnnSubCategoryGetData/w` | `pageno, strCat, subcategory, strPrevDate, strToDate, strSearch, strScrip, strType` | announcements feed (sync, outcomes) |
| `/Corpforthresults/w` | `fromdate, todate, scripcode` | forthcoming results (due-soon) |
| `/DefaultData/w` | `ddlcategorys, segment, Fdate, TDate, Purposecode, scripcode` | forthcoming corp actions / board meetings / AGM (due-soon) |
| `/PeerSmartSearch/w` | `Type, text` | name/symbol/ISIN → scrip code (holdings add) |
| `/TabResults_PAR/w` | `scripcode, tabtype` | results snapshot numbers (outcomes beat/miss) |
| `/getScripHeaderData/w` | `scripcode` | OHLC quote (context) |
| `/ComHeadernew/w` | `quotetype, scripcode, seriesid` | equity metadata (sector for holdings) |
| PDF `AttachLive/<ATTACHMENTNAME>` | — | concall transcript / filing PDF |

- `strType`: C=Equity, D=Debt, M=MF/ETF. Date format `YYYYMMDD`.
- `strCat` category values (string): `Result`, `Board Meeting`, `Corp. Action`, `AGM/EGM`, `Company Update`, `New Listing`, `Others`.
- Announcement response fields: `NEWSID, SCRIP_CD, NEWSSUB, DT_TM, NEWS_DT, ANNOUNCEMENT_TYPE, CRITICALNEWS, QUARTER_ID, ATTACHMENTNAME, MORE`.

## Top Workflows
1. Sync every tracked holding's filings since last cursor (incremental).
2. Grep a phrase across all concall transcripts in the portfolio (`concall-grep`).
3. Detect language/tone drift across a company's last N concalls (`thesis-drift`).
4. See which holdings have results/board meetings/AGM due in N days (`due-soon`).
5. Find a phrase appearing across multiple holdings — sector-wide shift (`cross`).

## Table Stakes (absorb from competitors)
- bse (BennyThadikaran): announcements, resultCalendar, corporate actions, scrip lookup, quote, results snapshot.
- bsedata / bseindia / bsescraper: announcements scrape, quotes, gainers/losers.
- nse-bse-mcp / Live-NSE-BSE-MCP / Indian-Stock-Exchange-MCP / finstack-mcp: MCP exposure of the above.
- awesome-stock-skills `fetch-concalls`: concall transcript retrieval (screener.in → BSE/NSE filings → aggregator fallback). Direct competitor for the concall feature.
- We match all of these AND add: offline FTS5 store, cross-holding grep, tone-drift, SQL, agent-native output. No competitor ships portfolio-wide concall grep + drift.

## Data Layer
- SQLite at `~/.bse-filings/filings.db`.
- Tables: `filings` (id, scrip_code, scrip_name, filing_type, filed_at, title, attachment_url, body_text), `holdings` (scrip_code, scrip_name, sector), `concall_chunks` (filing_id, paragraph_n, body), `results_outcomes` (filing_id, quarter, revenue, ebitda, pat, beat_miss).
- FTS5 on `filings.body_text` and `concall_chunks.body`.
- Incremental sync: per-scrip cursor (latest `NEWS_DT`).

## User Vision (from operator brief)
NOI: BSE filings as a *thesis-decay detector* — every concall is a forward-looking confidence vote you can grep across an entire portfolio at once. Cross-holding grep + tone-drift is the leverage no broker-research aggregator ships. Consumed by IMstockbox Council bots as a subprocess + Council group-chat tool.

## Holding universe (v1 seed — manual; IMstockbox Postgres wire-up deferred to v2)
NBCC 534309, TATAPOWER 500400, HDFCBANK 500180, RELIANCE 500325, TCS 532540, INFY 500209, ITC 500875, HINDUNILVR 500696, L&T 500510, SBIN 500112, AXISBANK 532215, BHARTIARTL 532454, MARUTI 532500, ASIANPAINT 500820, TATAMOTORS 500570, POWERGRID 532898, ONGC 500312, NTPC 532555, COALINDIA 533278.

## Product Thesis
- Name: **BSE Filings** (`bse-filings`)
- Why it should exist: the BSE site cannot answer compound, cross-holding, time-windowed questions. This mirrors filings locally and turns 8 quarters of concall language into a greppable, drift-detectable signal across an entire portfolio — for agents and operators.

## Build Priorities
1. **P0 data layer:** filings/holdings/concall_chunks/results_outcomes + FTS5 + per-scrip cursor sync.
2. **P1 absorb:** announcements feed, forthcoming results, corp actions, scrip lookup, results snapshot, PDF fetch+parse, holdings CRUD.
3. **P2 transcend:** `concall-grep`, `thesis-drift`, `cross`, `due-soon`, `outcomes`, `stale`, `concall` (parse+summarize).

## Pitfalls
- PDF attachments may redirect to a temporary signed URL — follow redirects, do not cache the signed URL.
- Normalize to `scrip_code` as primary key (JSON API uses it; ISIN/symbol are secondary).
- Some concall PDFs are scanned images — flag OCR need, skip parse-fail rather than crash.
- Results arrive in 2 waves (outcome filing → detailed financials 1–7 days later) — join on `quarter`.

## Out of scope v1
NSE filings, real-time quotes streaming, order placement, insider/SAST disclosures, IMstockbox Postgres auto-sync.
