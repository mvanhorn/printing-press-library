# BoardGameGeek CLI Absorb Manifest

## Source Tools Cataloged

| Tool | Type | Stars | Coverage | Notes |
|------|------|-------|----------|-------|
| lcosmin/boardgamegeek | Python library | 230 | search, thing, collection, user, plays, guild, hot | Most-starred; the reference client (no terminal binary) |
| philsstein/bgg_api | Python library | 90 | search, thing, collection, plays, user | Library, no CLI |
| beefsack/go-bgg | Go library | 22 | search, thing, collection | The only Go client — a library, not a CLI |
| Nuztalgia/bgg | Python CLI | 5 | collection-focused | The ONLY existing terminal CLI — narrow scope |
| MarkusFox/bgg-mcp | MCP server (TS) | 1 | thin wrapper | Only MCP surface in the field; 1 star, thin |

**BGG publishes an official XMLAPI2** (unlike Metacritic), but every tool above is a library or a narrow wrapper. The XML responses are why a generated CLI is novel: it rides the generator's new `xml` response_format to normalize XML to JSON.

## The Gap
There is no credible terminal-native, agent-friendly BoardGameGeek CLI covering the full read surface. The Go option is a library; the only existing CLI is a narrow 5-star collection tool; the only MCP server is a 1-star thin wrapper. This entry fills that gap from one host (`boardgamegeek.com/xmlapi2`) with `--json`/`--select` everywhere and a full MCP surface.

## Absorbed (match or beat what exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Game search | lcosmin (library) | `searches` | Terminal-native, `--json --select`, exact/type filters |
| 2 | Game detail + stats | lcosmin / philsstein | `thing --stats 1` | Full record + rating/rank/ownership, batch ids, versions/videos/comments flags |
| 3 | Hot rankings | lcosmin | `hot` | Live hot list, all hot types (boardgame/rpg/videogame/person/company) |
| 4 | User profile | lcosmin | `user` | Buddies, guilds, hot/top lists via 0/1 flags |
| 5 | Collection | Nuztalgia/bgg (CLI) | `collection` | Full status-flag filtering (own/rated/played/wishlist/want/trade) + stats |
| 6 | Plays | philsstein | `plays` | By user or by game, date-range filtered, paginated |
| 7 | Family / guild | lcosmin | `family`, `guild` | Family grouping + guild roster (paginated members) |
| 8 | MCP surface | MarkusFox/bgg-mcp (1★ thin) | `boardgamegeek-pp-mcp` | 8 tools mirroring the Cobra tree at runtime, not a hand-thin wrapper |
| 9 | Health / diagnostics | (none) | `doctor` | API reachability + auth/config validation |

## Transcendence (local data layer — scaffolded)

| # | Feature | Command | Status | Note |
|---|---------|---------|--------|------|
| 1 | Full-text game search | `search` | Scaffolded | FTS5 over synced things; offline rows need the `@id`→`id` map to be addressable |
| 2 | Local analytics | `analytics` | Scaffolded | Group-by/count over synced rows; same dependency |
| 3 | Compound workflows | `workflow` | Present | Chains search to thing-detail to collection in one call |
| 4 | Offline sync | `sync` | Partial (gap) | Rows sync but lack an extractable id (XML `@id`); tracked as the primary follow-up |
