Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# zimmo-pp-cli build log (2026-09-23)

## Foundation
- internal/zimmo: anonymous JWT (user-api token/anonymous, channel web, cached 0600 in the CLI cache dir, re-minted on 401), typed client over search-api / geo-api / score-api with AdaptiveLimiter (2 req/s) + RateLimitError, Criteria → Zimmo filter (status/category/placeId/postalCode/ranges with unknown:false/energyLabel expanded to +/- variants/text/zimmoCode/polygon [lat,lon]/newConstruction), zimmo.be URL parser (SEO paths + base64 search=), listing flattener (exact address, rooftop GPS, EPC letter normalised + kWh + suspect flag, renovation duty, rentPerYear → rented, flood/planning YES flags, subpoena, price history + total cut, days on market, sale-form flags viager/bare_ownership/usufruct/public_sale/shared_ownership).
- internal/cli/zimmo_auth_hook.go: client hook gives generated endpoint commands the anonymous token (ZIMMO_TOKEN overrides). Spec declares no security scheme so doctor reports "Auth: not required".
- internal/store/zimmo_store.go: zm_listings (full flattened JSON + indexed columns), zm_price_obs, zm_saved, zm_search_seen, zm_shortlist, zm_places (30d cache), zm_locality_price (7d cache); FTS via framework resources table.

## Absorbed (15)
find, show, photos, dump (json/csv/geojson), agency, places (generated promoted), prices, watch save/run/list/rm, shortlist, drops, find --status rent/sold/rented (+ --deal alias), geocode + listings property-count (generated).

## Transcendence (7, all hand-code)
enrich, same-as (reads immoweb/immovlan stores read-only), comps (rooftop-only radius, auto-widen to postcode), peb-trap, underpriced (skips viager etc.), yield (current lease or similar-surface rental comps), motivated (strict re-listing: same unit, ≥14 days apart).

## Deferred / removed
- dealers/{id}/reviews: 404 on every Zimmo host → removed from spec; agency shows the combined review score from the dealer record.
- Framework `sync` only covers places (listings are POST search); local commands print their own "run find first" hint instead of the generic sync hint.

## Bugs found by tests and live runs, fixed
- Seen-set never cleared on an empty result (NOT IN (NULL)) → gone listings re-reported every watch run.
- EPC letter filter missed D+/D- variants (61 vs 69 in 1050).
- peb-trap ranked input-error kWh (27 516) first → suspect flag, letter-first sort.
- motivated read units of one building as re-listings → same unit + 14-day gap.
- comps counted commune-centroid coordinates as 679 m neighbours → rooftop only.
- yield extrapolated studio rents to a 560 m² house (19%) → similar-surface comparables.
- enrich re-fetched every stored row → search results count as fresh.

## Generator notes
- OpenAPI response-path inference took the only array field (priceHistory) as the list envelope for GET listings/{id}; removed it from the schema.
- OpenAPI has no health_check_path extension; doctor probes "/" (404, reported reachable).
