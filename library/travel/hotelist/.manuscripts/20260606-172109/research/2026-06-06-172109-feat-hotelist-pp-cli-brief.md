# Hotelist CLI Brief

## API Identity
- **Domain:** AI-rated hotel discovery. Hotelist.com (by @levelsio / Pieter Levels, part of the Nomads.com ecosystem). No official/public API — data is scraped from the site's own JSON endpoint.
- **Users:** Travelers and digital nomads who distrust pay-to-play booking sites; people who want hotels filtered on *real* (photo-verified) amenities — a gym with actual weights, a real coworking desk, a bathtub that exists.
- **Data profile:** 84,137 hotels across 5,994 cities / 153 countries, ratings AI-normalized across 483,360 data points (last updated May 25, 2026). Rich per-hotel data: Hotelist Score, AI photo rating, AI review rating, consensus, per-source normalized ratings, pros/cons, verified amenities, price/night, year built, chain.

## Reachability Risk
- **None.** Live `GET https://hotelist.com/api` returns HTTP 200 JSON with a plain `curl` + UA + `X-Requested-With` header. No auth, no bot protection, no clearance cookie. Direct HTTP fully replayable.
- Probe-safe endpoint used: `GET /api` with `filters[0][target]=geohash&filters[0][value]=ezdn` → 200, 22 hotels for A Coruna.
- `POST /modal/{hotel_id}` → 200, 21KB HTML detail fragment.

## Discovered Contract (reverse-engineered from site JS, live-verified)
- **`GET https://hotelist.com/api`** — main search/filter. Params (jQuery nested-array serialization):
  - `filters[]` — array of `{target, value, type}`. Targets observed:
    - `geohash` (type `starts_with`) — **city location key**. Cities map to geohash prefixes (e.g. A Coruna = `ezdn`).
    - `country` (type `exact-match`) — country name string.
    - `bbox` (`{lat_min,lat_max,lng_min,lng_max}`) — map bounds (used by map UI).
    - `parent_chain_code` (exact-match) — hotel chain rollup (e.g. Marriott, Hilton).
    - `chain_code` (exact-match) — exact sub-brand (RZ, WI, SI…).
    - `boutique` = 1 / `collection` = 1 (exact-match) — independent / small-luxury rollups.
    - `amenities` (type `contains`) — matches `ai_amenities_json` column; multiple selections AND together (one LIKE each).
    - `either_amenities` (type `contains`) — merged amenity, OR'd across columns.
    - `hotellist_rating` (greater-than / less-than) — rating slider.
    - `price` (greater-than / less-than) — price slider.
    - `year_built` (greater-than / less-than) — newness slider.
    - `internet_json` (greater-than) — wifi-speed filter.
    - `sort-by` `{key, order}` — keys include `best-value` (rating/$), `hotellist_rating`, `price`, `year_built`, `year_renovated`.
  - `search` — top-level free-text param (filters visible hotels by name; server drops other filters so the searched hotel can always be found).
  - **Response JSON:** `{ hotels:[...], price_histogram, rating_histogram, year_built_histogram, grid_html, query_timings, runtime, runtime_ms }`.
  - **Hotel object fields:** `hotel_id, name, hotellist_rating, price, latitude, longitude, photo, pros, cons, year_built, parent_chain_code, rating_color, youtube_id`.
- **`POST https://hotelist.com/modal/{hotel_id}`** — hotel detail, returns HTML fragment (Hotelist Score, amenity breakdown, AI rating detail). Optional `?amenity=<name>` highlight param.
- **City → geohash table** — embedded in homepage/city-page `<select class="city-selector">` `<option value="<geohash>" data-country="<Country>" data-region="<Region>">City Name</option>`. ~6,000 cities. Scrape once, cache.
- **Amenity vocabulary (verified, exact site labels):** Modern interior, Traditional interior, Kitchen, Bath, Restaurant, Jacuzzi, Cinnamon rolls, Steak, Ocean view, Beach view, Mountain view, Ski hotel, Weightlifting gym, Squat rack, Sauna, Infrared sauna, Nespresso, Coffee maker, Kettle, Working desk, Coworking, Pets allowed, Gym, Pool, Tennis court, US Presidents stayed, Blackout blinds, Business center, Beach, Adults only, Massage, Parking, Iron.

## Top Workflows
1. **Find good hotels in a place** — `search bangkok` → AI-rated hotels sorted by Hotelist Score, with pros/cons.
2. **Filter on real amenities** — "hotels in Lisbon with a real weightlifting gym and a pool" → photo-verified, not hotel-claimed.
3. **Best value (rating per dollar)** — `value tulum` or `value marriott` → rank by Hotelist's own best-value sort, or compute rating/price locally and compare across a chain.
4. **Deep-dive one hotel** — `show <hotel-id>` → full Hotelist Score, AI photo/review ratings, consensus, amenities, pros/cons.
5. **Compare chains** — which chain delivers the best rating-per-dollar in a region (transcendence).

## Table Stakes (vs. how a traveler uses the site)
- Search by city / country / region / chain.
- Amenity filtering (gym/pool/tennis + arbitrary amenity).
- Sort by score, price, value, newness.
- Per-hotel detail with AI rating breakdown.
- Price-range and rating-range filtering.
- Currency display (site supports USD/EUR/GBP/… — prices come back in a base currency).

## Data Layer
- **Primary entities:** `hotels` (keyed by `hotel_id`), `cities` (city → geohash, country, region), `chains` (code → display name, parent).
- **Cache:** fresh-on-read SQLite. Hotel result sets keyed by (location, filter-hash); city/chain reference tables synced once and refreshed on demand. Stale-after window domain-appropriate (hotel data updates ~monthly per the stats page → default ~7 days for result sets, longer for the city/chain reference tables).
- **FTS/search:** offline search over cached hotels by name, pros/cons text, city.

## `--checkin` / `--checkout` reality
- Hotelist's `/api` has **no date-based pricing**. `price` is a single AI-derived nightly figure, not a date-specific quote. The site has no date pickers. The requested `--checkin`/`--checkout` flags therefore cannot change `/api` results. Decision: accept the flags for ergonomic parity but **emit an honest note** that Hotelist prices are AI-estimated nightly averages, not live date-specific quotes, and pass them through only as display context. Do not fabricate date filtering. (Flag this at the gate.)

## Product Thesis
- **Name:** `hotelist` (binary `hotelist-pp-cli`).
- **Why it should exist:** Hotelist has no API and no CLI. This puts levelsio's anti-pay-to-play, photo-verified-amenity, AI-normalized hotel data into the terminal and into agent context — composable with `--json`/`--select`, with an offline SQLite mirror so an agent can run `value`, `rating/$` comparisons, and amenity cross-filters that the website UI can't express (chain-vs-chain value, multi-city best-value, amenity-verification deltas). Firehose → queryable local dataset.

## Build Priorities
1. Internal YAML spec for `/api` (search/filter) + `/modal` (show) + city-geohash sync; SQLite store for hotels/cities/chains; fresh-on-read cache; polite rate limit.
2. The four user commands: `search`, `filter`, `value`, `show` — all with `--json`, honest Hotelist labeling.
3. Transcendence: chain value comparison, multi-location best-value, amenity-verification cross-filter, local rating/$ ranking, "exceptional" badge logic (score 8+, high consensus, built <10y).

## Labeling / Honesty
- Every output and the README/SKILL must state data is **scraped from Hotelist.com (community/AI-rated, not an official API)**. Ratings are AI-normalized; prices are AI-estimated nightly figures.
