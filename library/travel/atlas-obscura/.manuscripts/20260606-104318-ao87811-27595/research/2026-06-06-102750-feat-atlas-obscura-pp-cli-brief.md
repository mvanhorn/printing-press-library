# Atlas Obscura CLI Brief

## API Identity
- Domain: Atlas Obscura (atlasobscura.com) — community-curated database of the world's
  hidden wonders, oddities, and offbeat places. **No official API.** Discovered surfaces are
  community/undocumented and may change. All output must be labeled community-sourced.
- Users: travelers planning offbeat itineraries, road-trippers, writers/worldbuilders seeking
  "spice," curiosity browsers, AI agents enriching trip plans.
- Data profile: ~25k+ places, each with title, subtitle, coordinates, location string,
  description, category tags, "Know Before You Go" practical notes, image.

## Reachability Risk
- None/Low. `probe-reachability` → `standard_http`. Homepage 200 (Cloudflare front, no challenge).
  Search JSON + place pages all 200 over plain HTTP with a normal browser UA.
- No tier/permission gating. No auth.

## Discovered Request Contract (undocumented; via direct probing)
- **Search (text):** `GET /search?q=<query>&page=<n>` with headers
  `Accept: application/json` + `X-Requested-With: XMLHttpRequest` → JSON:
  `{ q, total:{value,relation}, per_page(15), current_page, location, coordinates, results:[...] }`.
  Each result: `{ title, subtitle, location, thumbnail_url, url("/places/<slug>"), id(int),
  hide_from_maps, coordinates:{lat,lng}, distance_from_query }`.
- **Near (geo):** same endpoint, `GET /search?lat=<lat>&lng=<lng>&page=<n>` → results sorted by
  distance; each `distance_from_query` in **miles** (string). NOTE: when lat/lng present, `q` is
  **ignored** (geo path is location-only — verified). No server-side radius param (verified:
  radius/distance/within ignored) → `--radius` is a **client-side** filter on distance_from_query.
- **Show (detail):** `GET /places/<slug-or-numeric-id>` (both forms serve 200; numeric id does NOT
  redirect to slug). HTML with JSON-LD `Place`: name, description, url, geo{latitude,longitude},
  address{street,locality,region,postalCode,country}, image, datePublished, sameAs. Category tags
  via embedded `/categories/<slug>` links. "Know Before You Go" is a free-text section in the body.
  AO has **no universal structured hours** — practical/visit info lives inside Know Before You Go.
- **Category browse:** category pages `GET /categories/<slug>` (HTML, ~27 place links). Categories
  e.g. cemeteries, abandoned, caves, ruins, museums, temples, architecture, natural-wonders, etc.
  Category is NOT a search facet on the geo path → `near --category` = scan nearby results +
  fetch detail pages + filter by category-tag membership (scan-and-filter, bounded).
- **Geocoding (place names → lat/lng):** Atlas Obscura search does not geocode place-name strings
  (place=/near= params ignored). Use **Open-Meteo geocoding** (`geocoding-api.open-meteo.com/v1/search`,
  no auth, free) to resolve `<place>` for `near` and `<cityA>/<cityB>` for `route`.

## Top Workflows
1. "What weird stuff is near me / near <city>?" → `near` (geo sort + radius + category).
2. "Find places about <theme/keyword>" → `search`.
3. "Tell me everything about this place" → `show` (description, KBYG, coords, categories).
4. "What wonders are along my road trip from A to B?" → `route` (corridor scan; NOBODY has this).

## Table Stakes (absorb from competitors)
- search by keyword (node-atlas-obscura, atlas-obscura-api, Apify)
- search by coordinates / nearby (atlas-obscura-api `search({lat,lng})`, travel-hacking-toolkit)
- place by id/slug, short + full detail (atlas-obscura-api `placeShort/placeFull`)
- browse by category (Apify `mode=byCategory`; 26+ categories)
- browse by country / listing pages (`/things-to-do/<country>/places`; Apify, martin0925)
- trending / recently added / featured (Apify `mode=trending`)
- radius filter (Apify `radius`, default 50)
- optional images flag (travel-hacking-toolkit `--images`)
- interestingness scoring / filter mundane markers (travel-hacking-toolkit) — strong idea

## Data Layer
- Primary entity: **place** (id, slug, title, subtitle, location, lat, lng, description,
  categories[], know_before_you_go, image_url, country, region). Persist to SQLite.
- Secondary: **category** (slug, name). **search/near query cache** keyed by params.
- Sync cursor: places are append-mostly; cache by slug/id, fresh-on-read with TTL.
- FTS: full-text over title/subtitle/description/location for offline `search`.

## Codebase Intelligence (community wrappers)
- `bartholomej/atlas-obscura-api` (TS scraper, dominant): `search({lat,lng})`, `placeShort(id)`,
  `placeFull(id)`, `placesAll()`. Numeric IDs.
- `TruitMeGood/node-atlas-obscura`: `getPlaces({city,country})`, `getAllPlaces()`, `search(kw)`.
- `borski/travel-hacking-toolkit` AO skill: `search <lat> <lng>` w/ interestingness scoring +
  mundane-marker filtering, `quick`, `place <id>`, `short <id>`, `--images`, `--all`.
- Apify `crawlerbros/atlas-obscura-scraper`: modes search|byLocation|byCountry|byCategory|trending.
- All competitors are **live scrapers with no local cache** — our SQLite + offline + agent-native +
  corridor routing is the differentiator.

## Product Thesis
- Name: **atlas-obscura** (binary `atlas-obscura-pp-cli`).
- Why it should exist: every existing tool is a thin live scraper. Ours is the only one with a
  local SQLite mirror (offline search, fast repeat queries, fresh-on-read), road-trip **corridor
  routing** no competitor offers, agent-native output (`--json`/`--select`/typed exits), low
  request rate + caching to be a polite scraper, and honest "community-sourced, not an official
  API" labeling throughout.

## Build Priorities
1. Data layer (place + category + query cache, SQLite, fresh-on-read TTL), HTTP client with low
   request rate + browser-like headers, JSON-LD/HTML place parser.
2. Four headline commands: `search`, `near` (geo+radius+category), `show`, `route`.
3. Absorbed: browse by category, browse by country, trending/recent, places index, `--images`.
4. Transcendence: corridor `route`, offline cache, interestingness score, trip/itinerary builder,
   "surprise me" random wonder, visited-tracking. (Finalized in absorb manifest.)

## Browser-Sniff Refinements (Phase 1.7, approved)
- **`search <query>` must use `kind=keyword`**: `GET /search?q=<q>&kind=keyword&page=<n>` gives
  proper relevance-ranked text results (15/page). Default/`kind=place` returns a generic fallback.
- **No category facet exists** in AO's search UI (confirmed by driving the live UI) →
  `near --category` filters client-side by place-page category tags (scan-and-filter, bounded).
- **No usable native geocoder**: `/search/combined?q=` (the suggest XHR) returns HTTP 500
  server-side for all variations → use Open-Meteo geocoding for `near <place>` and `route`.
- Runtime: standard_http, no auth, no browser transport in the shipped CLI.
