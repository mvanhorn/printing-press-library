# HotelTonight Browser-Sniff Discovery Report

**Goal:** capture geo/city-autocomplete + hotel-detail endpoints the direct-HTTP probe missed.
**Method:** Chrome (claude-in-chrome MCP) walked the search flow on www.hoteltonight.com (anonymous), plus direct `curl` confirmation of every discovered endpoint.
**Runtime classification:** `standard_http` — all consumer endpoints reachable header-only via plain stdlib HTTP. No Akamai/TLS block, no clearance cookie, no browser at runtime. The research brief's HIGH reachability fear did not materialize for the consumer surface.

## Auth model
- **No credentials.** Endpoints gate on *app-identifying* headers (`ExpectedMobileApiException` when missing), not on a token.
- Required static headers (baked into the generated client via `required_headers`):
  - `X-App-Platform: iphone`
  - `X-App-Version: 2024.40`
  - `X-App-Country: US`
  - `X-App-Currency: USD`
  - `X-App-Device: iPhone`
  - `X-App-Time-Zone: America/Los_Angeles`
  - `Accept: application/json`
- CORS `Access-Control-Allow-Headers` enumerated the full accepted set: `Token, X-App-CXID, X-App-Country, X-App-Currency, X-App-Debug, X-App-Device, X-App-Experiments, X-App-Meta-Partner, X-App-Platform, X-App-SessionId, X-App-Time-Zone, X-App-Version, X-Segment-Anonymous-Id`. Only the platform/version/country/currency/device/timezone subset is needed for anonymous reads.

## Endpoints (all confirmed HTTP 200 header-only)

| Method | Path | Purpose | Key params |
|--------|------|---------|-----------|
| GET | `/v6/inventory` | **Primary** deal/inventory feed (geo search) | `latitude`, `longitude`, `check_in` (YYYY-MM-DD), `check_out`, `rooms` |
| GET | `/v2/market_cities` | Curated list of ~20 major markets (geo resolver: name→id/lat/lng) | none |
| GET | `/v2/market_cities/{id}` | Market detail (id, city, lat/lng, seo_slug, state) | path id |
| GET | `/v2/market_cities/{id}/popular_market_cities` | Nearby/popular markets for a market | path id |

- Base host: `api.hoteltonight.com`. Image base: `imagery.hoteltonight.com`.
- Hotel-detail endpoints (`/v2|v6/hotels/{id}`, `/rooms`, `/availability`) all 404 — **no separate hotel-detail endpoint exists**. The `/v6/inventory` feed embeds full hotel data per deal, so hotel detail is derived from inventory, not fetched.
- Web autocomplete is **client-side / prefetched** (bundled popular-markets list filtered as you type), not a live API — so there is no autocomplete endpoint to wrap. `/v2/market_cities` is the server-side equivalent and is what the CLI uses for name→geo resolution.

## /v6/inventory response shape (high-gravity fields)
- Top-level: `primary_market` (id, city_name, display_name, lat/lng, seo_slug, state, iana_time_zone), `nearby_markets`, `booking_window` (days, max_length_of_stay), `currency` (iso_code, display_format), `current_day`, `num_nights`, `room_count`, `search_controls` (presets: default/budget/premium; amenities), `room_ota_comparison_discounts_formatting` (deal-type taxonomy), `rooms[]`.
- `rooms[]` (the deals): `id`, `deal_id`, `deal_type` (`standard`, `classic_deal`, `ht_perks`, `segmented_offer_nearby`, `daily_drop`...), `customer_price_per_night`, `customer_price_per_night_for_display`, `strikethrough_price` (highest rate in last 30 days → instant % off), `total_customer_price`, `num_remaining`, `available`, `sold_out`, `cancelable`, `amenities`, `value_adds`, `hotel{...}`.
- `rooms[].hotel`: `id`, `name`, `slug`, `neighborhood`, `address`, `city`, `state`, `zipcode`, `latitude`, `longitude`, `market_city_id`, `market_city_name`, `review_count`, `percent_positive_reviews`, `positive_review_count`, `category`, `description`, `why_we_like_it`, `photos`, `iana_time_zone`.

## Build implications
- Generate with a hand-authored internal YAML spec (`auth: none` + `required_headers`), `--spec`. No HAR/browser-sniff generate path needed — runtime is plain HTTP.
- `strikethrough_price - customer_price_per_night` gives % off as a first-class derived field (table-stakes per brief).
- Local SQLite `price_snapshots` keyed on (hotel_id, geo/market, check_in, observed_at) powers the transcendence price-drop-watch features.
