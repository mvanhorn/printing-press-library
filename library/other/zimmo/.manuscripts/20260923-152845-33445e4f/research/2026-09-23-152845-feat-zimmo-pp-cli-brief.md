# Zimmo CLI Brief

## API Identity
- Domain: zimmo.be — Belgian real-estate portal (Zimmo Group), ~86k active properties (60k for sale, rest to rent), FR/NL/EN.
- Users: buyers/renters, investors sourcing deals (the investor client, Brussels), agents, market analysts; the tool builder builds sourcing tools for clients (sibling CLIs immoweb-pp-cli, immovlan-pp-cli).
- Data profile: structured JSON listings (exact address + rooftop GPS, rooms with surfaces, EPC letter + kWh/m² value, flooding, planning, construction year, condition, price, documents, photos, dealer with reviews), SOLD/RENTED historical listings, locality €/m² with monthly history.
- No official public API. The Angular SSR site talks to a family of JSON micro-services, all reachable with plain Go HTTP (probe-reachability: standard_http; curl alone gets a Cloudflare challenge because of its TLS fingerprint, Go stdlib does not).

## Discovered surface (verified live 2026-09-23)
- Auth: `POST https://user-api.zimmo.be/token/anonymous` body `{deviceName, deviceId(uuid), locale, channel:"web"}` → `{token, refreshToken}` JWT (ROLE_ANONYMOUS, 24h). No account, no cookie. Sent as `Authorization: Bearer`.
- `POST https://search-api.zimmo.be/listings/search` body `{paging:{from,size}, sorting:[{type:PRICE|DATE|DISTANCE..., order}], filter:{status:{in:[FOR_SALE|TO_RENT|TAKE_OVER|SOLD|RENTED]}, placeId:{in:[id]}, postalCode:{in:[..]}, category:{in:[HOUSE|APARTMENT|COMMERCIAL|GARAGE|PLOT|ROOM|OTHER]}, subType, price|bedrooms|bathrooms|floorspaceSurface|plotSurface|constructionYear:{range:{min,max},unknown:false}, energyLabel:{in:[A..G]}, epcValue, condition, newConstruction:{eq}, text:{query}, zimmoCode:{in:[..]}, polygon}}` → `{total, searchAfter, listings[]}`.
- `GET search-api/listings/{id|zimmoCode}` — full listing (same shape, plus priceHistory when visible).
- `GET search-api/listings/property-count`, `GET search-api/dealers/{id}`, `GET search-api/dealers/{id}/reviews`, `POST search-api/dealers/search`, `POST search-api/listings/recommendations`, `POST search-api/map/search`.
- `GET geo-api/places/by-postal-code-and-or-name?postalCode=&name=` → place ids (levels 4 province…10), `GET geo-api/geocode?address=`, `GET geo-api/sublocalities?keyword=&locale=`, `GET geo-api/places/{id}`, `GET geo-api/reverse-geocode`.
- `GET score-api/locality-price/{placeId}?startDate=` and `sub-locality-price/{placeId}` → €/m² now by type + monthly history. `score-api/property-scores/{zimmoCode}` (market comparison).

## Reachability Risk
- Low. Go stdlib 200 on site and all APIs; Cloudflare challenges only curl's TLS fingerprint. Anonymous JWT minted without captcha. Risk: token endpoint could add recaptcha (site key present for forms, not used for anonymous token today).

## Top Workflows
1. Search a commune/postcode with type, price, bedrooms, EPC filters → table/JSON; all pages into a local store.
2. Show one listing by zimmo code (LAISZ) or URL: full specs, EPC, flooding, rooms, dealer, price history.
3. Market price: €/m² for a commune with monthly history; compare a listing's €/m² to its commune.
4. Watch saved searches daily: new / price cut / gone (sold) — the client's sourcing cron.
5. Sold comps: SOLD/RENTED listings for a commune (the site exposes them), for valuation.

## Table Stakes (Apify zimmo scrapers, immoweb/immovlan siblings)
- Search with filters, pagination, detail fields, photos, agency contact, export CSV/JSON, saved searches/alerts, favourites, price drops.

## Data Layer
- Primary entities: listings (id, zimmo code, status, price, address, GPS, EPC, surfaces), price observations per sync, places, dealers, locality prices.
- Sync cursor: paging.from over a stable DATE sort; `searchAfter` returned too.
- FTS: address + description (fr/nl) + dealer name.

## Product Thesis
- Name: zimmo-pp-cli
- Why: Zimmo is the only Belgian portal whose backend gives exact addresses, rooftop GPS, numeric EPC, flood zone, SOLD history and official-looking €/m² series — as clean JSON. A CLI turns that into deal sourcing, comps and alerts, joinable with the immoweb/immovlan stores.

## Build Priorities
1. Anonymous-token client + search/show/places/price commands (JSON first).
2. Local store + sync + watch (new / drops / gone) + export.
3. Transcendence: sold comps, €/m² vs commune (underpriced), PEB trap, cross-portal same-as with immoweb/immovlan stores.
