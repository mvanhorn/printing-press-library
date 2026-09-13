Manifest transcendence rows: 5 planned, 5 built. Phase 3 will not pass until all 5 ship.

# immoweb-pp-cli build log

## Built
- P0 foundation: `internal/immo` (criteria normalisation EN/FR/NL, Immoweb search-URL parser, listing flattener for search/map/detail JSON, stats, triage scoring, EPC ranks, haversine, €/m² plausibility filter) with table tests; `internal/store/immoweb_store.go` (immo_listings, immo_price_obs, immo_saved, immo_search_seen, immo_hidden, immo_shortlist, immo_pulls; FTS indexing through framework `resources`) with store tests; shared CLI helpers (`immoweb_common.go`: crit flags, commune resolution via autocomplete, soft-throttle detection as typed rate-limit error, non-JSON detection, count, detail, harvest strategy).
- Harvest strategy (discovered live): the map endpoint returns a fixed top-200 and ignores paging, so harvest = exact count → one map call when ≤200 → per-postal-code split for multi-postcode communes → search-results paging (30/page) otherwise. Ixelles rentals: 1,361/1,361 stored in 49 requests.
- P1 absorbed (all 24 rows): find (friendly filters, --url, --pages/--limit, --private-only, --hide-under-option, --near/--radius-km, stores results), show (readable card + price history), photos (--list / download, harness-safe), saved add/list/remove, watch (baseline, new / price changes with old price / gone, --all), hide/--undo, shortlist add/list/remove, dump (csv/geojson/jsonl), pull (area harvest + gone marking), generated endpoints listings search/count/map/get/similar + locations, framework search over synced listings.
- P2 transcendence (5/5): market (live count + auto-pull + medians, p25/p75, days listed over known publication dates, under-option / price-cut shares, gone 30d, --by bedrooms|epc|type|postcode), deal (detail + comparables ±1 bed ±25% surface incl. gone, percentile, fair price at median, days listed, cut history, saves/day, EPC gap), triage (saved search × hidden × postcode €/m² percentile × cut × private × EPC; --enrich N), drops (local history + Immoweb old-price field), yield (listing or commune mode, rent comparables, refuses n<5).

## Fixes found during build (live data)
- Private sellers: Immoweb sets customerName "PRIVATE" (a missing logo also covers companies) — detection switched to the literal.
- Detail `isOwner` means the agency owns the listing, not a private seller — removed.
- Days listed: search pages carry no publication date; first-seen fallback removed (it made every first-pull listing look new).
- €/m² plausibility bounds (rent 3–150 €/m²/month, sale 300–25,000 €/m², surface ≥10 m²) — data-entry errors were topping triage.
- Old price: search results expose transaction.sale.oldPrice / price.oldValue — captured so drops/deal work without local history.

## Deferred / known behaviours
- Immoweb's commune definition ("(all localities)") can include a neighbour postal code (Ixelles → 1000 + 1050); documented, `--by postcode` and `--postcode` let users narrow.
- Immoweb account favourites / saved searches not integrated (auth-only, thin surface; user chose public-first).

## Generator notes (retro candidates)
- Dead generated helpers: handleBinaryResponseDelivery, readSecretFromStdin, successfulNoop.
- Single-endpoint resource promotion turns `locations lookup` into `locations`; the spec example kept the sub-command form.
