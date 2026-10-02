# Traveloka runtime contract for the pending build

Status: source discovery and implementation contract, not implemented CLI or Phase 18 evidence. Absorb approval is still pending. Primary evidence is the sanitized browser capture and standalone replay proofs in this run's discovery directory.

## Runtime and session seam

Normal CLI commands must be direct Go HTTP against the pinned origin https://www.traveloka.com. No ordinary-command browser sidecar. Initial capture/refresh can use the already installed browser-use in an isolated, task-owned anonymous guest session. All capture paths must be explicitly restricted to Traveloka cookies and read-only flight/hotel request profiles; do not export full browser state or read unrelated site credentials. No global installation or account changes.

An imported private session includes Traveloka-only cookies plus per-operation header and sentinel profiles. Accept only exact www.traveloka.com or traveloka.com cookie domains (including their leading-dot form), valid paths and expiry semantics; reject any foreign-domain cookie, foreign-host request profile, or nonallowlisted operation. Preserve private file permissions and redact all credential values from errors/dry-run/debug. A public quote snapshot contains normalized queries and returned offer fields only, never cookie/token/header profiles, user-context capsules, inventoryRateKey, or tracking payloads.

Observed read-only POST allowlist and x-domain:

| Path | Product header |
|---|---|
| /api/v2/airport/search-nexus | flightDiscovery |
| /api/v1/hotel/autocomplete | accomContent |
| /api/v2/hotel/autocomplete/features | accomContent |
| /api/v2/flight/search/initial | flight |
| /api/v2/flight/search/poll | flight |
| /api/v2/flight/search/redirection | flight |
| /api/v2/hotel/searchList | accomSearch |
| /api/v2/hotel/search/rooms | accomRoom |

Import the source operation-specific www-app-version, User-Agent, approved request-token headers and request sentinel when present. Do not manufacture protection tokens. Flight search header fpr-search-id is an ordinary fresh UUID and must match data.searchId. Same-origin POST headers are essential: Origin=https://www.traveloka.com, Sec-Fetch-Site=same-origin, Sec-Fetch-Mode=cors, Sec-Fetch-Dest=empty, and a JSON Accept/Content-Type. Hotel autocomplete returned upstream 500 without these and 200 with them: standalone-location-origin-replay.json.

Shopper flags populate tv-country, tv-language, tv-currency, x-route-prefix and payload currency where present; record the exact market/locale/currency in the public query. Translate public en-SG to source en_SG and en-sg route prefix. Do not silently relabel a returned price in the requested currency: validate the source currency. SGD and USD are proven for SG/en-SG dated hotel HTTP replay; broader market/locale behavior still needs delivered-CLI verification.

Export cookies after normal source interactions, not just before them. A multi-party hotel replay returned HTTP 202 with an empty body; after actual normal-source request capture and a fresh scoped cookie export, the next HTTP replay returned 200. Treat this as protection/access state, never no inventory. Ordinary commands should give a precise explicit refresh action on expired/protected access, without looping identical failures or invoking CAPTCHA solving. Final acceptance must include Go HTTP after the task browser has closed.

Reject credential-bearing redirects away from the pinned origin. Limit response bytes, request duration, polls, grid cells and result limits; distinguish a truncated/incomplete source search from a completed empty search. Honour source refresh delay within a bounded search deadline. Any returned reusable sentinel/cookie refresh stays only in the private session store.

## Airport and hotel resolution

Airport search body data has query, filters, frequentAirport, originAirport and showTags. Results are source-ranked sections[].results[]; directory is a large nested array, not the query result set. Preserve code/entityId, type, areaCode, iataCode, country, location/geoLocation and displayData title/subtitle. CITY and AIRPORT are distinct; do not invent a single airport when the query resolves to a city or several entities.

Hotel autocomplete body data has query and experimentContext.mapParamKeyToVariant.varAutocompleteLogic=control. Response data groups rows under geoRegionContent, geoCityContent, geoAreaContent, landmarkContent and hotelContent; autoCompleteContent can be empty while other groups have valid results. Preserve source id/type/name/displayName/country and hotel geoId/attributes. Singapore region is 107493; Bangkok region is 10000045; Berkeley hotel is 9000000001714. Filter and bound actual source matches, not unrelated popular/recent suggestions. An upstream 500 remains an upstream error.

## Flights

ONE_WAY or ROUND_TRIP initial body uses seatPublishedClass, journeys with source airport codes and ISO departureDate, journeyIndex=0, selectedFlights=[], selectedFlightsContext={}, numSeats(numAdults/numChildren/numInfants), a fresh searchId, currency and the observed additionalData/filter/inventoryPricingDisplayType/trackingMap shape. Stale search IDs yielded empty completed responses; a new client UUID produced live inventory.

Accumulate searchResults by source ID across initial and incremental poll replies. Empty incremental polls must not erase existing results. meta.searchCompleted, refresh delays, expiryTimeStamp and source sort types are separate metadata. The bounded coverage statement must be honest.

For return options, preserve both journeys, use poll with journeyIndex=1 and selectedFlights=[outboundID], selectedFlightsContext[outboundID].isBaggageFilterEnabled=false. Inbound display fares can be deltas, including zero. For each retained combination, read-only redirection/prefetch uses isPrefetch=true, isBreakSmartCombo=false, journeyIds and journeyContextData. Use its totalPrice as authoritative party trip total and displayedPricePerPax separately; never sum outbound and inbound display fares. Prefetch is source-observed search preparation, not a booking or payment request.

Preserve connectingFlightRoutes and every segment's local departure/arrival dates/times, tzDepartureMinuteOffset/tzArrivalMinuteOffset, airport codes, marketing airline/brand, operatingAirlineCode, flightNumber, seatClass/cabin, durationMinutes, transit/connections, baggage/facility and refund/reschedule fields. Source date/time/offset strings require calendar and integer parsing; unknown fields remain null. Do not infer baggage allowance or a cancellation refund from a broad airline/property flag. A local inspect operation shows the original source-backed snapshot and retrieval time, without pretending it was re-priced.

Observed canonical handoffs:
- One-way: /en-sg/flight/fullsearch?ap=SIN.CGK&dt=20-11-2026.NA&ps=1.0.0&sc=ECONOMY
- Return: /en-sg/flight/fulltwosearch?ap=SIN.CGK&dt=20-11-2026.27-11-2026&ps=1.0.0&sc=ECONOMY

Use source portal paths, dates and adult.child.infant counts. Source selection hashes are optional; a dated search handoff must not claim a reserved fare. Unsupported currencies/contexts or mismatched source currencies require clear errors, not conversions or fallback labels.

## Hotels, room rates and handoff

Catalog data uses checkInDate/checkOutDate objects (string year/month/day), numOfNights derived from the actual dates, currency, numAdults/numChildren/childAges/numInfants/numRooms, rateTypes PAY_NOW/PAY_AT_PROPERTY, observed ccGuaranteeOptions and sourceType/geoId. filterSortRequestSpec provides skip/top and source POPULARITY ordering. Filter mixed response entries to real hotel inventory cards; expose the source numOfHotels and pagination without treating banners/curations as offers or claiming the bounded page is globally cheapest.

hotelInventorySummary has matchSearchOccupancy, numChargedRooms and finalPrice. Preserve mismatched alternatives explicitly. finalPrice.totalPriceRateDisplay and perRoomPerNightDisplay are distinct source price bases, each with source baseFare/taxes/fees/totalFare/inclusiveFinalPrice/exclusiveFinalPrice and numOfDecimalPoint. Never derive the stay total from a rounded nightly display.

The two-night, two-room, 3-adult/2-child([8,5]) SG/en-SG replay returned Berkeley occupancy_match=true and numChargedRooms=2. Source SGD inclusive stay total was 47181 at decimal scale 2; inclusive nightly per-room display was 11795. Four rounded nightly values give 47180, so preserve the returned 47181. A separately queried USD response returned currency USD with source stay total 36705 and nightly 9176. These are actual source quotes, not an exchange-rate conversion or a delivered-CLI fixture claim.

Rooms body uses hotelId plus the same exact date/party/currency context and source contexts.hotelDetailURL. Response recommendedEntries preserves hotelRoomId/name, base/display occupancy, matchSearchOccupancy, beds, size and amenities. Each hotelRoomInventoryList rate has its own inventory IDs, inventoryName, max occupancy/child capacity, remaining rooms when exposed, mealPlanDisplay/breakfast, isRefundable, rateType, payment/credit-guarantee terms and finalPrice. Keep roomCancellationPolicy's labels, date ranges, fee amounts and details; a source deadline without a zone remains zone-unknown. Property hasFreeCancellationRooms is not evidence that each rate is cancellable.

Normal-source forms prove the current hotel URL spec field order:
`check-in.check-out.nights.rooms.entity-type.entity-ID.entity-name.adults`
Dates are dd-mm-yyyy. Names and comma-separated ages are URL encoded; child ages are a separate childSpec query value. Do not reuse the early minimal legacy query's third field as an adult count.

Observed matching source URLs, stripped of tracking:
- Search: https://www.traveloka.com/en-sg/hotel/search?spec=06-01-2027.08-01-2027.2.2.HOTEL_GEO.10000045.Bangkok.3&childSpec=8%2C5
- Property: https://www.traveloka.com/en-sg/hotel/detail?spec=06-01-2027.08-01-2027.2.2.HOTEL.9000000001714.The+Berkeley+Hotel+Pratunam.3&childSpec=8%2C5

These links match both the visible normal source form and the actual API request with 2 nights, 2 rooms, 3 adults and ages [8,5]. Omit loginPromo, prevSearchId, iuid and other marketing/tracking parameters. Missing names/URLs must be handled explicitly; don't fabricate an SEO slug or silently omit party fields. A booking page may reprice; the CLI only hands off.

## Public snapshots, exact money and errors

Represent amounts without binary floating point. Retain raw integer amount strings, currency and the explicitly supplied decimal scale, and format decimal amounts exactly. Missing scale/unit/tax inclusion remain unknown. Source trip/stay totals, per-passenger/per-room-per-night displays and component taxes/fees are separately labelled. Include retrieved_at, original query, source IDs/URLs, source expiry if known, indicative=true, and a fresh retrieval versus saved-snapshot marker.

Store only normalized public snapshots in SQLite and optional caller-selected JSON files. Match context before comparing, retain complete itinerary/room identities, and never refresh a volatile snapshot silently. Compare commands reject different contexts/currencies/price units; snapshot diff says 'not returned' rather than 'sold out'. Date-grid cells have individual status and timestamps, a bound and fixed occupancy; hotel grid stays have equal lengths. Flight frontier uses authoritative total/stops/known elapsed time and reports unknown dimensions separately. Cancellation pairing requires the same property/room/stay/occupancy/meal/payment/currency and explicit source classifications; unknown/unmatched rates stay unpaired.

Validation must reject malformed calendar dates, nonfuture search dates, checkout/return before departure/check-in, invalid adult/room counts, negative passenger counts and child-age count mismatches. Preserve unsupported-operation, auth-required, access-blocked, upstream/malformed-response, incomplete-search and successful-no-inventory outcomes as distinct structured codes with documented nonzero exit status where applicable. Empty protection 202 is access-blocked. Empty completed flight/hotel inventory is no inventory only when the source confirms successful completion. Invalid-input and failure branches require separately labelled simulated Go tests; all approved live features require delivered-binary evidence.

## Evidence and remaining acceptance

Relevant new proofs: hotel-multiple-room-child-url.json, hotel-multi-night-url-contract.json, standalone-multiple-party-hotel-replay.json (protection 202), standalone-refreshed-multi-night-hotel-replay.json (HTTP 200), standalone-usd-hotel-context-replay.json (HTTP 200 USD). Earlier complete flight, room, airport and corrected hotel resolver proofs remain in discovery. All are discovery prototypes. Required remaining work is approval, generation/build, Go correctness tests, all Printing Press gates, the delivered-binary future-date/source-matched matrix, polish, promotion/archive and closed receipts. No core workflow is complete merely because these prototypes succeeded.
