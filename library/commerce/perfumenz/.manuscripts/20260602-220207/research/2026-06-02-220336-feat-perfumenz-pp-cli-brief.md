# Perfumenz CLI Brief

## API Identity
- Domain: https://www.perfumenz.co.nz/ — New Zealand's largest wholesale distributor of authentic designer and niche perfumes/fragrances. 100% NZ owned and operated. Sells only genuine, boxed/sealed products (heavy emphasis on anti-tester/knockoff).
- Users: Fragrance enthusiasts in NZ/AU, people looking for authentic designer at competitive prices, wholesale-style buyers, anyone who wants note-level discovery without the markup or fakes common in grey market.
- Data profile: Catalog of ~250+ perfumes. Each has: title, vendor (brand), product_type (Mens/Unisex/etc), tags (price band like "50-100", "designer"), variants (price, sku, available), rich body_html with explicit "Fragrance Notes: Top Notes: ..., Heart Notes: ..., Base Notes: ...", images, handle.

## Reachability Risk
- None. Public Shopify JSON endpoints (`/products.json`, `/collections/all.json`) return full structured catalog with 200s. No auth required for browsing the shop or the JSON feeds. Strong replayable HTTP surface.

## Top Workflows
1. Discover perfumes by specific notes or note combinations (e.g. "vanilla + oud but not patchouli", "fresh citrus for summer under $100").
2. Browse by brand + gender + price band, see real stock/availability.
3. Compare two or more fragrances side-by-side on notes, price per ml, similarity.
4. Find "dupes" or similar profiles from the catalog (local overlap scoring beats website faceted search).
5. Sync the full authentic NZ catalog locally for offline use, fast search, and agent scripting (e.g. "build me a 5-perfume discovery set under $400 with these notes").

## Table Stakes
- Full product list + get by handle/slug + search (name, brand, notes, description).
- Filter by brand, gender/category, price range, in-stock.
- Sync the catalog into local SQLite (idempotent, with price history potential).
- --json, --select, --csv, --dry-run, typed errors everywhere.
- Agent-friendly structured output and composable flags.

## Data Layer
- Primary entity: perfume (shopify_id, handle, title, vendor/brand, product_type, tags, price (from first variant), available, body_html, top_notes, heart_notes, base_notes [parsed], image_url).
- Secondary: brand (from vendors).
- Sync cursor: updated_at or simple full replace (catalog is small, ~250 items).
- FTS/search: on title + vendor + parsed notes + description. SQL for price, stock, note overlap queries.
- Store tables: resources (generic + typed perfume rows), plus note indexes for fast "contains these notes" queries.

## Product Thesis
- Name: perfumenz-pp-cli (or perfume-nz)
- Why it should exist: The website is a great authentic source for NZ, but the web UI has weak note discovery. A local CLI + SQLite turns the entire catalog into a queryable, offline, scriptable, agent-native database of real fragrances with explicit note pyramids. Power users and agents get "show me every unisex with grapefruit top + cedar base under $80 that is in stock" in one command, plus similarity, exports, and future price tracking — none of which the Shopify frontend provides at this depth.

## Build Priorities
1. Core sync from the public /products.json (and /collections if needed) into a typed local store with note parsing from body_html.
2. list / get / search commands (with note filters, brand, price, gender, --limit, --json --select).
3. Strong FTS + SQL-backed search that understands Top/Heart/Base.
4. Novel transcendence: note-profile overlap / similarity ("similar to X"), price-per-ml, "note gaps in my collection", brand stats, "what's new since last sync", agent-optimized --agent output.
5. Polish: doctor (verifies the public JSON is reachable), nice examples in --help, full --dry-run on mutating paths (none here, but sync can be --dry-run).
