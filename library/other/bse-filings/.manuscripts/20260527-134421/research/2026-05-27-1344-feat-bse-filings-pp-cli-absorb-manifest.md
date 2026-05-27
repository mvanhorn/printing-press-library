# BSE Filings — Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Corporate announcements feed | bse.actions / bsedata / bsescraper | `AnnSubCategoryGetData/w` synced to SQLite | offline FTS5, per-scrip cursor, --json |
| 2 | Result calendar (forthcoming results) | bse.resultCalendar | `Corpforthresults/w` | holdings-filtered, date windowing |
| 3 | Forthcoming corp actions (board mtg/AGM/dividend) | bse DefaultData / nse-bse-mcp | `DefaultData/w` | holdings-filtered |
| 4 | Scrip-code lookup (name/ISIN → code) | bse.getScripCode / PeerSmartSearch | `PeerSmartSearch/w` | drives `holdings add` |
| 5 | Results snapshot numbers | bse.resultsSnapshot / TabResults_PAR | `TabResults_PAR/w` | join to outcomes |
| 6 | Quote / OHLC | bse.quote / bsedata / MCP servers | `getScripHeaderData/w` | context command |
| 7 | Concall transcript retrieval | awesome-stock-skills fetch-concalls | PDF `AttachLive` fetch+parse → store | offline, portfolio-wide, no screener.in dep |
| 8 | Equity metadata (sector) | bse ComHeadernew | `ComHeadernew/w` | holdings sector tagging |
| 9 | MCP exposure of all commands | nse-bse-mcp / Live-NSE-BSE-MCP / finstack-mcp | runtime cobratree mirror | every command typed + agent-native |
| 10 | Holdings CRUD | (custom) | `holdings add/list/remove` + SQLite | seeds portfolio universe |

No stubs. All absorbed features back onto a confirmed-reachable endpoint or the local store.

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Portfolio concall grep | `concall-grep <phrase> [--sector X] [--quarter QN]` | 9 | FTS5 over local `concall_chunks` joined to holdings — BSE has no transcript search |
| 2 | Thesis drift | `thesis-drift <scrip> [--terms a,b,c] [--last N] [--all]` | 9 | Per-quarter term-frequency matrix across `concall_chunks`; no API/competitor ships drift |
| 3 | Cross-holding phrase sweep | `cross <phrase> [--min-holdings 2]` | 8 | Cross-holding FTS5 join grouped by sector — sector-wide shift detector |
| 4 | Due-soon calendar | `due-soon [--days N] [--kind results,board,agm]` | 8 | Merges Corpforthresults + DefaultData + holdings filter — BSE can't combine |
| 5 | Results outcomes (2-wave join) | `outcomes [--quarter QN] [--beat\|--miss]` | 8 | Joins results numbers ↔ announcement filing on `quarter`, reconciling the 2 waves |
| 6 | Stale-thesis scan | `stale [--days N]` | 7 | Local `MAX(filed_at)` per holding vs threshold — API can't answer "who's quiet" |
| 7 | Critical-news watch | `critical [--days N]` | 8 | Portfolio-wide `CRITICALNEWS=1` (Reg-30) filter in one call |
| 8 | Concall extract (feeder) | `concall <scrip> [--quarter QN] [--mentions phrase]` | 6 | PDF→chunks in SQLite + `--mentions` paragraph filter (3 paragraphs, not 40 pages) |

Customer model + killed candidates: see `2026-05-27-1344-novel-features-brainstorm.md`.
