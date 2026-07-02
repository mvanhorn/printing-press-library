# Browser-Sniff Report — mercadolivre.com.br

Discovery via chrome-MCP against the user's logged-in Chrome (pre-approved, Phase 0 "the website itself"). 2026-07-02.

## Primary user goal
Consulta de preços + comparação de características técnicas entre produtos, para automação de cotação/suprimentos.

## Reachability
- Homepage: plain HTTP 200.
- Search (`lista.mercadolivre.com.br/<query>`) and product (`/p/MLB...`) pages: plain HTTP 302 -> `/captcha/wall`. Logged-in Chrome renders fully.
- Runtime mode: **browser_clearance_http** — needs Chrome cookie import + Chrome-fingerprint HTTP (Surf). No resident browser at runtime.

## Replayable surfaces (confirmed live)
1. **Search page** — JSON-LD `@graph`: ~48 Product entries (name, brand, price BRL numeric, availability, product URL, aggregateRating) + SearchResultsPage. Pagination `_Desde_{offset}` (49/page).
2. **Product page /p/{catalog_id}** — JSON-LD Product (price, sku, rating, reviewCount, shipping, return policy) + JSON-LD Table + DOM spec table (~40 'Atributo :: Valor' rows) + BreadcrumbList category path.
3. **api.mercadolibre.com/categories/{id}/attributes** (no auth) — attribute schema per category for comparison columns.
4. **api.mercadolibre.com/sites/MLB/domain_discovery/search?q=** (no auth) — free-text -> category/domain.
5. **http2.mlstatic.com autosuggest** (no auth) — query expansion.

## Official API status
Gated behind OAuth (403 PolicyAgent) for search/items/products/currencies/sites. No client_credentials flow. Website JSON-LD is the chosen primary; the 3 open no-auth endpoints are helpers.

## Extraction strategy
Parse `application/ld+json` (standard, stable) as primary; DOM poly-cards / spec-table as fallback. Prices are numeric in JSON-LD (offers.price), avoiding fraction/cents span parsing.
