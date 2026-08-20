# FPI India CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Historical FY net-investment series (1992-93→present) by asset class | NSDL Yearwise.aspx RptType=5 | fpi-india-pp-cli net-investment list --period fy | Offline, filterable by year range/asset class, --json/--select, persisted forever |
| 2 | Historical CY net-investment series | NSDL Yearwise.aspx RptType=6 | fpi-india-pp-cli net-investment list --period cy | Same for calendar year |
| 3 | Monthly breakdown within a calendar year | NSDL Yearwise.aspx RptType=6&year=YYYY | fpi-india-pp-cli net-investment list --period cy --year 2024 | Drilldown; NSDL only shows one year at a time in browser |
| 4 | INR/USD currency toggle | NSDL CurrVal param | fpi-india-pp-cli net-investment list --currency usd | Same, offline |
| 5 | Quarterly net investment | NSDL QuarterlyWise.aspx | fpi-india-pp-cli net-investment list --period quarterly | Offline, persisted |
| 6 | Latest fortnight snapshot | NSDL Latest.aspx | fpi-india-pp-cli net-investment latest | Single command vs manual page visit |
| 7 | AUC country-wise (top 10) | NSDL ReportDetail.aspx (AUC country report) | fpi-india-pp-cli auc list --by country | Offline, historical snapshots retained (NSDL shows current only) |
| 8 | AUC category-wise | NSDL ReportDetail.aspx (AUC category report) | fpi-india-pp-cli auc list --by category | Same |
| 9 | Fortnightly sector-wise FPI investment | NSDL ReportDetail.aspx (sector report) | fpi-india-pp-cli sector list | Offline, filterable, trendable |
| 10 | Trade-wise equity data | NSDL ReportDetail.aspx (trade equity report) | fpi-india-pp-cli trades list --asset equity | Offline |
| 11 | Trade-wise debt data | NSDL ReportDetail.aspx (trade debt report) | fpi-india-pp-cli trades list --asset debt | Offline |
| 12 | List of registered FPIs | NSDL RegisteredFIISAFPI.aspx | fpi-india-pp-cli registry list | Offline, searchable |
| 13 | FPI category-wise registration counts | NSDL ReportDetail.aspx (registration report) | fpi-india-pp-cli registry categories | Offline |
| 14 | DDP-wise pendency of FPI applications | NSDL ReportDetail.aspx (DDP pendency report) | fpi-india-pp-cli registry pendency | Offline |
| 15 | Foreign investment limit monitoring | NSDL ForeignInvestmentLimitMonitoringListing.aspx | fpi-india-pp-cli limits list | Offline |
| 16 | Local sync + offline SQL/search across all synced FPI data | (none — no existing tool offers this) | fpi-india-pp-cli sync / sql / search | Full offline analytical layer; nselib (closest competitor) is a bare Python function, no persistence, no CLI, no CDSL |
| 17 | CDSL daily snapshot cross-check | CDSL latest_DDMMYYYY.xls files | fpi-india-pp-cli cdsl latest | Only tool combining both depositories |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|-------------------|
| 1 | Net-investment trend | net-investment trend --asset equity --period fy --window 12 | hand-code | Rolling-window growth-rate computed over synced net_investment rows in local SQLite; no source exposes this | Use for a derived rolling growth-rate/direction over N periods. Do NOT use for raw historical rows; use 'net-investment list' for that. |
| 2 | Net-investment year-over-year comparison | net-investment yoy --asset equity --year 2024 | hand-code | Local join of the requested year's row against the prior year's row from synced net_investment; NSDL only shows one year at a time | none |
| 3 | Buying/selling streak detection | net-investment streaks --asset equity | hand-code | Local ordered scan of synced net_investment for sign-change points and current/historical streak lengths; deterministic, dogfood-verifiable | Use for consecutive-direction run length and the latest flip point. Do NOT use for raw period values; use 'net-investment list'. |
| 4 | Sector rotation top movers | sector rotation --top 5 --period latest | hand-code | Period-over-period delta computed and ranked from synced sector_flow; NSDL's sector report shows one fortnight's absolute values only | Use for ranked period-over-period sector deltas. Do NOT use for a single period's absolute figures; use 'sector list'. |
| 5 | AUC historical trend | auc trend --by country --country Mauritius | hand-code | Time-series over retained synced AUC snapshots — a view impossible from NSDL's current-only page | Use for custody-share change across >=2 synced snapshots. Do NOT use for a single latest/specific snapshot; use 'auc list'. |
| 6 | Foreign investment limit utilization | limits utilization --sector Banking | hand-code | Local join of synced limits table against synced net-investment/AUC totals to compute %-of-cap; no NSDL page performs this join | Use for %-of-cap derived from a cross-table join. Do NOT use for raw limit figures; use 'limits list'. |
| 7 | Historical extremes | net-investment extremes --asset equity --top 10 | hand-code | Local ORDER BY magnitude over the full synced 1992-93->present series; deterministic, dogfood-verifiable | none |
| 8 | NSDL/CDSL discrepancy flagging | cdsl diff --date 09072026 | hand-code | Local join/diff of same-date rows from synced NSDL-latest and synced CDSL-latest tables; the source pages never compare themselves | Use for a same-date mismatch report between the two sources. Do NOT use for a single source's raw snapshot; use 'cdsl latest'. |

## Customer Model

**Rhea — sell-side equity strategist.** Today she opens Yearwise.aspx by hand every Monday, toggles CurrVal between INR/USD, and copies the table into a manually-maintained Excel tab. Her weekly ritual: writes the "FPI flow commentary" section of a client note — this fortnight's equity/debt net flow vs last fortnight, vs the same fortnight last year, plus a cumulative-total callout. Frustration: YoY comparison means loading two separate pages and computing the delta by hand every week.

**Vikram — macro / EM capital-flows researcher.** Maintains a personal spreadsheet updated by hand from Yearwise.aspx and QuarterlyWise.aspx. Weekly ritual: checks AUC country-wise/category-wise breakdown and fortnightly sector-wise rotation for early signs of a shift. Frustration: NSDL's AUC and sector pages show only the current snapshot — no historical trend view without manually screenshotting and diffing every week.

**Ismat — financial journalist covering Indian markets.** Pulls Latest.aspx for the fortnight number, then scrolls the full historical table by eye to find streaks ("seventh straight fortnight of selling") and superlatives ("largest outflow since 2020") for Friday stories. Frustration: NSDL computes none of this; scanning 30+ years of rows on deadline is slow and error-prone.

**Dev — quant / algo builder.** Writes throwaway Python scrapers against NSDL/CDSL since neither offers an API or persistence. Weekly ritual: refreshes sector-rotation signals before market open and checks which sectors/debt categories are near their regulatory FPI limit (a breach can trigger a halt). Frustration: cross-checking NSDL vs CDSL means manually diffing two differently-shaped pages, with nothing flagging disagreement or near-cap categories programmatically.

## Killed Candidates

| Feature | Kill reason |
|---------|-------------|
| Net-investment cumulative growth window | Duplicate of the cumulative-total column already in `net-investment list`; folded into `trend`'s window computation. |
| Buy/sell flip detection | Sibling of streak detection; merged into `net-investment streaks` to avoid two near-identical time-series-diff commands. |
| FPI registry growth trend | No explicit evidence of demand in brief/personas; scored 4/10, below the 5/10 bar. |
| Arbitrary two-period compare | Subsumed by `net-investment yoy`, which covers the evidenced adjacent-year use case without added ambiguity. |
| Flow forecast | Fails LLM-dependency/verifiability kill check — forward-looking projection can't be mechanically verified and risks reading as investment advice. |
| Sector/AUC correlation | Fails verifiability kill check — cross-domain statistical correlation needs domain judgment to interpret; no deterministic way to confirm correctness. |

## Hand-code Count

**8 of 8 transcendence rows are hand-code.** There is no official API for this domain, so nothing is spec-emitted beyond generated list/get scaffolding over already-synced local tables (which is absorbed-feature plumbing, not transcendence). All 8 transcendence rows require hand-written Go: SQLite window/join/scan logic plus Cobra wiring in root.go, roughly 50-150 LoC each.
