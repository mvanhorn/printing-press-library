# Pinterest CLI Brief

## API Identity
- Domain: Visual discovery, idea bookmarking, content creation, social commerce
- Users: Content creators, marketers, brand managers, ad buyers, designers, developers
- Data profile: Boards (collections), Pins (bookmarks with media), Ads (campaigns/groups/creatives), Catalogs (product feeds), Analytics

## Reachability Risk
- Low. Official API returns 401 on unauthenticated requests (expected). No access blocking.
- OAuth2 required for all endpoints.
- Trial access tier: 1000 req/day per category. Standard access: higher limits.
- Probe endpoint: GET /v5/user_account → 401 (expected, auth-gated)

## Top Workflows
1. **Board management** — create, organize, and curate boards; add/remove pins; manage sections
2. **Pin research** — search for pins by keyword, analyze top-performing content
3. **Analytics** — track pin impressions, engagement, saves, video views over time
4. **Ad management** — create campaigns, manage ad groups/creatives, pull performance data
5. **Catalog management** — upload product feeds, manage catalog items for shopping ads

## Table Stakes (what competing tools have)
- collactivelabs/pinterest-mcp-server: user info, boards list/create/get, pins list/create/get (7 tools)
- CData MCP server: read-only boards/pins
- terryso/mcp-pinterest: Pinterest image search (scraping, no auth)
- brtdwchtr/pinterest-export: public board scraping → JSON/MD/images + Gemini vision analysis
- motebaya/pinterest-js: media downloader
- Official SDK (Python): full API coverage but requires custom coding

## Data Layer
- Primary entities: boards, pins, ad_accounts, campaigns, ad_groups, ads, audiences, catalog_feeds, catalog_items
- Sync cursors: bookmark-based pagination on all list endpoints
- FTS/search: pins.title, pins.description, boards.name, boards.description
- Analytics snapshots: time-windowed metrics (impressions, engagements, saves, pin_clicks)

## Product Thesis
- Name: pinterest-pp-cli
- Why it should exist: Every existing tool is either a thin API wrapper or a scraper with no auth. No tool combines the full official API with a local SQLite data layer, compound analytics queries, offline search, and agent-native output. A power user managing Pinterest for a brand needs to answer "which boards drive the most saves?" in one command — not 10 API calls.

## Build Priorities
1. Full board + pin CRUD with --json and --dry-run
2. Local SQLite sync of boards/pins/analytics for offline compound queries
3. Analytics aggregator: cross-board and cross-pin performance in one query
4. Ad account overview: campaign → ad group → ad hierarchy with spend/performance
5. Search with local FTS + API search combined
6. Catalog feed management for shopping campaigns
