# hotel-goat CLI Brief

## API Identity
- Domain: Google Hotels (https://www.google.com/travel/hotels and the /travel/search?q=hotels+... entry surface).
- What it is: Google's hotel metasearch — aggregates rates from Booking.com, Expedia, Hotels.com, Agoda, the hotel's own site, and dozens of OTAs into one ranked list for a city/area/landmark on specific dates.
- Users: leisure travelers picking a hotel for a vacation block, business travelers near a venue/airport, families with date flexibility, points/loyalty hackers cross-checking cash vs award rates, and (the strategic case) AI travel agents composing an itinerary across flights + hotels + awards.
- Data profile: per-search result set of ~30-100 hotels with name, brand chain, address, lat/lng, star rating, guest rating + review count, lead price (lowest of all OTAs for the date range), an OTA breakdown of `(source, price, link)`, amenity tags, neighborhood, image URLs, and a stable Google `property_token` for deep-dive lookups. Date-keyed: prices vary nightly, the property index is mostly stable.

## Reachability Risk
- **None observed.** Direct `curl https://www.google.com/travel/search?q=hotels+San+Francisco&checkin=2026-08-15&checkout=2026-08-17&hl=en` with a plain Chrome User-Agent returns HTTP 200, 2.83 MB of HTML, and two `AF_initDataCallback({key: 'ds:0', ...})` / `ds:1` blobs containing the structured hotel data. No API key, no auth, no JS execution required. Architecture is identical to flight-goat's Google Flights scrape — same `AF_initDataCallback` envelope, same SSR-with-embedded-JSON pattern.
- **Why this is durable:** Google Hotels has been on this `AF_initDataCallback`/`ds:N` shape for years (Google Travel's whole SPA stack). It can break when Google reshuffles internal keys, but the entry path stays plain HTTP. Mitigations: pin a UA, parse defensively per ds-index, keep a fallback path to re-fetch the property detail page (`/travel/hotels/entity/<token>`) which carries the same envelope.
- **Commercial confirmation:** SerpAPI sells Google Hotels as a managed scrape engine ($50-$250/mo, capped at 1K-30K searches). That market exists precisely because the raw fetch works — they sell anti-bot rotation + parsing, not access to a private API. We are bypassing the $50/mo floor.

## Top Workflows
1. **Spot-check a hotel for a specific date range** — "what's the cheapest 3-night stay in Paris Jul 20-23 under €300/night, rating 4.0+?". Mirrors flight-goat's `flights ORIG DEST DATE` shape: one command, one location, one date range, sorted by price/rating with hard filters.
2. **Date-flex search across a window** — "I want 3 nights in Lisbon sometime in August; which dates are cheapest?". Same pattern as flight-goat `dates`: scan a date range, return the cheapest N-night window for the cheapest hotel matching filters.
3. **Brand/loyalty-affiliated search** — "only show Hyatt/Marriott/Hilton/IHG properties in this city". Travel hackers won't book a random hotel when they're trying to earn/burn loyalty status. Critical filter that Google's web UI buries.
4. **Multi-city / multi-leg comparison** — "I'm choosing between Lyon, Marseille, and Nice for the same week; which is cheapest with comparable ratings?". The orchestration the future `travel-pp-cli` will lean on.
5. **Geo-radius search around a specific point** — "hotels within 1mi of <address> SF, max $300/night". Conferences, school visits, hospital stays, family events. Web UI requires panning a map; CLI does it in one call.
6. **Family / multi-room booking** — "2 rooms, 2 adults + 2 kids each, kids-friendly amenities". Web UI handles party config but loses the kid-specific signal in the rank.

## Table Stakes
Inferred from SerpAPI's Google Hotels engine surface, the borski/travel-hacking-toolkit hotel skills (compare-hotels, premium-hotels, rapidapi, serpapi), the esakrissa/hotels_mcp_server tool set, MFori/google-hotels-scraper inputs, jongan69/fast_hotels, and the Booking.com MCP (markswendsen-code/mcp-booking) feature list:

- Location + date-range + party-size query (adults, children, children ages, rooms).
- Currency override (ISO 4217).
- Hotel-class filter (2/3/4/5-star).
- Guest-rating floor (3.5+/4.0+/4.5+).
- Price range (min/max per-night).
- Amenity filter (pool, parking, breakfast, wifi, gym, pet-friendly, kitchen, etc.).
- Brand filter (chain affiliation — Hyatt, Marriott, Hilton, IHG, Accor, Wyndham, Choice, etc.).
- Property-type toggle (hotel vs vacation rental).
- Sort: cheapest / best / rating / most-reviewed.
- Free-cancellation / special-offers / eco-certified flags.
- Per-result OTA breakdown — every booking source Google saw, with price and deep-link, so users don't get stuck with one OTA's rate.
- JSON + table output; per-result deep-link to hotel and Google Hotels page.

## Data Layer
- **Primary entities deserving local SQLite:**
  - `properties` (property_token, name, brand_chain, address, lat, lng, hotel_class, neighborhood, amenities JSON) — stable enough to cache across searches.
  - `price_snapshots` (property_token, checkin, checkout, source_ota, price_total, price_per_night, currency, captured_at) — every search result row; powers the `drift` novel command (price-over-time charting from local history).
  - `searches` (id, location, checkin, checkout, party_json, filters_json, captured_at, result_count) — search audit log.
  - `brand_aliases` (chain_id, brand_name, sub_brands JSON) — static lookup so `--brand Hyatt` matches "Park Hyatt", "Andaz", "Thompson", etc. Seeded at build time.
- **Sync cursor:** per-property `captured_at`. Properties refresh on demand when re-queried; price snapshots append-only (never overwritten — that's what makes `drift` work).
- **FTS/search:** FTS5 over `properties.name + brand_chain + neighborhood + amenities` for offline `search "boutique pool paris"` lookups against everything previously seen.
- **Why this matters:** Hotel rates change daily and Google Hotels offers no historical view. Local snapshots are the moat — every `hotels` call seeds `drift`, `brand-history`, `cheapest-window-by-hotel`. The same SQLite-as-moat pattern flight-goat uses for reliability/disruption history.

## User Vision
hotel-goat is a building block for a future `travel-pp-cli` orchestrator that combines flight-goat + hotel-goat + seats-aero + maxmypoint + a points-wallet TOML for "rough date + location → full itinerary with cash and points cost." Agents are the primary consumer. Therefore the CLI MUST:

- Mirror flight-goat's UX conventions exactly: `hotels <location> <checkin> <checkout>`, `dates <location>`, `compare <location>`, identical global flag set (`--agent`, `--json`, `--compact`, `--select`, `--currency`, `--sort`, `--no-cache`, `--data-source`).
- Emit first-class JSON envelopes with `meta.source = "google-hotels"`, ISO timestamps, stable field names, `meta.search_query` echoed back.
- Per-result `booking_urls` object matching flight-goat: `{primary, hotel_url, google_url}` — primary = top OTA deep-link, hotel_url = direct-to-hotel-site link when Google surfaces one, google_url = the Google Hotels detail page.
- Be composable: `hotels SFO 2026-08-15 2026-08-17 --agent --select results.name,results.price,results.rating --max-price 300 | jq` should just work.
- Keep filters as flat flags, not nested config: `--sort cheapest`, `--max-price 300`, `--min-rating 4.0`, `--brand Hyatt,Marriott`, `--neighborhood "Marais"`.

## Product Thesis
- **Name:** `hotel-goat-pp-cli` (binary `hotel-goat-pp-cli`, slug `hotel-goat`, matches the flight-goat naming family).
- **Why it should exist:**
  1. **No free, agent-native Google Hotels CLI exists today.** Everything in the landscape is either (a) a paid SaaS scrape API (SerpAPI, Bright Data, ScrapingBee, Apify, $50+/mo with hard request caps), (b) an OTA-specific MCP that locks you to Booking.com / Expedia / Airbnb individually, (c) a Python scraper library you have to wire up yourself, or (d) the Amadeus SDK which requires partner credentials and only sees a fraction of inventory. None of these match flight-goat's "one binary, no API key, agent-first, local SQLite, $0" shape.
  2. **The OTA breakdown is the killer feature.** Every booking aggregator buries which source has the lowest price; Google Hotels exposes it. Surfacing `prices[].source + price + link` in JSON makes hotel-goat the only tool an agent needs to write "find me the cheapest booking link for X" — no need to fan out to Booking + Expedia + Hotels separately.
  3. **It unlocks the travel-pp-cli combo.** The orchestrator needs a hotel side that speaks the same flag-and-envelope dialect as the flight side. Building hotel-goat to flight-goat's exact shape means the combo CLI becomes a thin merge layer, not a rewrite.
  4. **Local price history is a real moat.** No incumbent — free or paid — gives you "what did this hotel cost the last 5 times I searched?". A SQLite snapshot per search costs nothing to write and powers `drift`, `brand-history`, and the eventual travel-pp-cli's "is this a good deal?" question.

## Build Priorities
1. Generated endpoint surface (or hand-coded fetch + parse, depending on what Phase 1.7 browser-sniff produces) for the Google Hotels search HTML → AF_initDataCallback `ds:0` JSON extraction pipeline, with `property_token`-keyed detail-page lookups.
2. Three headline hand-coded commands matching flight-goat's shape: `hotels <location> <checkin> <checkout>` (primary), `dates <location>` (date-flex), `compare <location>` (price comparison across dates).
3. Local SQLite store: `properties`, `price_snapshots`, `searches`, `brand_aliases`. FTS5 over the property index. Append-only price history. Standard sync/search/import/export.
4. Filter flags: `--sort cheapest|best|rating|reviews`, `--max-price`, `--min-rating`, `--brand`, `--neighborhood`, `--hotel-class`, `--amenities`, `--free-cancellation`, `--currency`, `--adults`, `--children`, `--rooms`.
5. Per-result `booking_urls`: `{primary, hotel_url, google_url}` — same shape as flight-goat.
6. Transcendence commands per the absorb manifest's Transcendence table — minimum 5, ideally 6-8. Locked in at Phase Gate 1.5.
7. `--agent`, `--json`, `--compact`, `--select`, `--no-cache` global flags inherited from the standard root flag set, so every command (generated or hand-coded) is agent-composable from day one.
