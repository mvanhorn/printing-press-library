# InvestorPaisa CLI — Absorb Manifest

Competitors surveyed (no wrapper/SDK exists for investorpaisa.com itself): nse-mcp, finstack-mcp, India Market MCP, Screener MCP Server, Stocker, Indian-Stock-Exchange-MCP; public-library peers `nse-india`, `fpi-india`.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Live market snapshot (indices, breadth, rows) | nse-mcp / finstack-mcp live quotes | (generated endpoint) live get | --json/--select, offline cache |
| 2 | Stock universe list / search by symbol-name-sector | India Market MCP search | (generated endpoint) universe list | local FTS |
| 3 | Screener metrics table (48 fields) | Stocker / Screener MCP screens | (generated endpoint) screener rows | local SQLite, sortable |
| 4 | Market overview + ranked signals | finstack-mcp sector performance | (generated endpoint) market overview | agent output |
| 5 | News by symbol | Screener MCP / nse-mcp news | (generated endpoint) news by-symbol | --select, local store |
| 6 | Market-wide news/video feed w/ cursor | site | (generated endpoint) news feed | pagination |
| 7 | Finance videos/shorts | site | (generated endpoint) videos list | --json |
| 8 | Watchlists list/create/delete/set-items | Kite/Groww apps | (generated endpoint) watchlists list | dry-run, scriptable |
| 9 | Bookmarks list | site | (generated endpoint) bookmarks list | --json |
| 10 | Saved layout / saved screens / profile | site | (generated endpoint) me layout | --json |
| 11 | Health check | site | (generated endpoint) health get | doctor |
| 12 | Cookie-session auth import | n/a | investorpaisa-pp-cli auth login --chrome | no password handling |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Local expression screener (score 10) | screen "close > sma50 and rs >= 70" | hand-code | The site's expression language is client-side only; we evaluate it over synced 48-metric rows | Use this to run an expression over the synced Nifty 200 metrics, with --sector, --sort and --saved. Do NOT use it for a single stock's full picture; use 'quote' instead. Do NOT use it for ranking today's moves; use 'movers' instead. |
| 2 | One-stock deep dive (10) | quote RELIANCE | hand-code | Joins universe + live + screener + news + watchlists + sector percentile in one call | Use this for the current picture of one or a few stocks. Do NOT use it for 5-year price series; use 'history' instead. Do NOT use it to scan the universe; use 'screen' instead. |
| 3 | Movers with confluence (8) | movers --kind volume --min-rs 70 | hand-code | Joins live rows (relVol, gap, 52w signals) to screener RS/trend | Use this for today's ranked gainers, losers, unusual-volume names, gappers and 52-week highs/lows. Do NOT use it for custom multi-metric filters; use 'screen' instead. Do NOT use it for index/breadth picture; use 'breadth' or 'brief' instead. |
| 4 | Sector rotation heat (8) | sectors --window 1m --trend | hand-code | Equal-weight multi-window sector returns + RS + %above DMAs, rank change vs prior snapshot | Use this to see which of the 18 sectors gain or lose strength over 1w-1y. Do NOT use it for index-wide breadth; use 'breadth' instead. Do NOT use it to find stocks in a sector; use 'screen --sector' instead. |
| 5 | Snapshot diff (8) | changes --since 1d | hand-code | Local self-join across dated snapshots: entries/exits, RS and DMA crossings, new signals | Use this to list which stocks entered or left conditions between two stored snapshots. Do NOT use it for market-level trend; use 'breadth' instead. Do NOT use it for today's ranking; use 'movers' instead. |
| 6 | Morning brief (8) | brief --watchlist "My watchlist" | hand-code | Joins watchlists, live rows, screener rows, signals, news and breadth in one agent-ready JSON | Use this for a single-call market backdrop plus watchlist rows and fresh headlines. Do NOT use it for a single stock deep dive; use 'quote' instead. Do NOT use it for state changes since an earlier snapshot; use 'changes' instead. |
| 7 | Breadth history (7) | breadth --days 30 | hand-code | The site shows only today's breadth; history exists only in local snapshots | Use this for how breadth has trended across stored snapshots. Do NOT use it for per-sector breakdowns; use 'sectors' instead. Do NOT use it for which stocks changed; use 'changes' instead. |
| 8 | Adjusted 5y history (7) | history TCS --years 5 | hand-code | Parses the 5y OHLCV + split/bonus factors + events out of the /chart/{SYM} RSC payload (no JSON endpoint exists) | Use this for a stock's 5-year daily OHLCV, adjustment factors and corporate events. Do NOT use it for current metrics or headlines; use 'quote' instead. Do NOT use it to find stocks matching a pattern; use 'screen' instead. |

Scope: 12 absorbed + 8 transcendence (all hand-code, ~50-150 LoC each plus wiring). No stubs.
