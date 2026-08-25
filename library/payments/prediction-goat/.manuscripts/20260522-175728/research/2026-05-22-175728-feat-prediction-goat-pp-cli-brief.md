# prediction-goat CLI Brief

## API Identity
- Domain: Prediction markets (event/outcome odds), read-only by design
- Users: AI agents and humans who want to find the odds on a topic without trading
- Data profile: ~50K active Polymarket markets across ~5K events, ~5K Kalshi markets across ~1K events. Markets have many fields (Polymarket: 91, Kalshi: 44) but a handful of high-gravity ones (slug, title, yes/no prices, volume, liquidity, end date, tags)

## Source Priority
- Primary: **Polymarket** (Gamma OpenAPI, 42 endpoints, free, no auth needed)
- Secondary: **Kalshi** (OpenAPI 3.0, 85 endpoints, free for public read; portfolio/orders/balance require auth and are OUT OF SCOPE for this read-only CLI)
- Economics: Both sources are 100% free for our read-only scope. No paid keys involved. Auth is **not required** for either source's market data, events, series, or order book reads.
- Inversion risk: None. Polymarket lacks a fielded full-text "topic" affordance that ranks the full hierarchy, but its /public-search endpoint exists and covers events/tags/profiles. Kalshi lacks a free-text search endpoint at all — the user-typed `topic <name>` will use **local FTS5** as the unifying surface. Polymarket leads README & headline `topic` results; Kalshi appears in the same bundle for cross-market comparison.

## Reachability Risk
- **None.** Both APIs probed via printing-press probe-reachability return `standard_http` with confidence 0.95. Plain stdlib HTTP works. No browser-clearance, WAF, or rate-limit signals observed.

## Top Workflows
1. **Topic odds search**: `prediction-goat topic kanye-west` → ranked bundle of every Polymarket + Kalshi market/event/series mentioning the topic, with current % odds, volume, end date. **Killer feature.**
2. **Screen discovery**: `trending`, `resolving --week`, `liquid --min-volume 100k`, `mispriced --threshold 0.05`, `new --days 7` — six screens that answer "what should I be watching right now" without scraping JSON
3. **Cross-market comparison**: `compare "AZ basketball" --venues polymarket,kalshi` — same topic, both venues, side-by-side YES/NO prices and implied probability
4. **Drill-down**: `markets get <slug-or-ticker>` and `markets prices <slug> --interval 1d` for price history sparkline
5. **Sync + offline SQL**: `sync` once, then `sql "SELECT ..."` and `search "..."` work instantly without network. Power-users compose joins across markets + events + tags.

## Table Stakes (what the official CLIs / MCPs already do)
- Polymarket official CLI (`Polymarket/polymarket-cli`): list/get for markets, events, tags, series, sports, comments, profiles; CLOB price/midpoint/spread/book/last-trade/price-history; full trading & wallet flows (we DROP these). Search is `markets search "bitcoin"` returning the raw 91-field rows with no rank tuning.
- shaanmajid/prediction-mcp: kalshi_list_markets/get_market/get_orderbook/get_trades/search; polymarket_list_markets/get_market/get_orderbook/get_price. Has local FTS cache. **Tool design issue**: separate tools per platform forces the agent to fan out. No unified `topic` tool.
- JamesANZ/prediction-market-mcp: one tool `get-prediction-markets` across Polymarket+PredictIt+Kalshi. No local caching, no screens.
- 6+ Polymarket-only MCPs (pab1it0, berlinbra, CarlosIbCu, IQAIcom, joinQuantish, caiovicentino, etc.) — most are read-only Gamma/CLOB wrappers, one (caiovicentino) is a 45-tool trading server.
- qtzx06's open PRs on the official CLI add: `--fields` JSON filter (#53), volume/liquidity/date/tag filters on list (#52), balance cache refresh (#55). #52 and #53 are upstream catching up to what slim output and rich filtering should look like — neither addresses search or topic bundling.

## Data Layer
- Primary entities (PM Gamma): markets, events, tags, series, sports, teams
- Primary entities (Kalshi): markets, events, series
- Sync cursor: PM has keyset pagination (`/markets/keyset`, `/events/keyset` with `after_cursor`); Kalshi has cursor field in response (`cursor` returned, passed as next cursor)
- FTS5 index: a unified `markets_fts` table indexes (slug, title, description, tags joined, series joined, venue) across BOTH sources. This is what makes `topic` instant.
- Estimated sync: ~150MB JSON before slim projection, ~20-30MB after slim normalization into SQLite

## Codebase Intelligence
- Polymarket Gamma OpenAPI spec at `https://docs.polymarket.com/api-spec/gamma-openapi.yaml`: 42 endpoints. /public-search supports `q`, `events_status`, `events_tag`, `keep_closed_markets`, `sort`, `ascending`, `search_tags`, `search_profiles`, `recurrence`. /markets supports `liquidity_num_min/max`, `volume_num_min/max`, `start_date_min/max`, `end_date_min/max`, `tag_id`, `order`, `ascending`, plus arrays for `id`, `slug`, `clob_token_ids`, `condition_ids`. No auth needed for read endpoints. Server at `https://gamma-api.polymarket.com`.
- Kalshi OpenAPI spec at `https://docs.kalshi.com/openapi.yaml`: 85 endpoints, **read-only subset** for our scope: /markets, /markets/{ticker}, /markets/{ticker}/orderbook, /markets/trades, /markets/candlesticks, /events, /events/{event_ticker}, /events/{event_ticker}/metadata, /series, /series/{series_ticker}, /exchange/status, /search/tags_by_categories, /search/filters_by_sport, /multivariate_event_collections. Servers: `https://api.elections.kalshi.com/trade-api/v2` (preferred for elections markets) and `https://external-api.kalshi.com/trade-api/v2`.

## User Vision
> Combo Polymarket / Kalshi CLI focused on odds/prediction, not executing trades. The killer surface is `topic <name>` (kanye-west, argentina, chatgpt-5) which returns a slim ranked bundle of every related market/event/tag in one ~3KB call instead of upstream's ~250KB firehose. Screens (trending, resolving --week, liquid, mispriced, new) cover the other common agent intents. Slim-by-default output (12 fields, not 156) with --profile full|prices and --fields a,b,c overrides. MCP server exposes all of it as tools. Read-only by design and by CI lint — the binary structurally cannot trade.

## Product Thesis
- Name: `prediction-goat-pp-cli` (binary), library slug `prediction-goat`, library category `payments` (sibling of `kalshi`)
- Why it should exist:
  1. Official Polymarket CLI returns 91 fields per market row; one `events?limit=10&tag_slug=politics` call is 205KB before any rendering. That's catastrophic agent context cost. A slim profile (12 fields) cuts response payload ~80x.
  2. No existing tool offers a unified `topic` bundle that ranks markets/events/tags across BOTH Polymarket and Kalshi. The best Polymarket search ranks closed markets alongside active ones; the best Kalshi has is filtering by category/sport.
  3. No existing tool has a local SQLite+FTS5 mirror of both venues. That's the foundation for every transcendence feature: cross-venue diff, sparkline history of /odds, mispriced screens, NULL-friendly composability via `sql` and `search`.
  4. No existing tool ships an MCP server that exposes screens + topic as code-orchestrated tools (search+execute pair), which is exactly the shape big-surface APIs need for agents.
  5. CI lint guarantees read-only. The binary structurally cannot trade — no signing code, no L1/L2 keys, no `post-order` paths. This makes it safe to ship as an MCP server inside someone's agent without trading-risk approvals.

## Build Priorities
1. Foundation: sync (PM + Kalshi → local SQLite with markets, events, tags, series, fts5 index), with cursor resume
2. Killer feature: `topic <name>` (unified ranked bundle, slim by default, --profile full for the full 91/44-field shape)
3. Screens: `trending`, `resolving [--week|--month|--days N]`, `liquid [--min-volume X]`, `mispriced [--threshold P]`, `new [--days N]`
4. Cross-venue: `compare <topic>` and `markets diff <pm-slug> <kalshi-ticker>` showing implied probabilities side-by-side
5. Drill-down: `markets get`, `markets prices --interval`, `events get`, `tags list/get/related`
6. SQL + search composability: existing PP scaffolding gives us `sql`, `search`, `context`, `agent-context`
7. MCP server: code-orchestrated pair `prediction_goat_search` + `prediction_goat_execute` + a curated set of intents (topic, screens, compare). Endpoint-mirror tools hidden.
8. CI lint: a `nolint:trade` policy check ensures no PR adds a trading endpoint, signing import, or wallet code

## Auth Profile
- **No auth required.** Both APIs work fully read-only against public endpoints. The CLI ships no auth subcommand for v1. Future: optional `KALSHI_API_KEY` only IF we later add portfolio/positions views — but per user vision, that's out of scope.
