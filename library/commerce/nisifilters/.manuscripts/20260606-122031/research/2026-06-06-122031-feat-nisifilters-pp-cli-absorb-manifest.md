# nisifilters-pp-cli Absorb Manifest

Scope: READ-ONLY public mirror of nisifilters.it (WooCommerce catalog + WordPress content). No auth, no writes.

## Landscape
No dedicated CLI exists for this store. "Competitors" are the storefront UI, the raw
wp/v2 + wc/store endpoints, and generic REST clients. We absorb the full public read
surface and beat it with a local SQLite store, offline search, a filter-finder, and
agent-clean output.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/get products | wc/store/v1/products | (generated endpoint) products list, products get | prices+stock, offline after sync |
| 2 | List product categories | wc/store/v1/products/categories | (generated endpoint) product-categories list | offline lookup |
| 4 | List/get posts | wp/v2 /posts | (generated endpoint) posts list, posts get | --json/--select, offline |
| 5 | List/get pages | wp/v2 /pages | (generated endpoint) pages list, pages get | scriptable |
| 6 | List/get media | wp/v2 /media | (generated endpoint) media list, media get | resolves source_url |
| 7 | List/get categories | wp/v2 /categories | (generated endpoint) categories list, categories get | offline |
| 8 | List/get tags | wp/v2 /tags | (generated endpoint) tags list, tags get | offline |
| 9 | List/get comments | wp/v2 /comments | (generated endpoint) comments list, comments get | filter by post |
| 10 | List/get authors | wp/v2 /users | (generated endpoint) authors list, authors get | public author info |
| 11 | Global site search | wp/v2 /search | (generated endpoint) find list | live cross-type search |
| 12 | Sync everything to local store | (framework) | (behavior in nisifilters-pp-cli sync) | one command mirrors all entities to SQLite |
| 13 | Offline full-text search | (framework) | (behavior in nisifilters-pp-cli search) | FTS5 over products + content, zero network |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Enriched shop view | shop | hand-code | Joins WooCommerce products with categories locally; name, price, stock, sorted by price; offline after sync | none |
| 2 | Filter finder | filters | hand-code | Reads product attributes + each product's attribute values from the local mirror to answer "what sizes/types exist and which products match" — no storefront endpoint returns this cross-product view | Use this to discover available filter sizes/types and find matching products. Do NOT use it for blog posts; use 'read'. |
| 3 | Agent-clean reader | read | hand-code | Reads a post/page from the local store, emits title + plain-text content, dropping yoast/jetpack/_links noise | Use this for a clean view of one Academy post or page. Add --html to keep markup. |
| 4 | Catalog + site digest | digest | hand-code | Cross-entity aggregation over the local store: product count, price range, in-stock vs out, top categories, content counts — no single endpoint provides it | none |
| 5 | Recently changed | since | hand-code | Time-windowed scan of locally-stored modified timestamps across posts/pages at once; the API has no unified recently-changed endpoint (Store API products lack timestamps, so they are excluded) | Use this for 'what is new on the site in the last N days/weeks'. |
| 6 | Featured-image / full-res resolver | image | hand-code | Resolves a post/page featured_media to media.source_url + size variants, or a product's inline images; the source entity only stores an ID | none |

All transcendence rows are hand-code; 0 spec-emits. Hand-code count: 6.

## Removed during build
- product_attributes resource: NiSi's global `/wc/store/v1/products/attributes` endpoint returns `[]` (all attributes are per-product). Dropped from the spec; the `filters` command surfaces attribute discovery from the products table instead.

## Stubs
None. Every row ships fully implemented (read-only, no external dependency).
