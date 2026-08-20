# prediction-goat Absorb Manifest

> Source ordering (confirmed): Polymarket primary, Kalshi secondary.
> Combo CLI; both APIs free + no auth required for read-only scope.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List markets with rich filters | Polymarket Gamma /markets (liquidity_num_min/max, volume_num_min/max, dates, tag_id, order, ascending) | `markets list --venue all|polymarket|kalshi` with unified filter set | Cross-venue list, slim-by-default (12 fields), `--json`/`--select`/`--csv`/`--profile slim|full|prices` |
| 2 | Get market by id/slug/ticker | PM Gamma /markets/{id}, /markets/slug/{slug}; Kalshi /markets/{ticker} | `markets get <id-or-slug-or-ticker>` auto-routes by ID shape; `--venue` override | One command, both venues, slim default |
| 3 | Text search markets | PM Gamma /public-search; shaanmajid kalshi_search | `search "<query>"` over local FTS5 (markets + events + tags from both venues) | Instant, offline, cross-venue, slim output |
| 4 | List events | PM Gamma /events, Kalshi /events | `events list --venue all` with tag/category/status filters | Unified event surface, slim profiles |
| 5 | Get event by id/slug/ticker | PM Gamma /events/slug/{slug}; Kalshi /events/{event_ticker} | `events get <id-or-slug-or-ticker>` venue auto-detect | One command for both venues |
| 6 | List tags / categories | PM Gamma /tags; Kalshi /search/tags_by_categories | `tags list --venue all` | Unified taxonomy view |
| 7 | Related tags | PM Gamma /tags/{id}/related-tags | `tags related <slug-or-id>` | Discover adjacent topics |
| 8 | List series | PM Gamma /series; Kalshi /series | `series list --venue all` | Cross-venue series view |
| 9 | Get series detail | PM Gamma /series/{id}; Kalshi /series/{series_ticker} | `series get <id-or-ticker>` | Unified series surface |
| 10 | Price history with interval | PM Gamma /markets/{id}/prices-history (1m/1h/6h/1d/1w/max); Kalshi /markets/candlesticks | `prices history <id-or-ticker> --interval ...` | Both venues, sparkline render |
| 11 | Order book snapshot | PM CLOB /book; Kalshi /markets/{ticker}/orderbook | `orderbook <id-or-ticker>` | Both venues, slim output |
| 12 | Last trade price | PM CLOB /last-trade-price; Kalshi last_price field | `prices last <id-or-ticker>` | Both venues, single command |
| 13 | Best bid/ask/midpoint | PM CLOB /midpoint, /spread, /price | `prices best <id-or-ticker>` returns yes/no bid/ask/midpoint | One call vs 3 with Polymarket CLI |
| 14 | Filter list output to fields | qtzx06 PR #53 (PM CLI, unmerged) | `--select field1,field2` + `--profile slim|full|prices` | Already shipping, not aspirational. Slim profile = 12 high-gravity fields |
| 15 | Sort/order list output | PM CLI `--order field` `--ascending` | Same flags, same semantics; default = volume desc | Sensible defaults |
| 16 | List comments | PM Gamma /comments | `comments list` (Polymarket only — Kalshi has no comments) | Market commentary only |
| 17 | Sports + teams metadata | PM Gamma /sports, /sports/market-types, /teams | `sports list`, `sports market-types`, `teams list` | Polymarket-only filter helpers |
| 18 | MCP server | berlinbra/polymarket-mcp, shaanmajid/prediction-mcp | Built-in `prediction-goat-pp-mcp` with stdio + http transports; code-orchestrated `prediction_goat_search` + `prediction_goat_execute` plus intents | Cross-venue from day one, slim payloads |
| 19 | Local cache / sync | shaanmajid (in-memory) | `sync` writes to SQLite with FTS5; cursor resume across both venues | Persistent, offline-queryable |
| 20 | Exchange status / health | Kalshi /exchange/status; PM /status | `doctor` + `health` check both venues | Single cross-venue check |
| 21 | Profile lookup | PM Gamma /profiles | `profiles get <address>` | Polymarket-only |
| 22 | Sports market types | PM Gamma /sports/market-types | `sports market-types` | Helper |
| 23 | Comments by user | PM Gamma /comments/user_address/{address} | `comments by-user <address>` | Polymarket-only |
| 24 | Get market tags by id | PM Gamma /markets/{id}/tags | `markets tags <id-or-slug>` | One-call tag list |
| 25 | Tweet count for event | PM Gamma /events/{id}/tweet-count | `events tweet-count <id>` | Trivial mirror |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Topic bundle | `topic <name>` | 10/10 | hand-code | Local FTS5 over unified markets/events/tags table indexed from PM Gamma /markets+/events+/tags and Kalshi /markets+/events+/series; ranked slim ~3KB bundle | Persona A (Riley) weekly ritual; user vision names; no MCP today offers unified topic ranking; PM /public-search ranks closed markets alongside active |
| 2 | Trending screen | `trending` | 9/10 | hand-code | Local SQL ranks markets by 24h volume across both venues | Persona B (Casey) Monday ritual; user vision names; no cross-venue trending feed |
| 3 | Resolving screen | `resolving [--week|--month|--days N]` | 9/10 | hand-code | Local SQL on end_date < window joined across PM+Kalshi synced markets | Personas B + C; user vision names; PM filters end-date, Kalshi doesn't — unification needs local layer |
| 4 | Mispriced screen | `mispriced [--threshold P]` | 9/10 | hand-code | Local join pairs PM markets to Kalshi markets via topic-text/tag overlap, computes \|p_pm − p_kalshi\|, filters by threshold | Persona C (Jordan) calibration ritual; structurally impossible upstream; ~120 LoC |
| 5 | Cross-venue compare | `compare <topic>` | 9/10 | hand-code | Resolves topic to paired (PM, Kalshi) markets in local store and renders side-by-side YES/NO + implied prob | All three personas; user vision names; no existing tool offers cross-venue comparison |
| 6 | Read-only CI lint | (CI policy — `.github/workflows/read-only-lint.yml`) | 8/10 | hand-code (CI artifact, not a Cobra command) | CI lint rejects PRs that import signing libs, add CLOB order endpoints, or introduce wallet/L1-L2 key code | User vision names as structural guarantee; required to ship as agent-embeddable MCP |
| 7 | Movers screen | `movers [--window 24h|7d]` | 8/10 | hand-code | Local SQL diffs current price vs price-history snapshot at T-window across both venues | Persona B + C; distinct from volume-based `trending` |
| 8 | Liquid screen | `liquid [--min-volume X]` | 8/10 | hand-code | Cross-venue normalized liquidity/volume floor over local store | Persona C; user vision names; unification only possible locally |
| 9 | New screen | `new [--days N]` | 7/10 | hand-code | Local SQL on created-at across PM+Kalshi synced markets | Persona B newsletter; user vision names |
| 10 | Markets diff | `markets diff <pm-slug> <kalshi-ticker>` | 7/10 | hand-code | Local field-by-field structural diff between a specific PM market row and a specific Kalshi market row | Personas C + B drill-down; complements `compare` |

## Stub / deferred

None. Every survivor above is shipping scope.

## Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Calibration export | Requires settled-market resolution data not reliably free for read-only APIs | `mispriced` |
| Watchlist CRUD | Thin local CRUD; reachable via saved `sql` view | `topic` |
| Topic alerts (--watch) | Persistent background process = scope creep | `movers` |
| Topic-resolve summary | Redundant with `topic --sort end_date` (absorbed as flag) | `topic` |
| Cross-venue tag bridge | Speculative fuzzy join; verifiability low | `compare` |
| Inline sparkline on every list row | Already covered by absorb #10; violates slim-default contract | absorb #10 (`prices history --sparkline`) |
