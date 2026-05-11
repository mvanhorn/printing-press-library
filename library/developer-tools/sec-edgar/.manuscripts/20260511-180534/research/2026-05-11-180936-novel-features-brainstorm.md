# Novel Features Brainstorm — sec-edgar-pp-cli

## Customer model

### Persona 1: Maya — sell-side equity analyst covering US large-cap tech

**Today (without this CLI):** Maya has 11 tabs open across `sec.gov/cgi-bin/browse-edgar`, EDGAR full-text search, three company Investor Relations pages, and a Bloomberg terminal. When a 10-Q drops she ctrl-clicks the accession, hunts the primary 10-Q doc, ctrl-Fs "going concern" and "subsequent events," then opens the exhibits index to find the EX-99.1 press release. For XBRL she uses a colleague's Python script that wraps `companyfacts.json` per ticker — works one company at a time, never across her 14-company coverage list. She can't easily answer "show me every 8-K my coverage list filed with Item 2.05 in the last 90 days."

**Weekly ritual:** Monday morning she pulls every 8-K filed by her 14 coverage names since Friday close, scans Item codes to triage what matters (5.02 exec change, 2.05 restructuring, 1.01 material agreement). Then Tuesday-Thursday she does deep-dive XBRL pulls on whichever name reported — revenue, operating margin, gross margin, comparing latest quarter to four prior quarters and to the two closest comps.

**Frustration:** She can't pivot. EDGAR shows one company or one form at a time; her coverage list is implicit in her head. Cross-company comparison means 14 browser tabs and a manual spreadsheet she rebuilds quarterly.

### Persona 2: Devraj — forensic accounting / activist short-seller researcher

**Today (without this CLI):** Devraj has the EDGAR full-text search bookmarked and a stack of Excel files exported from bellingcat/EDGAR. He hunts for late filers (NT 10-K), restated financials, 8-K Item 4.02 (non-reliance on prior financials), and unusual auditor changes. He runs the same five EFTS queries every Monday and pastes results into a Google sheet by hand. For insider trades he goes to OpenInsider or sec-api's paid stream; he hates paying for what EDGAR gives away free.

**Weekly ritual:** Monday: scan last week's NT 10-K, 8-K/A, and 8-K Item 4.02 filings firm-wide. Wednesday: pull every Form 4 filed by C-suite at his ~40-name short watchlist; flag clustered selling (3+ insiders selling within 5 trading days). Friday: re-run EFTS searches for keywords ("material weakness", "subsequent event", "going concern") across the last 7 days of filings.

**Frustration:** The cluster pattern — "3 insiders at the same issuer selling within 5 days" — is the highest-signal pattern he tracks and there's no way to ask EDGAR for it. He runs it manually in Excel after pulling Form 4s one issuer at a time. Same for "company whose 10-K filing cadence slipped from on-time to NT."

### Persona 3: Priya — quant researcher building factor models

**Today (without this CLI):** Priya wants every public company's `AccountsPayableCurrent`, `Revenues`, and `OperatingIncomeLoss` for the last 20 quarters, joined to SIC code, joined to insider buying pressure. Today she pulls XBRL frames endpoint quarter-by-quarter, concept-by-concept, into a dozen JSON files, then writes pandas to flatten and join them. The whole pipeline takes a weekend per refresh.

**Weekly ritual:** Quarterly: rebuild the universe of XBRL facts she models against. Weekly: increment with new filings since last cursor, recompute factor exposures, re-rank her portfolio. She lives in `pandas.merge` and SQL.

**Frustration:** There is no single SQLite she can `JOIN` against. The XBRL frames API gives one (concept, period) at a time as JSON — she needs a queryable store. Building that store herself is the tax she pays every quarter.

### Persona 4: Sam — fintech / agent builder shipping an LLM that answers SEC questions

**Today (without this CLI):** Sam glues together sec-edgar-mcp, edgartools, and a hand-rolled `Archives/edgar/data/` fetcher. The MCP tools he has cover CIK lookup and 10-K section extraction but not XBRL frames pivots or insider clustering. His agent can answer "what did Apple's last 10-K say about supply chain" but not "which of Apple's peers filed 8-K Item 2.05 this quarter."

**Weekly ritual:** Ship features. Whenever a customer asks a new SEC question, he extends his tool surface. He wants typed tool annotations, JSON-by-default output, and a `--select` that lets the agent prune deeply-nested XBRL payloads before they blow the context window.

**Frustration:** Every existing tool is a thin endpoint wrapper. The agent needs derived signals (clustered insider buys, peer-group XBRL pivots, filings-since-cursor) that no single endpoint exposes — and writing them himself per customer is unscalable.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline kill/keep |
|---|------|---------|-------------|---------|--------|------------------|
| C1 | Coverage-list 8-K item pivot | `watchlist items --since 7d --item 2.05,5.02,4.02` | Across a saved watchlist of CIKs, pivot 8-Ks filed in the window by Item code | Maya, Devraj | (a) persona, (b) 8-K item codes are SEC-specific content | KEEP — cross-CIK pivot, no single endpoint does this |
| C2 | Insider cluster detector | `insider-cluster --within 5d --min-insiders 3 --code S` | Issuers where N+ distinct insiders filed Form 4 with code S (or P) within a rolling window | Devraj, Maya | (a) persona, (b) Form 4 transaction codes | KEEP — local JOIN on `insider_txns`, no endpoint exists |
| C3 | Live filing watch with filters | `watch --form 8-K --item 2.05 --cik-in @watchlist --keyword "going concern"` | Stream the Atom getcurrent feed, apply filters, print one line per match | Maya, Devraj, Sam | (a) persona, brief Build Priority 2 | KEEP — Atom polling + local filter |
| C4 | XBRL peer-group bench | `industry-bench --tag us-gaap:Revenues --period CY2024Q4 --sic 7372 --stat p50,p90` | XBRL frame for one (concept, period) grouped by SIC | Priya, Maya | (a) persona, (b) frames + SIC join | KEEP — joins `xbrl_facts` to `companies.sic` |
| C5 | One concept across N companies | `cross-section --tag us-gaap:Revenues --cik AAPL,MSFT,GOOGL --periods last8` | Pivot one XBRL concept across explicit company list and last N periods | Priya, Maya | (a) persona, (c) cross-entity | KEEP — JOIN xbrl_facts on (cik, tag) by period |
| C6 | Restatement scanner | `restatements --since 90d` | 8-K Item 4.02 + 10-K/A + 10-Q/A in window with issuer SIC | Devraj | (b) restated 10-Ks, 8-K Item 4.02 is SEC-specific | KEEP |
| C7 | Late filer scanner | `late-filers --form 10-K --since 90d` | Issuers that filed NT 10-K (or NT 10-Q) in the window | Devraj | (b) NT 10-K is SEC-specific | KEEP |
| C8 | Filing-cadence drift | `form-cadence --form 10-K --status drifted` | Companies whose 10-K filing day-of-fiscal-year shifted | Devraj | (a) persona, (c) cross-entity | KILL — verifiability low, brittle thresholds |
| C9 | XBRL concept history | `concept-history --cik AAPL --tag us-gaap:OperatingIncomeLoss` | One concept across all periods for one company | Priya, Maya | (b), brief Priority 2 | KILL — wrapper-vs-leverage fails; subsumed by C5 |
| C10 | 13F holdings delta | `holdings-delta --filer-cik <institution> --period 2024Q4 --vs 2024Q3` | Diff of 13F holdings across two consecutive quarters | Maya, Priya | (b), (c) | KEEP — local JOIN on holdings_13f |
| C11 | Filings-since cursor for watchlist | `since --cik-in @watchlist --since 2025-04-01` | Print every filing across watchlist since cursor | Maya, Sam | (a), (c) | KILL — duplicates filings list with multi-CIK |
| C12 | SQL escape hatch | `sql "SELECT ..."` | Raw SELECT against local store with FTS5 | Priya, Sam | (c), brief Priority 2 | KILL from novel list — framework already provides it; keep capability |
| C13 | Coverage XBRL refresh | `sync facts --cik-in @watchlist` | Sync command | All | brief Priority 0 | KILL — Priority 0, not novel |
| C14 | Cluster + price-action | `insider-cluster --with-price-move` | C2 + overlay stock price | Devraj | (a) | KILL — external service required |
| C15 | Exhibit-graph cross-file | `exhibits-by-issuer --cik <cik> --exhibit-type EX-10` | Across an issuer's filings, list every EX-10 material contract exhibit | Maya | (b) | KILL — not weekly-use; better as README recipe |
| C16 | Recent IPO watch | `ipos --since 30d` | Recent S-1 filings | Maya, Devraj | (b) | KILL — thin filter on filings.form='S-1' |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Coverage-list 8-K item pivot | `watchlist items --since 7d --item 2.05,5.02,4.02 --cik-in @watchlist` | 9/10 | Joins synced `filings` (form='8-K', items column parsed from submissions) against an in-CLI watchlist file | Brief workflow #1+#2; Maya's Monday ritual; 8-K Item codes are SEC-specific. |
| 2 | Insider cluster detector | `insider-cluster --within 5d --min-insiders 3 --code S --since 90d` | 9/10 | SQL self-join on rolling date window grouped by issuer_cik HAVING COUNT(DISTINCT insider_cik) >= ? | Devraj's Wednesday pattern; no competitor clusters Form 4s. |
| 3 | Live filing watch with filters | `watch --form 8-K --item 2.05 --cik-in @watchlist --keyword <re>` | 8/10 | Polls Atom getcurrent, matches each entry against multi-dimensional filter, emits NDJSON; verify-safe | Brief workflow #6; sec-api offers paid stream, no free competitor has multi-dim filter. |
| 4 | XBRL peer-group bench | `industry-bench --tag us-gaap:Revenues --period CY2024Q4 --sic 7372 --stat p50,p90` | 8/10 | JOIN xbrl_facts ON companies.sic; compute percentiles in-process | Priya's quarterly rebuild; XBRL frames + SIC join. |
| 5 | One concept across N companies | `cross-section --tag us-gaap:Revenues --cik AAPL,MSFT,GOOGL --periods last8` | 7/10 | SELECT cik, period, value FROM xbrl_facts WHERE tag=? AND cik IN(?); pivot output | Maya's comp-set ritual; edgartools/frames each cover one axis, not both. |
| 6 | Restatement scanner | `restatements --since 90d` | 7/10 | SELECT FROM filings WHERE (form='8-K' AND items LIKE '%4.02%') OR form LIKE '%/A' | Devraj's Monday pattern; non-reliance is textbook signal; no first-class competitor command. |
| 7 | Late filer scanner | `late-filers --since 90d --form 10-K` | 6/10 | SELECT WHERE form IN ('NT 10-K','NT 10-Q','NT 20-F') AND filed >= cursor | Devraj's Monday pattern; bellingcat can search, no one surfaces NT-form as first-class. |
| 8 | 13F holdings delta | `holdings-delta --filer-cik <cik> --period 2024Q4 --vs 2024Q3` | 7/10 | LEFT JOIN holdings_13f current vs prior on (filer, issuer); categorize ADD/EXIT/INCREASE/DECREASE | Maya quant overlay + Priya factor; sec-api/edgartools require hand-rolled diff. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C8 Filing-cadence drift | Verifiability fails; brittle thresholds; persona overlap | C7 late-filers |
| C9 XBRL concept history | Wrapper-vs-leverage fails; subsumed by C5 with one CIK | C5 cross-section |
| C11 Filings-since cursor | Thin wrapper on table-stakes #3; transcendence proof fails | C1 watchlist items |
| C12 SQL escape hatch | Framework-provided, not novel; keep capability | C4/C5/C2 (depend on it) |
| C13 Coverage XBRL refresh | Sync / Priority 0 not novel | n/a |
| C14 Cluster + price-action | External service required (price data) | C2 insider-cluster |
| C15 Exhibit-graph cross-file | Not weekly-use; better as README recipe | Table-stakes #5 filings exhibits |
| C16 Recent IPO watch | Thin filter on filings.form='S-1' | C1 watchlist items, C7 late-filers |
