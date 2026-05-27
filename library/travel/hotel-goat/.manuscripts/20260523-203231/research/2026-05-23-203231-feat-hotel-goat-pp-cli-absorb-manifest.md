# hotel-goat CLI Absorb Manifest

## Source Tools

Catalogued in parallel via Steps 1.5a searches. Two distinct competitor categories:
- **A. Direct Google Hotels scrapers** (closest competitors — same data source we'll use).
- **B. Adjacent hotel-search MCPs / SDKs / CLIs** (different data source, same user job — features to absorb anyway).

### A. Google Hotels scrapers / APIs

| Tool | URL | Lang | Auth | Notes |
|---|---|---|---|---|
| jongan69/fast_hotels (PyPI: `fast-hotels`) | https://github.com/jongan69/hotels | Python | none (free, protobuf encoding) | The closest 1:1 prior art. Inputs: location (city or IATA), checkin/checkout, guests {adults, children, infants}, room_type {standard, deluxe, suite}, amenities[], sort_by {price, rating}, limit, fetch_mode. Returns name, price, rating, amenities, url. **No currency, no brand, no rating filter, no neighborhood, no OTA breakdown.** |
| MFori/google-hotels-scraper (Apify actor) | https://github.com/MFori/google-hotels-scraper | JS | none direct (paid Apify host) | Inputs: searchQuery, checkInDate, checkOutDate, numberOfAdults, numberOfChildren, currencyCode, maxResults. Returns name, website, address, phone, photos, rating, reviews, **OTA price-per-provider with link**, price range. No filter flags. |
| ScraperHub/google-hotels-scrapers | https://github.com/ScraperHub/google-hotels-scrapers | Python | paid Crawlbase token | Two scrapers: Listing (name/price/rating/link from search) and Detail (address/contact/amenities from property page). |
| luminati-io/google-hotels-api | https://github.com/luminati-io/google-hotels-api | Python/JS | paid Bright Data | Marketing repo for Bright Data's managed Google Hotels SERP API. |
| ScrapingBee/google-hotels-api | https://github.com/ScrapingBee/google-hotels-api | Python | paid ScrapingBee | Same shape: managed scrape with anti-bot. |
| oxylabs/trivago-scraper | https://github.com/oxylabs/trivago-scraper | Python | paid Oxylabs | Trivago metasearch — adjacent data source. |
| oxylabs/expedia-scraper | https://github.com/oxylabs/expedia-scraper | Python | paid Oxylabs | Single-OTA scrape. |
| clementreiffers/hotel-scraping | https://github.com/clementreiffers/hotel-scraping | Python | none (Selenium) | Multi-site (hotels.com, booking, trivago, kayak) Selenium-based bots, CSV output. |
| haoda-li/Booking.com-Scraper | https://github.com/haoda-li/Booking.com-Scraper | Python | none (Selenium) | Single-OTA scraper. |
| **SerpAPI Google Hotels engine** | https://serpapi.com/google-hotels-api | hosted | paid ($50-$250/mo) | The richest competitor feature surface; we mirror its parameter set (location, dates, adults/children/ages, currency, sort_by, min/max_price, rating floor, property_types, amenities, hotel_class, brands, free_cancellation, special_offers, eco_certified, vacation_rentals, bedrooms, bathrooms, property_token for detail lookups). Response per property: name, address, gps_coordinates, hotel_class, check-in/out times, overall_rating + reviews, location_rating, amenities + excluded_amenities, prices[].source/logo/rate, images, nearby_places, reviews_breakdown. **This is the gold-standard feature set we absorb in full.** |

### B. Hotel-search MCPs and SDKs

| Tool | URL | Lang | Auth | Notes |
|---|---|---|---|---|
| esakrissa/hotels_mcp_server | https://github.com/esakrissa/hotels_mcp_server | Python | RapidAPI Booking.com | Tools: `search_destinations(query)`, `get_hotels(destination_id, checkin, checkout, adults)`. Returns rooms, pricing, discounts, ratings, reviews, photos, check-in/out times, star ratings. |
| markswendsen-code/mcp-booking | https://github.com/markswendsen-code/mcp-booking | TS | Booking.com session (browser-automation) | 14 tools: `booking_status, booking_login, booking_logout, booking_search, booking_get_property, booking_check_availability, booking_get_prices, booking_filter_results, booking_sort_results, booking_save_property, booking_book, booking_get_reservations, booking_cancel_reservation, booking_get_reviews`. Only one in the field that actually books. |
| soren-olympus/amadeus-mcp | https://github.com/soren-olympus/amadeus-mcp | Python | Amadeus partner key | Hotel search + booking via Amadeus self-service API. |
| birariro/agoda-review-mcp | https://github.com/birariro/agoda-review-mcp | Python | none | Hotel review aggregator (Agoda). |
| achel-b8/rakuten-hotel-search-mcp | https://github.com/achel-b8/rakuten-hotel-search-mcp | Python | Rakuten Travel | Japan-focused hotel availability. |
| hirochachacha/rakuten_travel_mcp | https://github.com/hirochachacha/rakuten_travel_mcp | Python | Rakuten Travel | Duplicate domain, distinct impl. |
| openbnb-org/mcp-server-airbnb | https://github.com/openbnb-org/mcp-server-airbnb | TS | none | Airbnb listing search + details, no key, respects robots.txt. |
| ivannikolovbg/repull-mcp | https://github.com/ivannikolovbg/repull-mcp | TS | Repull key | Unified API across Airbnb, Booking, VRBO, + 46 channel managers. |
| lev-corrupted/travel-mcp-server | https://github.com/lev-corrupted/travel-mcp-server | TS | Amadeus + AviationStack | Flights + hotels combo MCP. |
| ppiova/TravelMCP | https://github.com/ppiova/TravelMCP | .NET | various | Flights + hotels combo MCP. |
| skarlekar/mcp_travelassistant | https://github.com/skarlekar/mcp_travelassistant | Python | various | Suite of MCP servers; hotel server exposes `search_hotels`, `get_hotel_details`, `filter_hotels_by_price`. |
| borski/travel-hacking-toolkit | https://github.com/borski/travel-hacking-toolkit | mixed | mixed | 5 free MCP servers (Trivago, Airbnb, Ferryhopper + 2) and Claude skills: `compare-hotels` (unified portal+metasearch+Airbnb compare), `premium-hotels` (4,659 Amex FHR/THC + Chase Edit properties with stacking detection), `rapidapi` (Booking pricing), `serpapi` (Google Hotels search + destination discovery), `ticketsatwork` (corp-perks portal). Closest CLI in spirit to what we're building. |
| amadeus4dev/amadeus-node | https://github.com/amadeus4dev/amadeus-node | JS | Amadeus key | Methods: `referenceData.locations.hotel.get` (autocomplete), `referenceData.locations.hotels.byCity/byGeocode/byHotels.get`, `shopping.hotelOffersSearch.get`, `shopping.hotelOfferSearch(id).get`, `booking.hotelOrders.post` (v2), `booking.hotelBookings.post` (v1), `eReputation.hotelSentiments.get`. |
| amadeus4dev/amadeus-python | https://github.com/amadeus4dev/amadeus-python | Python | Amadeus key | Same surface as Node SDK. |
| findhotel/sapi (npm `@findhotel/sapi`) | https://www.npmjs.com/package/@findhotel/sapi | TS | findhotel key | TypeScript hotel search SDK: getHotelOffers, getAvailability (price calendar — same concept as our `dates`), search. |
| google-hotel-api (npm) | https://www.npmjs.com/package/google-hotel-api | JS | unofficial | Thin unofficial Google Hotel wrapper. |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Hotel search by location + date range | SerpAPI Google Hotels | `hotel-goat-pp-cli hotels <location> <checkin> <checkout>` | Free, agent-native, JSON envelope identical to flight-goat. No SerpAPI subscription. |
| 2 | Adults / children / children-ages party config | SerpAPI Google Hotels + esakrissa/hotels_mcp_server | `(behavior in hotel-goat-pp-cli hotels)` `--adults`, `--children`, `--child-ages 5,8`, `--rooms` flags | Real children-ages signal flows through to Google's per-result pricing. |
| 3 | Currency override (ISO 4217) | SerpAPI + MFori/google-hotels-scraper | `(behavior in hotel-goat-pp-cli hotels)` `--currency EUR` global flag | Matches flight-goat `--currency` exactly; reused across `dates` and `compare`. |
| 4 | Sort by cheapest / best / rating / most-reviewed | SerpAPI sort_by; mcp-booking booking_sort_results; jongan69/fast_hotels | `(behavior in hotel-goat-pp-cli hotels)` `--sort cheapest\|best\|rating\|reviews` | Default `best` mirrors Google's curated ranking; `cheapest` is the agent path. |
| 5 | Min/max price filter | SerpAPI min_price/max_price; mcp-booking booking_filter_results | `(behavior in hotel-goat-pp-cli hotels)` `--min-price`, `--max-price` | Per-night defaults; `--total-price` switch for trip-cost budgeting. |
| 6 | Guest-rating floor | SerpAPI rating param | `(behavior in hotel-goat-pp-cli hotels)` `--min-rating 4.0` | Accepts arbitrary float (Google buckets only 3.5/4.0/4.5; we round to nearest). |
| 7 | Hotel-class (star rating) filter | SerpAPI hotel_class | `(behavior in hotel-goat-pp-cli hotels)` `--hotel-class 4,5` | Comma-list, matches flight-goat's `--airlines` shape. |
| 8 | Brand / chain filter | SerpAPI brands; borski premium-hotels skill | `(behavior in hotel-goat-pp-cli hotels)` `--brand Hyatt,Marriott,Hilton,IHG` | `brand_aliases` table expands "Hyatt" → "Park Hyatt", "Andaz", "Thompson", etc. |
| 9 | Amenity filter | SerpAPI amenities; jongan69/fast_hotels amenities[] | `(behavior in hotel-goat-pp-cli hotels)` `--amenities pool,parking,breakfast,gym` | Validated against a seeded amenity vocab; unknown amenities warned but accepted. |
| 10 | Free-cancellation toggle | SerpAPI free_cancellation | `(behavior in hotel-goat-pp-cli hotels)` `--free-cancellation` | Bool. |
| 11 | Special-offers / eco-certified toggles | SerpAPI special_offers, eco_certified | `(behavior in hotel-goat-pp-cli hotels)` `--special-offers`, `--eco-certified` | Bool. |
| 12 | Property-type (hotel vs vacation rental) toggle | SerpAPI vacation_rentals + bedrooms/bathrooms | `(behavior in hotel-goat-pp-cli hotels)` `--type hotel\|rental`, `--min-bedrooms`, `--min-bathrooms` | Default `hotel`; flip to `rental` for the same query against vacation rentals. |
| 13 | Per-result OTA price breakdown | SerpAPI prices[].source/logo/rate; MFori prices-per-provider | `(behavior in hotel-goat-pp-cli hotels)` returns `result.prices[]` with `{source, price, link}` plus `booking_urls.primary` set to cheapest | The killer feature — every OTA Google saw, ranked by price, with deep links. Agents reach `result.prices[0].link` to deep-link the cheapest booking. |
| 14 | Per-result booking links: hotel direct + Google detail | SerpAPI link + serpapi_property_details_link; flight-goat booking_urls shape | `(behavior in hotel-goat-pp-cli hotels)` returns `result.booking_urls = {primary, hotel_url, google_url}` | Identical shape to flight-goat for combo-CLI symmetry. |
| 15 | Hotel detail / property-token deep-dive | SerpAPI property_token + property_details_link; Amadeus hotelOfferSearch(id) | `hotel-goat-pp-cli hotel show <property-token>` | Returns full amenity list, nearby places, check-in/out times, review breakdown, all images. |
| 16 | Hotel ratings / sentiment breakdown | SerpAPI reviews_breakdown; Amadeus eReputation.hotelSentiments; agoda-review-mcp | `hotel-goat-pp-cli hotel reviews <property-token>` | Surface per-category ratings (cleanliness, location, value, service) when Google exposes them. |
| 17 | Hotel-name autocomplete / destination resolve | Amadeus referenceData.locations.hotel.get; esakrissa search_destinations | `hotel-goat-pp-cli resolve <query>` | Disambiguate "Park Hyatt Paris" → property_token; "Paris" → city/neighborhood candidates. Local SQLite first, live fallback. |
| 18 | Hotels-by-geocode (lat/lng search) | Amadeus referenceData.locations.hotels.byGeocode; SerpAPI gps_coordinates | `hotel-goat-pp-cli near "<address>, San Francisco" --radius 1mi` (see transcendence #4) | Built on Google's location search; the radius filter is local geo-math. |
| 19 | Property listing (by city / by IDs) | Amadeus referenceData.locations.hotels.byCity, byHotels | `(generated endpoint) properties listByCity` + `propertyTokens` flag on `hotels` | Listed for completeness; the city sweep is implicit in `hotels <city>`. |
| 20 | Save / wishlist | mcp-booking booking_save_property | `hotel-goat-pp-cli wishlist add <property-token>` | Local SQLite wishlist; survives across sessions, exportable to JSON. |
| 21 | Manage bookings (list / cancel / book) | mcp-booking booking_book/booking_get_reservations/booking_cancel_reservation; Amadeus booking.hotelOrders | `(stub - Google Hotels does not transact; bookings happen on the OTA. Deferred until/unless an OTA-side adapter is added.)` | Honest stub — Google Hotels is metasearch, not a booking endpoint. We deep-link out. |
| 22 | Review aggregation | mcp-booking booking_get_reviews; agoda-review-mcp; SerpAPI google_hotels_reviews_link | `hotel-goat-pp-cli hotel reviews <property-token>` (same as #16) | Single command for reviews + sentiment breakdown. |
| 23 | Price calendar / availability calendar | findhotel/sapi getAvailability; (flight-goat `dates` analog) | `hotel-goat-pp-cli dates <location> [--from --to --nights]` | Sweep a date window, return cheapest N-night blocks. Mirrors flight-goat `dates ORIG DEST`. |
| 24 | Multi-city comparison | borski compare-hotels skill | `hotel-goat-pp-cli compare <location-csv> --checkin --checkout` | Mirrors flight-goat `compare`. See transcendence #5 for the cross-city expansion. |
| 25 | Hotel images gallery | SerpAPI images[].thumbnail/original_image | `(behavior in hotel-goat-pp-cli hotel show)` returns `images[]` array | Plain URLs; agent can fetch directly. |
| 26 | Nearby places / points of interest | SerpAPI nearby_places | `(behavior in hotel-goat-pp-cli hotel show)` returns `nearby_places[]` | What Google shows under "transportation & nearby". |
| 27 | Health & safety / COVID protocols | SerpAPI health_and_safety.groups | `(behavior in hotel-goat-pp-cli hotel show)` returns `health_safety` block when present | Pass-through. |
| 28 | Pagination across result sets | SerpAPI next_page_token | `(behavior in hotel-goat-pp-cli hotels)` `--limit N` and `--page N` | We surface enough that most queries fit one page; `--page` accesses subsequent pages. |
| 29 | Session-based authenticated flows (login/logout/status) | mcp-booking booking_login/logout/status | `(stub - Google Hotels has no authenticated surface for browsing rates; Google Travel saved-trips would require OAuth and is out of scope for v1.)` | Honest stub. |
| 30 | Premium-stay / loyalty-portal stacking | borski premium-hotels skill | `(behavior in hotel-goat-pp-cli hotels)` `--brand` matches our brand_aliases; full Amex FHR/Chase Edit stacking is out of v1 scope | We surface the Hyatt/Marriott/Hilton/IHG/Accor sub-brand union; premium-portal stacking is a future v0.2 (would require curated 4,659-property list ingest). |
| 31 | Trivago-style metasearch (multiple OTA sources for one hotel) | oxylabs/trivago-scraper | `(behavior in hotel-goat-pp-cli hotels)` covers it via per-result `prices[]` — Google Hotels itself is a metasearch over OTAs | We don't need a separate Trivago scrape; Google Hotels surfaces the same OTAs. |
| 32 | Sync to local SQLite | maxmypoint-pp-cli sync; flight-goat sync | `hotel-goat-pp-cli sync` (generated framework command) | Standard auto/live/local data-source flag inherited from the root flag set. |
| 33 | Full-text search across synced properties | flight-goat search | `hotel-goat-pp-cli search "<query>"` (generated framework command) | FTS5 over name + brand + neighborhood + amenities. |
| 34 | Export / import (JSONL backup) | flight-goat export/import | `hotel-goat-pp-cli export` / `import` (generated framework commands) | Standard. |
| 35 | Tail (poll for changes) | flight-goat tail | `hotel-goat-pp-cli tail` (generated framework command) | Standard. |
| 36 | Doctor (health check) | flight-goat doctor | `hotel-goat-pp-cli doctor` (generated framework command) | Standard. |
| 37 | Profile (saved flag sets) | flight-goat profile | `hotel-goat-pp-cli profile` (generated framework command) | Standard. |
| 38 | Which (capability → command resolver) | flight-goat which | `hotel-goat-pp-cli which` (generated framework command) | Standard. |
| 39 | Feedback capture | flight-goat feedback | `hotel-goat-pp-cli feedback` (generated framework command) | Standard. |
| 40 | Agent-context emitter | flight-goat agent-context | `hotel-goat-pp-cli agent-context` (generated framework command) | Standard. |

## Transcendence (only possible with our approach)

Sourced from the inline novel-features brainstorm (Step 1.5c.5). Customer model: travel hackers (points/loyalty optimizers), date-flex leisure travelers, families with kids, business travelers near venues, agents composing itineraries via the future `travel-pp-cli`. Generated 2× the target count of candidates, then adversarial-cut to survivors scoring ≥5/10.

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Cheapest N-night window across a month | `hotel-goat-pp-cli cheapest-window <location> --min-nights 3 --max-nights 5 --month 2026-08 [--max-price 300] [--min-rating 4.0]` | hand-code | Iterates every (checkin, checkout) pair in the month under the night-count constraints, calls `hotels` per pair with the user's filters, returns the cheapest window. Google Hotels web UI forces one date pair at a time; this is the date-flex traveler's #1 request. |
| 2 | Price drift / history for a single hotel | `hotel-goat-pp-cli drift <property-token-or-name> [--days 30]` | hand-code | Plots local SQLite `price_snapshots` over time. Google Hotels has no historical view at all — every search is the live point-in-time. Only possible because we append-only-snapshot every search to disk. |
| 3 | Brand-loyal search expanded to all sub-brands | `hotel-goat-pp-cli brand-loyal --program hyatt --location "Paris" --checkin 2026-08-15 --checkout 2026-08-17` | hand-code | Uses local `brand_aliases` to expand a loyalty program (hyatt, marriott_bonvoy, hilton_honors, ihg_one, accor_all) to all sub-brands (Park Hyatt, Andaz, Thompson, etc.), then filters Google Hotels results to only those. Google's UI doesn't group by loyalty program. |
| 4 | Geo-radius search around an address | `hotel-goat-pp-cli near "<address>, San Francisco" --radius 1mi --checkin 2026-08-15 --checkout 2026-08-17 [--max-price 300]` | hand-code | Geocodes the address (Google Hotels accepts free-text or coords), filters response by Haversine distance from the geocoded point. Conferences, school visits, hospital stays. Google's web UI requires panning a map. |
| 5 | Compare same nights across multiple cities | `hotel-goat-pp-cli compare-cities "Paris,Lyon,Marseille" --checkin 2026-07-20 --checkout 2026-07-25 [--min-rating 4.0]` | hand-code | Fans `hotels <city>` across the city list in parallel, returns a stacked-by-city ranked table + summary stats (median price per city, rating distribution). The "where should we go this week" question. |
| 6 | Family-stay search with kid-aware ranking | `hotel-goat-pp-cli family <location> --checkin --checkout --rooms 2 --kids 2 --child-ages 5,8 [--require-amenities pool,breakfast]` | hand-code | Multi-room logic + amenity defaults tuned for families (pool, breakfast, kitchen, kid-friendly) + rank boost for properties with explicit "Family friendly" tags. Surfaces total cost across rooms, not per-room. Google's web UI loses the kid signal in the rank. |
| 7 | Watch / alert on price drop | `hotel-goat-pp-cli watch <property-token-or-name> --checkin --checkout --threshold-pct 10` | hand-code | Compares latest snapshot to prior snapshots in SQLite; exit 0 unchanged, exit 2 with summary on ≥N% drop. Cron-friendly. Same pattern as maxmypoint-pp-cli watch. Google Hotels offers no native price-drop alerts. |
| 8 | Budget bundle: best stay under total budget | `hotel-goat-pp-cli bundle <city> --nights 3 --budget 1500 --pax 4 [--checkin-window 2026-08-01:2026-08-31]` | hand-code | Given a budget envelope and party size, sweeps date windows + hotel options, returns the best (cheapest + highest-rated) 3-night stay that fits. The "I have $1500 and need 3 nights in Lisbon for 4 people, pick something good" question. |
| 9 | Agent-composable nested JSON via --select dotted paths | `hotel-goat-pp-cli hotels "San Francisco" 2026-08-15 2026-08-17 --agent --select results.name,results.rating,results.prices.source,results.prices.price,results.booking_urls.primary` | spec-emits | Dotted-path selection through the nested `prices[]` and `booking_urls` blocks. Inherited from the standard `--select` flag; the per-hotel feature is having a nested-enough response that selecting through it is worth the example. |

## Stubs

- **Row 21 (Manage bookings — `book`, `bookings`, `cancel`):** Google Hotels is a metasearch, not a booking endpoint. Bookings happen on the deep-linked OTA. Honest stub: `book` emits "hotel-goat surfaces booking_urls. Use --json to extract the deep-link, then book on the OTA site." Deferred unless/until a per-OTA booking adapter is added (out of v1 scope — would require Booking.com/Expedia/Hotels.com API partnerships).
- **Row 29 (Session-based auth — `auth login`, `auth status`):** Google Hotels has no authenticated browsing surface for rates. Google Travel saved-trips would require OAuth and is out of v1 scope. Honest stub.
- **Row 30 (Premium-stay portal stacking — Amex FHR / THC / Chase Edit):** The `--brand` flag covers loyalty-program filtering; full premium-portal stacking would require ingesting borski/travel-hacking-toolkit's curated 4,659-property list and detecting which booking codes stack. Deferred to a future v0.2. Not stubbed as a command — simply not built; `hotels --brand` is the v1 path.

All other rows are shipping scope.
