# PSX CLI — Ecosystem Absorb Manifest

Run: 20260819-222022-b3c3479a · Sources: dps.psx.com.pk (primary) + www.psx.com.pk (secondary)

## Absorb Manifest — Absorbed (match or beat everything that exists)

Sources surveyed: `mtauha/psxdata` (Python lib, 19*), `ahad-raza24/PSX-MCP-Server` (12 MCP tools),
`revolutionarybukhari/psx-mcp` (9 MCP tools, dividend-focused), `ahmedraza-96/psx-mcp-server`,
`AbdulSami455/PSX-Data-Api` (FastAPI), `psx-data-reader` (PyPI). **No CLI exists for PSX in any
language.** No tool keeps a local store; all are stateless fetch-and-return.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | All listed tickers | psxdata `tickers()` | psx-pp-cli symbols list | Offline after sync, `--json`, sector/ETF/debt filters |
| 2 | Symbol + sector metadata | psxdata `symbols()` | psx-pp-cli symbols list | Same call; adds `--sector`, `--etf`, `--debt` predicates |
| 3 | Symbol search | psx-mcp `search_symbols` | psx-pp-cli symbols search | FTS5 offline; upstream search is client-side JS only |
| 4 | Live quote | psxdata `quote()` / psx-mcp `get_quote` | psx-pp-cli quote | `--json`, `--select`, typed exit codes, cached fallback |
| 5 | Intraday tick series | PSX-MCP `intraday(symbol)` | psx-pp-cli intraday | Persisted to SQLite; upstream keeps nothing |
| 6 | Historical OHLCV | psxdata `stocks()` / PSX-MCP `history()` | psx-pp-cli history | Client-side date filter (upstream ignores date params) |
| 7 | Date-range history | PSX-MCP `date_range(sym,start,end)` | (behavior in psx-pp-cli history) `--from` / `--to` | Honest: upstream returns ALL rows; we filter locally |
| 8 | Time-range intraday | PSX-MCP `time_range(sym,t1,t2)` | (behavior in psx-pp-cli intraday) `--from` / `--to` | Local windowing over persisted ticks |
| 9 | OHLCV bars | PSX-MCP `ohlcv(symbol)` | (behavior in psx-pp-cli history) `--format ohlcv` | Same data, agent-native shape |
| 10 | Multi-symbol OHLCV | PSX-MCP `multi_ohlcv(symbols)` | (behavior in psx-pp-cli history) comma-separated symbols | Concurrent fan-out with partial-failure accounting |
| 11 | Price at timestamp | PSX-MCP `price_at_time(sym,ts)` | psx-pp-cli price-at | Answered from local ticks; no network round-trip |
| 12 | Volume analysis | PSX-MCP `volume_analysis(sym,days)` | (behavior in psx-pp-cli history) `--volume-stats` | Multi-day stats over the local mirror |
| 13 | Whole-market snapshot | PSX-MCP `market_data()` | psx-pp-cli market watch | Header-name-driven HTML parse (not positional) |
| 14 | Top gainers | PSX-MCP `gainers(limit)` | psx-pp-cli market performers | One command, `--kind gainers/losers/active` |
| 15 | Top losers | PSX-MCP `losers(limit)` | (behavior in psx-pp-cli market performers) `--kind losers` | Shares one endpoint |
| 16 | Market status (open/closed) | psx-mcp `get_market_status` | psx-pp-cli market status | Adds freshness age of last tick |
| 17 | Sector aggregates | psxdata `sectors()` | psx-pp-cli sectors list | 37 sectors, offline, sortable |
| 18 | Per-sector constituents | PSX-MCP `sector(sector)` | psx-pp-cli sectors show | Local join symbols x sector |
| 19 | Top sectors by volume | (browser-sniff `/data/top-10-sectors`) | psx-pp-cli sectors top | JSON endpoint no surveyed tool wraps |
| 20 | Index list | psx-mcp `get_indices` | psx-pp-cli indices list | All 18 index codes as typed vocabulary |
| 21 | Index constituents + weights | psxdata `indices(name)` | psx-pp-cli indices show | Weight, free-float, index points |
| 22 | Stock screener | psxdata `/screener` page | psx-pp-cli screener | PE, div yield, mkt cap, free float as real flags |
| 23 | Dividend screening | psx-mcp `screen_dividend_stocks` | (behavior in psx-pp-cli screener) `--min-dividend-yield` | Composable with every other screen predicate |
| 24 | Dividend history | psx-mcp `get_dividend_history` | psx-pp-cli payouts | Server-paginated (`count`/`offset`), persisted |
| 25 | Upcoming dividends | psx-mcp `get_upcoming_dividends` | (behavior in psx-pp-cli payouts) `--upcoming` | Date-filtered over the local payout mirror |
| 26 | Dividend buy deadline | psx-mcp `get_buy_deadline` | psx-pp-cli payouts deadline | Ex-date arithmetic, surfaced as a first-class command |
| 27 | Company announcements | psx-mcp `get_announcements` | psx-pp-cli announcements | All 5 streams; 222k-row corpus, FTS offline |
| 28 | Financial reports / fundamentals | psxdata `fundamentals()` | psx-pp-cli financials | Per-symbol report index |
| 29 | Debt market instruments | psxdata `debt_market()` | psx-pp-cli debt list | Coupon, maturity, yields |
| 30 | Debt-market movers | (browser-sniff `/debt-performers`) | (behavior in psx-pp-cli market performers) `--market debt` | Unwrapped by every surveyed tool |
| 31 | Margin-eligible scrips | psxdata `eligible_scrips()` | psx-pp-cli eligible-scrips | Offline, joinable against holdings |
| 32 | Trading board / order depth | psxdata `RealtimeScraper` | psx-pp-cli board | `--market REG,ODL,DFC,SQR,CSF` x `--board main,gem,bnb` |
| 33 | AGM/EOGM calendar | (browser-sniff `POST /calendar`) | psx-pp-cli calendar | JSON endpoint; no surveyed tool has it |
| 34 | Circuit breakers | (browser-sniff `/circuit-breakers`) | psx-pp-cli circuit-breakers | Unwrapped elsewhere |
| 35 | Listing status | (browser-sniff `/listings-table/{board}/{status}`) | psx-pp-cli listings | Board/status parameterised |
| 36 | Corporate briefings | (browser-sniff `/corporate-briefing`) | (behavior in psx-pp-cli announcements) `--stream briefing` | Fifth announcement stream |
| 37 | REST service over the data | `psxdata-api` FastAPI | (behavior in psx-pp-cli mcp) MCP server + `--json` everywhere | No server to deploy; agent-native by default |


### Transcendence (only possible with our approach)

All 8 are `hand-code` — the generator emits none of them.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Corporate-action digest | actions --watchlist --since 7d | hand-code | Unions 4 locally-mirrored feeds (announcements 222k rows, payouts, AGM calendar, circuit breakers) against local watchlist state. No surveyed tool wraps any of them. | Use this command for a joined digest of announcements, payouts, AGM dates and breaker events across the symbols on your watchlist. Do NOT use it to search the full announcement corpus by keyword or date; use 'announcements' instead. Do NOT use it to compute a single dividend's buy deadline; use 'payouts deadline' instead. |
| 2 | Snapshot delta | diff --since 7d | hand-code | Compares two retained local snapshots of quotes + screener_rows. Upstream is a current view by construction; all 6 surveyed tools are stateless. | Use this command to see what changed between two points in time across the market or your watchlist. Do NOT use it for the current-state market table; use 'market watch' instead. Do NOT use it for a full OHLCV series of one symbol; use 'history' instead. |
| 3 | Local watchlist | watchlist add / watchlist show | hand-code | PSX has no accounts; /watchlist is a static homepage fragment, not user state. Local SQLite gives the portal a concept of "you" it structurally lacks. | Use this command to manage and price your saved symbol set. Do NOT use it for a one-off price on a symbol you do not track; use 'quote' instead. Do NOT use it for the whole-market table; use 'market watch' instead. |
| 4 | Baseline-relative anomaly scan | unusual --baseline 30d --top 20 | hand-code | Ranks deviation from each name's own trailing median/MAD from local eod_bars. /performers gives raw ranks with no baseline. Mechanical, no LLM. | Use this command to find names trading abnormally versus their own history. Do NOT use it for the plain top gainers/losers/most-active ranking; use 'market performers' instead. |
| 5 | Sector rotation | rotation --window 30d --top 5 | hand-code | Ranks 37 mirrored sector aggregates by N-day change and attributes each move to top constituents. /data/top-10-sectors is volume-only and point-in-time. | Use this command to rank sectors by movement over a window and see which constituents drove each one. Do NOT use it for the current top-10-by-volume snapshot; use 'sectors top' instead. Do NOT use it to list a single sector's members; use 'sectors show' instead. |
| 6 | Valuation drift | drift OGDC --metric pe --since 90d | hand-code | Per-symbol time series of PE/yield/mkt-cap/free-float from accumulated screener snapshots. /screener exposes metrics but retains nothing. | Use this command to trace one symbol's valuation metric over time. Do NOT use it to filter the universe by current metric thresholds; use 'screener' instead. |
| 7 | Futures basis | basis --market DFC --top 20 | hand-code | Cross-market spread over PSX's five-market board vocabulary (REG/ODL/DFC/SQR/CSF). Every surveyed tool touches at most one market. | Use this command to compare futures-board prices against the regular spot board. Do NOT use it to inspect bid/offer depth on a single market and board; use 'board' instead. |
| 8 | Regulatory document search | docs search | hand-code | Full-text index over the www.psx.com.pk document map (500 URLs / 384 PDFs with lastmod). The corporate site offers no search across its PDF library, and no surveyed tool touches the www host at all. | Use this command to find PSX rulebooks, listing guides and notices by keyword. Do NOT use it for company corporate-action filings; use 'announcements' instead. |

### Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Morning brief | Formatting layer over existing commands; its only new content is the delta, which is `diff`'s job. | diff |
| Cost-basis portfolio | Needs purchase prices/trade dates absent from the entire API surface. | watchlist |
| Bulk export | Global `--csv`/`--json` on `history` already dumps the mirror. | diff |
| Filing-frequency burst | Same anomaly mechanic on a lower-signal table; output is a subset of `actions`. | actions |
| Index turnover | Fails weekly-use test (recomposition is semi-annual) and cannot bootstrap — no historical-constituent endpoint. | diff |
| Tradability score | All inputs are already screener flags; the "score" is an unverifiable weighting. | unusual |
| Shariah eligibility | Reduces to a membership boolean; presenting index membership as a compliance verdict overstates the data. | unusual |
| Peer comps | Static half is `screener --select`; dynamic half is `drift`. | drift |
| Schema-drift doctor | Maintainer tooling, not a persona ritual; header-name-driven parsing already kills the bug class. | diff |

### Notes
- **DeepWiki (Step 1.5a.6) skipped.** Equivalent intelligence was extracted at higher fidelity by
  reading `psxdata/constants.py`, `psxdata/scrapers/*.py`, and `PSX-MCP-Server/src/psx_mcp/client.py`
  directly (see brief `## Codebase Intelligence`).
- **Crowd-sniff (Phase 1.8) ran and returned nothing.** npm downloads API 400; `ccxt` and
  `romdevtools` skipped on the 10 MB tarball limit; the PSX ecosystem is PyPI-based, which
  crowd-sniff does not mine. Filed as retro candidates.
- **Stubs: none.** Every row above is shipping scope.
- **www-root scope (user-approved at Phase Gate 1.5):** row 8 exists so the secondary source contributes real capability. Source: `https://www.psx.com.pk/sitemap.xml` (500 URLs, 384 PDFs, `lastmod` present).
