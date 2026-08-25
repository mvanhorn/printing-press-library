# Etsy Seller Dashboard CLI Brief

## API Identity
- Domain: Authenticated Etsy Shop Manager intelligence and marketing operations.
- Equal-weight sources: Marketplace Insights, Etsy Ads, Offsite Ads, and Sales & Discounts.
- README lead: Marketplace Insights, because one surface must anchor first-run narrative.
- Data profile: Search demand, listing competition, related terms, category trends, ad delivery/performance, offsite acquisition, promotion rules, sends, redemptions, revenue, and fees.
- Runtime target: Replayable HTTP/HTML discovered from authenticated Etsy Shop Manager pages; no resident browser transport.

## Reachability Risk
- High: anonymous direct HTTP receives Etsy DataDome protection; all four surfaces require an Etsy seller session.
- Authenticated browser discovery succeeded across the four pages.
- Runtime must import local Chrome cookies, preserve them locally with restrictive permissions, and refresh ephemeral CSRF material only for mutations.
- Shipping priority is read-only analytics. Ad toggles, opt-out controls, and promotion mutations are excluded until direct replay and idempotent safeguards are proven.
- Marketplace Insights adds a product quota: 15 keyword searches per week for standard sellers, unlimited for Etsy Plus, with free results retained for seven days.

## Users
- **POD catalog operator:** checks niche phrases before a weekly design batch, monitors which listings deserve ad spend, and uses discounts to move weak inventory without wasting the keyword allowance.
- **Seasonal physical-goods planner:** compares search/category momentum with ad and promotion performance before buying materials or scheduling production.
- **Multi-shop growth operator:** exports keyword, ads, offsite acquisition, and promotion data into recurring reports and needs consistent JSON/CSV instead of four browser tabs.
- **Profit-conscious seller:** needs to reconcile Etsy Ads spend, Offsite Ads fees, discount depth, and attributed revenue before increasing a campaign or promotion.

## Top Workflows
1. Research a keyword and return searches, competing listings, trend direction, related terms, and opportunity signals.
2. Compare candidate keywords without wasting quota on duplicate or still-cached searches.
3. Review Etsy Ads by listing, date range, status, and metrics: views, clicks, click rate, orders, revenue, spend, and ROAS.
4. Review Offsite Ads traffic, channel share, listing performance, direct/indirect orders and revenue, new buyers, and fees.
5. Inventory active and historical sales/coupons with dates, discount rules, sends, uses, conversion, and revenue.
6. Join search demand, listing metadata, ad performance, offsite acquisition, and promotions to decide what to make, advertise, discount, or stop.
7. Preserve weekly/monthly snapshots locally and detect changes across all four surfaces.

## Table Stakes
- Marketplace Insights keyword lookup, related terms, trends, categories, saved searches, quota, sorting, filtering, and export.
- Etsy Ads campaign totals, listing-level metrics, date comparison, pagination, filters, sorting, and CSV-equivalent output.
- Offsite Ads traffic history, comparison periods, channel performance, listing performance, fees, orders, revenue, and new buyers.
- Sales & Discounts promotion inventory, rules, active/paused state, targeted-offer sends, uses/redemptions, conversion, and revenue.
- Structured JSON/CSV, field selection, typed exit codes, local SQLite sync, FTS, and SQL access.
- Read-only default; mutation commands require explicit later approval, dry-run, and idempotent safeguards.

## Data Layer
- Marketplace entities: keyword searches, keyword metrics, trend points, related keywords, categories, saved searches, and quota snapshots.
- Etsy Ads entities: listing ad status, listing metadata, impression/click/order/conversion metrics, spend, revenue, click rate, and ROAS snapshots.
- Offsite Ads entities: traffic points, comparison points, channels, listing performance, direct/indirect attribution, new buyers, fees, orders, and revenue.
- Promotion entities: promotion/coupon IDs, types, names, dates, status, conditions, discount rules, targeted-offer metadata, send counts, uses, and revenue.
- Shared keys: shop, listing, observation date/range, and normalized keyword/list membership.
- Retention: preserve historical snapshots locally beyond Etsy page windows and Marketplace Insights free-result expiration.

## Codebase Intelligence
- Marketplace Insights: five useful JSON endpoints plus two authenticated HTML routes; keyword enqueue carries `x-csrf-token`.
- Etsy Ads: listing stats endpoint returns listing metadata and impressions, clicks, spend, conversions, revenue, click rate, and ROAS.
- Offsite Ads: separate JSON resources cover traffic history, channel mix, listing performance, and direct/indirect revenue, fees, orders, and new buyers.
- Sales & Discounts: combined promotions resource exposes definitions, dates, discount rules, targeted-offer state/send counts, and revenue stats.
- Community Etsy MCP source uses `x-api-key` for public reads and `Authorization: Bearer <token>` plus `x-api-key` for public OAuth shop operations, but those projects do not expose these private dashboard routes.
- DeepWiki was unindexed/thin for the discovered MCP repository, so no architectural claims were taken from it.

## Source Priority
- Confirmed by user: all four dashboard surfaces are peers.
- README/first-run lead: Marketplace Insights only because the presentation needs one starting point.
- Scope rule: no source may be dropped or receive fewer quality guarantees merely because another surface has cleaner captured JSON.

## Competitive Landscape
- eRank: keyword metrics, long-range trends, top-listing/tag analysis, keyword lists, exports, rank checking, listing audits, competitor research, and traffic stats.
- Alura: demand/competition, historical trends, related terms, filters, sort/save/export, and listing analysis.
- EverBee: on-page volume, competitor tags, variants, and opportunity research.
- Marmalead: keyword engagement, competition quality, and longer-horizon trend context.
- Seerxo: CLI/MCP/skill for listing generation, SEO audit, optimization, and autosuggest keyword mining.
- Etsy's dashboard remains the first-party source for actual seller search, ad, offsite attribution, and promotion data.

## Product Thesis
- Name: Etsy Seller Dashboard CLI
- Thesis: Turn four disconnected Etsy seller pages into a quota-aware, historical growth system that connects demand, paid acquisition, promotion performance, and listing economics without inventing data or requiring a resident browser.

## Build Priorities
1. Prove cookie-import replay for read endpoints and HTML parsing across all four surfaces.
2. Ship complete read commands for keywords, Etsy Ads, Offsite Ads, and Sales & Discounts.
3. Persist normalized snapshots in SQLite with JSON/CSV/select/SQL access.
4. Add cross-surface decisions: quota planning, lifecycle, ad efficiency, discount economics, attribution, and listing action queues.
5. Dogfood on the user's Etsy session without changing ad status, offsite opt-in, promotions, listings, or shop state.

## Sources
- Etsy Help and Seller Handbook for Marketplace Insights, Etsy Ads, Offsite Ads, and sales/discounts.
- Authenticated browser captures of all four user-provided Shop Manager URLs.
- eRank feature catalog and Marketplace Insights comparison.
- Alura Keyword Finder, EverBee Keyword Research, Marmalead comparison, and Seerxo.
- `administrativetrick/etsy-mcp-server` and `profplum700/etsy-mcp-server` source.
