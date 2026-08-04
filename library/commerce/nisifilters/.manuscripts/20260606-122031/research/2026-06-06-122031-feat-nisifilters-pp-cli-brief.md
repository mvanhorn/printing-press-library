# nisifilters.it CLI Brief

## API Identity
- Domain: Official Italian NiSi store (nisifilters.it) — photographic filters: ND, GND, polarizers, UV, square/round, holders, kits, plus an "Academy" content area. WordPress + WooCommerce.
- Surface: self-documenting WordPress REST API (`/wp-json/wp/v2`) + WooCommerce Store API (`/wp-json/wc/store/v1`). No `avada_portfolio`; comments enabled; product attributes present (Dimensione/size, kit). `wc/store/v1/products/attributes` exposes global attribute terms.
- Users: the site owner / a photographer who wants a scriptable, offline, agent-clean view of the NiSi catalog — search filters by size/type, compare prices, check stock — without clicking through the storefront.
- Data profile: verbose WordPress/WooCommerce JSON. Products carry prices (minor-unit strings), images, categories, attributes, stock; posts/pages carry rendered HTML + Yoast noise.

## Scope (user-selected)
- EVERYTHING, READ-ONLY. Full public mirror: products + product categories + product attributes + posts + pages + media + categories + tags + comments + authors + global search. No auth, no writes.

## Reachability Risk
- None. Every targeted endpoint returns HTTP 200 with no credentials (wc/store products + categories + attributes, wp/v2 posts/pages/media/categories/tags/comments/users/search all 200).
- Probe-safe endpoint used: GET /wp-json/wc/store/v1/products?per_page=1

## Top Workflows
1. "Mirror the catalog and search it offline" — sync products + content into SQLite, full-text search filters by name/description.
2. "Find the right filter" — discover available sizes/types via product attributes, then list matching products.
3. "Scan the shop by price/category/stock" — priced, categorized, in-stock view sorted by price.
4. "Resolve a product's images / read a guide" — full-res product images; clean HTML-stripped reader for Academy posts/pages.
5. "Catalog overview" — counts, price range, categories, stock levels from the local mirror.

## Table Stakes
- list/get for products, product categories, posts, pages, media, categories, tags, comments, authors; global search; WooCommerce prices/stock.

## Data Layer
- Primary entities: product, product_category, product_attribute, post, page, media, category, tag, comment, author.
- Sync cursor: page-based (`?page=N&per_page=M`); iterate until empty.
- FTS/search: name + description + short_description + post content → SQLite FTS5 offline search.

## Product Thesis
- Name: nisifilters-pp-cli (display: NiSi Italia)
- Thesis: A read-only, offline-first mirror and search engine for the public NiSi Italia catalog + content — every filter, price, and guide queryable from the terminal with a filter-finder no storefront offers, agent-clean output, and zero auth.

## Build Priorities
1. Data layer + sync for all public entities (products central).
2. Absorbed: list/get per resource + WooCommerce catalog + cross-entity search.
3. Transcendence: shop (priced/categorized), filters (attribute finder), read (clean reader), digest (catalog summary), since (recent content), image (image resolver).
