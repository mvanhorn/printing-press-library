# Pinterest CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | Get user info | collactivelabs MCP | pinterest-pp-cli user get | --json, offline cache |
| 2 | List boards | collactivelabs MCP | pinterest-pp-cli boards list | --json, FTS, offline |
| 3 | Create board | collactivelabs MCP | pinterest-pp-cli boards create | --dry-run, --json |
| 4 | Get board | collactivelabs MCP | pinterest-pp-cli boards get | --json, offline |
| 5 | Update board | Pinterest API | pinterest-pp-cli boards update | --dry-run |
| 6 | Delete board | Pinterest API | pinterest-pp-cli boards delete | --dry-run, confirmation |
| 7 | List board pins | collactivelabs MCP | pinterest-pp-cli boards pins | paginated, --json |
| 8 | List board sections | Pinterest API | pinterest-pp-cli boards sections list | --json |
| 9 | Create board section | Pinterest API | pinterest-pp-cli boards sections create | --dry-run |
| 10 | List pins | collactivelabs MCP | pinterest-pp-cli pins list | --json, FTS, offline |
| 11 | Create pin | collactivelabs MCP | pinterest-pp-cli pins create | --dry-run, --json |
| 12 | Get pin | collactivelabs MCP | pinterest-pp-cli pins get | --json, offline |
| 13 | Update pin | Pinterest API | pinterest-pp-cli pins update | --dry-run |
| 14 | Delete pin | Pinterest API | pinterest-pp-cli pins delete | --dry-run |
| 15 | Save pin to board | Pinterest API | pinterest-pp-cli pins save | --dry-run |
| 16 | Pin analytics | Pinterest API | pinterest-pp-cli pins analytics | --json, date range |
| 17 | Search pins | terryso/mcp-pinterest | pinterest-pp-cli search pins | API + local FTS combined |
| 18 | Search boards | Pinterest API | pinterest-pp-cli search boards | --json |
| 19 | Export board to JSON | brtdwchtr/pinterest-export | pinterest-pp-cli boards export | official API (not scraping) |
| 20 | Export board to Markdown | brtdwchtr/pinterest-export | pinterest-pp-cli boards export --format md | LLM-ready, no Playwright needed |
| 21 | Image download/cache | brtdwchtr/pinterest-export | pinterest-pp-cli boards export --cache-images | official URLs, faster |
| 22 | List ad accounts | Pinterest Ads API | pinterest-pp-cli ads accounts list | --json |
| 23 | List campaigns | Pinterest Ads API | pinterest-pp-cli ads campaigns list | --json, filter by status |
| 24 | Get campaign analytics | Pinterest Ads API | pinterest-pp-cli ads campaigns analytics | date range, --json |
| 25 | List ad groups | Pinterest Ads API | pinterest-pp-cli ads groups list | --json |
| 26 | List ads | Pinterest Ads API | pinterest-pp-cli ads list | --json |
| 27 | Ad analytics | Pinterest Ads API | pinterest-pp-cli ads analytics | --json |
| 28 | List audiences | Pinterest Ads API | pinterest-pp-cli ads audiences list | --json |
| 29 | Trending topics | Pinterest API | pinterest-pp-cli trending | region filter, --json |
| 30 | User followers | Pinterest API | pinterest-pp-cli user followers | paginated, --json |
| 31 | List followed boards | Pinterest API | pinterest-pp-cli user boards | --json |
| 32 | List media uploads | Pinterest API | pinterest-pp-cli media list | --json |
| 33 | Upload media | Pinterest API | pinterest-pp-cli media upload | --dry-run, status poll |
| 34 | Catalog feeds list | Pinterest API | pinterest-pp-cli catalogs feeds list | --json |
| 35 | Catalog feed create | Pinterest API | pinterest-pp-cli catalogs feeds create | --dry-run |
| 36 | Catalog items list | Pinterest API | pinterest-pp-cli catalogs items list | --json |
| 37 | Sync all data | (none — no existing tool) | pinterest-pp-cli sync | boards+pins+analytics into SQLite |
| 38 | Offline SQL query | (none) | pinterest-pp-cli sql | arbitrary SQL against local store |
| 39 | Offline search | (none) | pinterest-pp-cli search | FTS5 across boards+pins locally |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Top boards by saves | top-boards | hand-code | Requires local join of boards + pin analytics snapshots | Find your highest-performing boards ranked by total saves. Do NOT use for per-pin analysis; use 'pins analytics' instead. |
| 2 | Pin performance trends | trends | hand-code | Requires time-series analytics in SQLite across multiple date windows | Track how pin impressions and saves change week-over-week. Use for content timing decisions. |
| 3 | Board gap analysis | boards gap | hand-code | Requires comparing board topic distribution against trending topics | Find which trending topics you haven't pinned in the last 30 days. |
| 4 | Ad spend vs organic reach | compare | hand-code | Requires joining ad analytics + organic pin analytics — no single API call returns both | Compare paid vs organic performance for the same content in one view. |
| 5 | Stale boards detector | boards stale | hand-code | Requires local SQLite time-windowed query on last-pin-created timestamp | List boards with no new pins in N days — actionable content calendar gap finder. |
| 6 | Best posting time | timing | hand-code | Requires aggregating engagement by day-of-week from analytics snapshots | Surface which days generate the most pin saves for your account historically. |
