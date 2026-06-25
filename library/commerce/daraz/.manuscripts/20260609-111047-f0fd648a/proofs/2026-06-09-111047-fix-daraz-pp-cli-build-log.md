Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Daraz CLI — Phase 3 Build Log

## Transcendence (6/6 built, all hand-coded, behaviorally smoke-tested live)
1. Price history & drop tracking — `watch <query>` + `price-history <itemId>` (price_snapshots table). VERIFIED: watch recorded 80 items; price-history returns count/current/lowest/highest.
2. Real-deal ranking — `deals <query>` (discount×rating×sold composite). VERIFIED: power-bank deals ranked sensibly.
3. Saved-search change alerts — `since <query>` (search_snapshots diff: new/drops/increases). VERIFIED: baseline + diff path.
4. Cross-seller compare — `compare <query>` (cheapest + best-rated + per-seller agg). VERIFIED: 120 scanned, 10 sellers.
5. Seller scorecard — `seller stats <sellerId>` (local aggregate). VERIFIED: avgRating/price-range/avgDiscount/topBrands.
6. Value / fake-discount detector — `value <query>` (market-median comparison + suspicious flag). VERIFIED: --only-suspicious flags inflated discounts.

## Absorbed
- `products` (search) — generated typed endpoint (q/page/sort/price). VERIFIED live.
- `reviews` — generated typed endpoint (item-id/page-size/filter/sort). VERIFIED live.
- `products get <itemId>` — hand-coded PDP detail via schema.org JSON-LD (name/brand/category/sku/description/availability); price enriched from local store when previously seen. VERIFIED.
- `seller products <sellerId>` — hand-coded local-store listing by seller. VERIFIED (7 listings).

## Design notes / intentional deviations
- All novel commands ride on the public catalog search JSON (`standard_http`, no auth) and a self-populating local SQLite store (`daraz_products_seen`, `daraz_price_snapshots`, `daraz_search_snapshots` in internal/cli/daraz_common.go). Every deals/value/compare/watch/since run compounds the store, so price-history/seller-stats/seller-products work from normal use.
- PDP price is NOT in the page's JSON-LD and not reliably in the __moduleData__ island, so `products get` sources price from the local store (with `priceAsOf`) rather than the PDP. Honest + documented.
- Daraz returns `totalResults` (and some IDs) as either JSON strings or numbers; the parser stringifies all list-item fields via productFromMap + flexInt to be type-robust (caught by live test).
- Server-side filters limited to verified-working params (q/page/sort/price). rating/location/brand are filtered client-side where needed (the site's rating=/location= query params did not actually filter). `seller products` (live-by-seller) has no public endpoint, so it is served from the local store instead — capability preserved, sourced honestly.

## Generator limitations found (retro candidates)
- Generated store logs "no extractable ID field" warnings for products/reviews typed endpoints (the catalog/reviews payloads nest IDs); offline caching of the raw endpoints is incomplete. Live queries unaffected; novel commands use their own tables.

## Tests
- internal/cli/daraz_common_test.go: parseMoney, leadingInt, discountPct, dealScore (monotonic), medianFloat, extractProductDetail, plus dry-run wiring for all 9 hand-built commands. `go test ./internal/cli/` PASS.
