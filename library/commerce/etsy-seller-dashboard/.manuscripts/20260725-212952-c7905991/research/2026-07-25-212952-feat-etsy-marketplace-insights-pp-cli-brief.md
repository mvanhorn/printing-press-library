# Etsy Marketplace Insights CLI Brief

## API Identity
- Domain: Authenticated Etsy Shop Manager keyword and demand intelligence.
- Users: Etsy sellers choosing products, titles, tags, inventory timing, and pricing.
- Data profile: Search counts, listing competition, related terms, category trends, time-series demand, search quotas, and feature-gated conversion/price metrics.
- Runtime target: Replayable HTTP/HTML discovered from `https://www.etsy.com/your/shops/me/marketplace-insights`; no resident browser transport.

## Reachability Risk
- High: anonymous direct HTTP returned `403`; the feature lives behind an Etsy seller session and is available only on desktop/mobile web.
- Authenticated browser discovery succeeded and captured five useful JSON endpoints plus two structured HTML routes. Direct HTTP with imported cookies remains to be proven during dogfood; keyword enqueue also requires an ephemeral `x-csrf-token`.
- Product constraint: Etsy allows 15 keyword searches per week for standard sellers; Etsy Plus has unlimited searches. The CLI must surface remaining quota and avoid accidental duplicate searches.

## Users
- **POD catalog operator:** checks several niche phrases before each weekly design batch, compares demand with listing competition, and needs to avoid spending the 15-search allowance twice on the same term.
- **Seasonal physical-goods planner:** reviews category trends before buying materials or scheduling production, then revisits the same terms week over week to decide whether demand is durable.
- **Multi-shop SEO operator:** researches and exports keyword sets for multiple listings, keeps notes and shortlists, and needs reproducible JSON/CSV rather than copying values between browser tabs.

## Top Workflows
1. Research a keyword and return searches, competing listings, trend direction, related terms, and opportunity signals.
2. Compare a batch of candidate keywords without wasting quota on duplicates.
3. Find high-demand, lower-competition related terms suitable for Etsy titles and tags.
4. Save keyword snapshots locally and detect week-over-week or date-range changes.
5. Plan inventory and listing refresh timing from category and keyword trends.

## Table Stakes
- Keyword lookup with search and listing counts.
- Related-keyword discovery, sorting, filtering, and export.
- Trend history and rising/falling detection.
- Saved searches, notes, lists, and repeatable comparisons.
- Opportunity ranking based on demand versus competition.
- Structured JSON/CSV output, field selection, and local SQL/FTS search.
- Explicit quota reporting and cache reuse.

## Data Layer
- Primary entities: keyword searches, keyword metrics, trend points, related keywords, categories, saved searches, and quota snapshots.
- Sync cursor: observation timestamp plus the feature’s selected date range.
- FTS/search: keyword text, related terms, list names, and notes.
- Retention: preserve historical snapshots locally even when Etsy’s free-plan results expire after seven days.

## Codebase Intelligence
- Browser capture: Marketplace Insights uses authenticated Shop Manager HTML plus private `/api/v3/ajax/.../marketplace-insights` JSON routes; the mutation observed in capture uses `x-csrf-token`.
- Community MCP source: Etsy API wrappers conventionally use `x-api-key` for public API reads and `Authorization: Bearer <token>` plus `x-api-key` for OAuth shop operations.
- Scope boundary: the public Etsy API MCP projects cover listings, shops, reviews, receipts, sections, and taxonomy, but not the private Marketplace Insights search-volume surface.
- DeepWiki: the discovered MCP repository page was unindexed/thin, so no architectural claims were taken from it.

## Competitive Landscape
- eRank: keyword metrics, 15-month trends, top-listing/tag analysis, keyword lists, notes, exports, rank checking, competitor research, and traffic stats.
- Alura: demand/competition metrics, historical trends, related terms, advanced filters, sorting, copy/save/export, and shop/listing analysis.
- EverBee: on-page Etsy search volume, competitor tag discovery, keyword variations, and high-demand/low-competition research.
- Marmalead: keyword engagement, competition quality, and longer-horizon trend context.
- Seerxo: CLI/MCP/Claude skill for listing generation, SEO audit, optimization, and autosuggest keyword mining.
- Community Etsy MCP servers: public listing/shop search plus authenticated listing, receipt, section, taxonomy, and inventory operations; they do not expose Marketplace Insights.

## Product Thesis
- Name: Etsy Marketplace Insights CLI
- Thesis: Turn Etsy’s first-party seller search data into a quota-aware, local historical research system that can compare, rank, export, and monitor keyword opportunities without losing prior snapshots.

## Build Priorities
1. Capture and prove the authenticated Marketplace Insights request contract is replayable outside page context.
2. Implement quota-aware keyword lookup, related terms, trends, and category discovery.
3. Persist snapshots and saved searches in SQLite with FTS, SQL, JSON, CSV, and field selection.
4. Add batch comparison, opportunity scoring, change detection, and inventory-planning views.
5. Dogfood against the user’s Etsy session without mutating listings or shop state.

## Sources
- Etsy Help: “How Do I Use Etsy’s Marketplace Insights Tool?”
- Etsy Help: “Newly Crafted: Etsy Updates for Your Shop”
- eRank feature catalog and Marketplace Insights comparison
- Alura Keyword Finder
- EverBee Keyword Research
- Marmalead Marketplace Insights comparison
- Seerxo CLI/MCP repository
- `administrativetrick/etsy-mcp-server` and `profplum700/etsy-mcp-server` source
