# Mercado Livre CLI Brief

## API Identity
- Domain: Brazilian marketplace (mercadolivre.com.br). Buyer-side price & spec lookup.
- Users: procurement/suprimentos automation — search products, compare prices across listings, compare technical specs across candidate products for cotação.
- Data profile: search results (price, seller, rating, url), catalog product pages (full technical attribute table), category attribute schemas.

## Reachability Risk
- **High (mitigated).** Search/product pages captcha-wall plain HTTP; require Chrome cookie import + Surf fingerprint transport (browser_clearance_http). Confirmed live: logged-in Chrome renders fully; JSON-LD extraction is clean. No-auth official endpoints (categories/attributes, domain_discovery, autosuggest) are always-reachable helpers.

## Top Workflows
1. Search a product term -> ranked list with prices, sellers, ratings, product URLs (procurement candidate discovery).
2. Fetch a product page -> full technical spec table + price + rating (single-item deep data).
3. Compare N products side-by-side -> aligned spec matrix + price column (the core cotação artifact).
4. Price snapshot over time -> local store of price history per product/query (drift detection).
5. Cheapest-per-spec: given a query + required attributes, find the lowest-priced listing meeting the spec floor.

## Table Stakes
- Search with pagination and price/rating extraction (every scraper does this).
- Product detail with attributes (linces/MercadoScraper, Unwrangle).
- Query autosuggest / expansion.
- CSV/JSON export for spreadsheet-driven procurement.

## Data Layer
- Primary entities: product (catalog_id, name, brand, price, currency, rating, url, category), attribute (product_id, name, value), price_snapshot (product_id, price, captured_at), search_result (query, product_id, position, captured_at).
- Sync cursor: per-query capture timestamp.
- FTS/search: FTS5 over product name + attributes for offline re-query.

## Product spec / attribute data
- Search JSON-LD @graph -> per-listing price/rating/url.
- Product /p/ JSON-LD Product + Table + DOM spec table -> ~40 structured 'Atributo :: Valor' rows.
- categories/{id}/attributes (no auth) -> attribute schema to normalize comparison columns.

## Product Thesis
- Name: mercadolivre-pp-cli (price & spec comparison for procurement).
- Why it should exist: the niche is empty — no open-source CLI does Mercado Livre price+spec comparison; existing tools are one-off scraper scripts or paid scraper APIs. This CLI gives procurement automation a scriptable, agent-native, offline-capable comparison tool that survives the captcha wall via Chrome clearance, extracts stable JSON-LD, and persists price/spec history locally for cotação workflows.

## Build Priorities
1. Data layer (product, attribute, price_snapshot, search_result) + sync + FTS + SQL.
2. Absorbed: search (paginated, JSON-LD), product detail (JSON-LD + spec table), autosuggest, domain_discovery, category attributes, CSV/JSON export.
3. Transcendence: compare (spec matrix), cheapest-per-spec, price-history/drift, spec-diff, cotação export.
