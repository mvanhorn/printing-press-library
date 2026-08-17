| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List markets with rich filters (active, closed, liquidity, volume, dates, tags) | Polymarket Gamma /markets | `markets list` covering both PM and Kalshi via `--venue` (default: both); maps PM filters AND Kalshi filters | Cross-venue list, slim-by-default (12 fields), composable with `--json` / `--select` / `--csv` |
| 2 | Get market by id/slug/ticker | Polymarket Gamma /markets/{id}, Kalshi /markets/{ticker} | `markets get <id-or-slug-or-ticker>` — auto-route by ID shape; `--venue` override | One command, both venues, slim by default |
| 3 | Text search markets | Polymarket Gamma /public-search; shaanmajid kalshi_search | `search "<query>"` — local FTS5 over synced markets from both venues, ranked | Instant, offline, cross-venue, ranks by relevance × activity, slim output |
| 4 | List events | Polymarket Gamma /events, Kalshi /events | `events list --venue all|polymarket|kalshi` with tag/category/status filters | Unified event surface, slim profiles |
| 5 | Get event by id/slug/ticker | Polymarket Gamma /events/slug/{slug}, Kalshi /events/{event_ticker} | `events get <id-or-slug-or-ticker>` with venue auto-detect | One command for both venues |
| 6 | List tags / categories | Polymarket Gamma /tags, Kalshi /search/tags_by_categories | `tags list --venue all` | Unified taxonomy view, related-tag lookup |
| 7 | Related tags | Polymarket Gamma /tags/{id}/related-tags | `tags related <slug-or-id>` | Discover adjacent topics for broader research |
| 8 | List series | Polymarket Gamma /series, Kalshi /series | `series list --venue all` | Cross-venue series view |
| 9 | Get series detail | Polymarket Gamma /series/{id}, Kalshi /series/{series_ticker} | `series get <id-or-ticker>` | Unified series surface |
| 10 | Price history with interval | Polymarket Gamma /markets/{id}/prices-history, Kalshi /markets/candlesticks | `prices history <id-or-ticker> --interval 1h|1d|1w` | Both venues, sparkline render, slim output |
| 11 | Order book snapshot | Polymarket CLOB /book, Kalshi /markets/{ticker}/orderbook | `orderbook <id-or-ticker>` | Both venues, slim output |
| 12 | Last trade price | Polymarket CLOB /last-trade-price, Kalshi market last_price field | `prices last <id-or-ticker>` | Both venues, single command |
| 13 | Market midpoint / spread / best-bid-best-ask | Polymarket CLOB /midpoint, /spread, /price | `prices best <id-or-ticker>` returning yes/no bid/ask/midpoint | Combined into one call (Polymarket CLI requires 3 calls) |
| 14 | Filter list output to fields | qtzx06 PR #53 (Polymarket CLI, unmerged) | `--select field1,field2` + `--profile slim|full|prices` on every list/get | Already shipping, not aspirational. Slim profile = 12 fields |
| 15 | Sort/order list output | Polymarket CLI `--order field` `--ascending` | Same flags, same semantics; default = volume desc on `markets list` | Sensible default, not random order |
| 16 | List comments | Polymarket Gamma /comments | `comments list` (Polymarket only — Kalshi has no comments) | Drops user-trading comments; keeps market commentary |
| 17 | Sports metadata + teams | Polymarket Gamma /sports, /teams | `sports list`, `sports market-types`, `teams list` | Polymarket-only surface, useful for sports-betting topics |
| 18 | MCP server | berlinbra/polymarket-mcp, shaanmajid/prediction-mcp, JamesANZ/prediction-market-mcp | Built-in `prediction-goat-pp-mcp` exposing screens, topic, compare; transports stdio + http | Code-orchestrated MCP (search+execute pair) instead of one-tool-per-endpoint; cross-venue from day one |
| 19 | Local cache / sync | shaanmajid/prediction-mcp (in-memory) | `sync` writes to SQLite with FTS5, cursor resume, both venues | Persistent, offline-survivable, queryable via `sql` |
| 20 | Exchange status / health check | Kalshi /exchange/status, Polymarket /status | `doctor` and `health` check both venues' status endpoints | Single command tells you if either side is down |
| 21 | Profile lookup | Polymarket Gamma /profiles | `profiles get <address>` | Polymarket-only, kept for completeness; not in killer-feature path |
| 22 | Sports market types | Polymarket Gamma /sports/market-types | `sports market-types` | Sports-specific filter helper |
| 23 | Comments by user | Polymarket Gamma /comments/user_address/{address} | `comments by-user <address>` | Drop deeper user lookups; minimal kept |
| 24 | Get market tags by id | Polymarket Gamma /markets/{id}/tags | `markets tags <id-or-slug>` | One-call tag list per market |
| 25 | Tweet count for event | Polymarket Gamma /events/{id}/tweet-count | `events tweet-count <id>` | Trivial mirror; cheap to include |
