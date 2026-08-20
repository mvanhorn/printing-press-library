# eRank CLI Brief

## API Identity
- Domain: Etsy seller SEO, keyword research, competitor analysis, listing optimization, and shop insights.
- Users: Etsy sellers, POD sellers, marketplace researchers, and agents comparing niches/listings at scale.
- Data profile: keyword metrics, top Etsy listings, tags, prices, processing times, category distributions, trend windows, shop/listing stats, and connected-shop health data.

## Spec Source
- Official eRank API docs: none found in public search.
- OpenAPI/HAR: none provided.
- Built-in Printing Press catalog: no eRank entry.
- Local/public Printing Press library: no eRank CLI found.
- Result: no generation-ready official API spec exists yet. The original `members.erank.com/keyword-tool/top-listings` URL is an authenticated website surface, so the next viable path is browser-sniffing or a user-provided HAR/spec.

## Reachability Risk
- High for the requested "official API" target: public docs describe member tools, not a developer API.
- Browser-session auth likely: eRank account login is required for member tools, and shop connection routes through Etsy authorization.
- Evidence:
  - eRank Help describes Keyword Tool, Rank Checker, Competitor Listings, Top Sellers, Shop Info, Listing Audits, and Traffic Stats as member/web tools.
  - eRank FAQ documents connecting an Etsy shop by logging into eRank, going to Settings, and granting Etsy access.
  - eRank marketing pages emphasize UI workflows: keyword lookup, top 100 listings, keyword lists, competitor shops, and trends.

## Top Workflows
1. Analyze `dad mug` or another keyword across Etsy and capture top listings, tags, price ranges, categories, and listing signals.
2. Compare keyword opportunities by competition, average searches/clicks, CTR, and trend direction.
3. Audit top competitor shops/listings for tags, thumbnails, estimated performance, pricing, and changes over time.
4. Track saved keyword lists and trend windows so a seller can revisit seasonal opportunities.
5. Cross-check connected-shop listings for missing tags/photos, spelling issues, rank, and traffic sources.

## Table Stakes
- Keyword lookup by marketplace/country.
- Top 100 listing analysis for a keyword.
- Popular tags and tag frequency from top listings.
- Keyword comparison and rank checking.
- Competitor shop/listing views.
- Listing audit and health checks.
- Trend views by day/week/month/category/country.
- Exportable JSON/CSV and agent-friendly filters.

## Data Layer
- Primary entities: keywords, keyword_metrics, listings, shops, tags, trends, keyword_lists, rank_checks, listing_audits.
- Sync cursor: keyword + marketplace + country + captured_at; shop/listing IDs for connected-shop surfaces.
- FTS/search: keyword text, listing title, shop name, tags, category, country.

## Product Thesis
- Name: eRank CLI.
- Why it should exist: eRank is UI-first. A CLI would make repeated Etsy research reproducible: snapshot a keyword, diff top listings over time, export compact agent-ready data, and build local trend history that the web UI does not make easy to query.

## Build Priorities
1. Resolve a spec via approved browser-sniffing of the eRank member workflow or stop for a user-provided HAR/spec.
2. If sniffing succeeds, generate replayable read-only commands for keyword top listings, keyword metrics, trends, and competitor views.
3. Add local snapshots/diffs for keyword and listing movement over time.
4. Keep the CLI read-only unless eRank exposes safe list/save endpoints with clear replayability.

## Sources
- eRank Help Features: https://help.erank.com/features/
- eRank Keyword Research Trial: https://erank.com/keyword-research-trial
- eRank FAQ: https://help.erank.com/faq/
- eRank sales/keyword workflow article: https://help.erank.com/blog/how-to-get-more-sales-on-etsy-use-erank/
