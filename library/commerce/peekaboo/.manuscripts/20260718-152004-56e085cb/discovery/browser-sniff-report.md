# Peekaboo Guru — Browser-Sniff Discovery Report

## Target
- Site: https://peekaboo.guru (Fetch Sky). React SPA, Cloudflare-fronted.
- Primary flow captured: browse food deals in a city -> open a merchant -> view its branches (with map coordinates) -> view its deals/card-offers.
- Capture method: agent-browser network log + HAR + direct HTTP replay with the guest bearer token.

## API base
- `https://peekaboo.guru/api` — same origin as the site. Endpoints are POST/GET with **per-endpoint version prefixes** (v5..v8).
- `:444` is the iframe-embed origin (`IFRAME_ORIGIN_SOURCES`), NOT the API; unreachable from data-center egress. The real API is same-origin `/api/...`.

## Auth (verified)
- Public guest **JWT bearer token** embedded in every page's SSR HTML: `window.__guest__.token` (role=guest, HS256, no expiry, issued 2019 — static & permanent).
- `POST /api/v5/locations` works with NO token. All other endpoints return `401` without it.
- Model: `bearer_token`. Zero-config bootstrap = fetch any page, scrape `window.__guest__.token`, cache, send as `Authorization: Bearer <token>`. Token value is NEVER stored in artifacts (scraped at runtime).

## Endpoints (all verified live, 200)
| # | Method | Path | Required params | Returns |
|---|--------|------|-----------------|---------|
| 1 | POST | /api/v5/locations | limit, offset | {total, locations[]} cities w/ lat,long,timezone,entityCount |
| 2 | POST | /api/v7/category | language, country, city, lat, long | [categories] id,name,scope,logo,discount,dealCount |
| 3 | POST | /api/v8/entities | country, city, category, limit, offset, latitude, longitude | {total,nextPage,entities[]} merchants w/ nearestBranch{lat,long},stats.branches |
| 4 | POST | /api/v6/sourceEntities | country, city, limit, offset, latitude, longitude | [brands] sourceEntityId,name,categories,logo |
| 5 | POST | /api/v8/entity/detail | entityId, city, country, lat, long, language | merchant detail: social,menu,gallery,tags,rating,stats |
| 6 | GET  | /api/v5/entity/{id}/branch/_all | city,country,entity,language,lat,long,limit,offset (query) | {branches[]} id,name,address,**latitude,longitude**,timings ← headline |
| 7 | GET  | /api/v5/entity/{id}/branch/_all/sourceEntities/_all | (same query) | [card sources] banks giving card deals at this merchant |
| 8 | POST | /api/v8/entity/deals | city,country,lat,long,language,offset,limit,associatedDeals,targetEntityId,card | {total,deals[]} title,percentageValue,dates,description |
| 9 | POST | /api/v6/entity/amenities | limit, offset, country, entityId | [amenities] amenityId,amenityName |
| 10 | POST | /api/v6/entity/widget | city,country,lat,long,language,platform,screen,limit,offset[,entityId,name] | {counts,data[]} UI widget layout |

## URL model (frontend)
- Category listing: `/{city}/places/{categoryId}/{categorySlug}` (e.g. /lahore/places/1/food)
- Merchant detail:  `/{city}/detail/{entityId}/{slug}` (e.g. /lahore/detail/13/kababjees)
- Branches page:    `/{city}/detail/{entityId}/{slug}/branches`  ← "Direction" CTA per branch = maps.google.com/maps?daddr={lat},{long}

## Categories (13): Food(1), Lifestyle(18), Health(27), Entertainment(10), E-Stores(55),
## Education(22), Home-Decor(19), Services(29), Electronics(16), Self-Care(28),
## Public-Services(39), Hotels(133), Grocery(42)

## Replayability
- All endpoints replay via plain HTTPS + guest bearer + Chrome User-Agent. No browser runtime needed in the CLI.
- Cloudflare present but does not challenge /api/* for a Chrome UA. Rate-limits on bursts (~30 rapid req) — CLI should pace.
