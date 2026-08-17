# Screener.in Novel Features Brainstorm

## Customer model
1. **Arjun Mehta, 38** — buy-and-hold value investor, IT consultant in Bengaluru. Buys 4-6 stocks/year, holds for years. Weekly ritual: Saturday re-check of ~20 watchlist candidates (latest quarter vs year-ago, profit growth acceleration, pros/cons changes, valuation vs peers). Frustration: wide HTML quarterly tables to eyeball, tab-switching mental math for comparisons, no offline copy.
2. **Priya Raghavan, 31** — screen-driven stock picker, finance analyst in Hyderabad. Weekly ritual: Monday run-through of 3-4 screens, spreadsheet reconciliation, intersection lists (Magic Formula ∩ sector screen), flags for owned stocks. Frustration: 50-row pagination with no client-side sort/filter, manual symbol dedup, can't tell which hits have improving fundamentals or insider buying.
3. **Nandini Iyer, 45** — event/earnings-driven investor, ex-banker in Mumbai. Weekly ritual: Sunday evening scan of reported results with YOY, insider trades for held companies, IPO calendar. Frustration: auth-gated paginated results, long insider row list ("who is net buying most?" unanswerable), no single pulse view.
4. **Dev, 29** — builds an equity-research agent ("my fund bot"). Weekly ritual: keeps fragile Python scraper alive, reruns screens-to-candidates query, pipes rows to LLM. Frustration: no official API, no stable JSON, no local store, wants ranked agent-ready JSON.

## Candidates (pre-cut)
1. Company compare `compare "TCS" "HDFCBANK"` — (c) cross-entity. KEEP.
2. Quarterly trend `qtrend "TCS"` — (b)+(a). KEEP.
3. Screen overlap `overlap <A> <B>` — (c). KEEP.
4. Screen rank `rank <screen>` — (c). KEEP.
5. Insider flow `insider-flow` — (b)+(c). KEEP.
6. Pros/cons scan `scan <kw>` — (b). KILL (cache-depth gated, occasional use).
7. Market pulse `pulse` — (b). KILL (wrapper reassembly).
8. Technical screen scan `techscreen` — (c). KILL (poor feasibility, off-identity).
9. Peer chain `peers-of` — (c). KILL (gimmick, not weekly).
10. Watch mode `--watch` — (b). KILL (scope creep).
11. Company brief `brief` — (a). KILL (wrapper; --agent covers it).
12. Deterioration scan `weakening` — (c). KILL (cache-depth dependent).

## Survivors and kills
### Survivors (all hand-code, 7-8/10)
1. compare — 8/10 — joins company_financials + trades in local SQLite
2. qtrend — 8/10 — computes YOY/margin drift/flags from synced quarterly table
3. overlap — 7/10 — intersects screen result tables by symbol
4. rank — 7/10 — re-scores screen with composite + insider flow
5. insider-flow — 7/10 — aggregates trades into net per-company flows

### Killed
| Feature | Kill reason | Sibling |
|---|---|---|
| scan | cache-depth gated, occasional | qtrend |
| pulse | wrapper reassembly | insider-flow |
| techscreen | poor feasibility, off-identity | rank |
| peers-of | gimmick, not weekly | compare |
| watch mode | scope creep | overlap |
| brief | wrapper; --agent covers | compare |
| weakening | cache-depth dependent | qtrend |
