# Traveloka CLI research brief

## API Identity
Target https://www.traveloka.com/, consumer flights and hotels only. Product: traveloka-pp-cli, a read-only, agent-friendly dated comparison and booking-handoff CLI.

## User Vision
Resolve cities, airports and properties. Search one-way and return flights by explicit dates and passengers; hotels by dates, rooms, adults and children. Inspect source-exposed offers and policies. Explicit configurable market, locale and currency. Preserve unknowns and price units. No booking, account changes, global installs, publication or PRs. Blocked core features remain required.

## Sources and access evidence
- Consumer homepage: direct HTTP GET 200, redirects to /en-en; raw HTML ~996 KB includes Next.js and AWS WAF challenge script. Chrome /en-sg renders hotel/flight search controls and SGD / EN without login. Homepage deals are indicative marketing data, not evidence of dated inventory.
- https://developer.travelokapartnersnetwork.com/get-started: raw HTTP 200. Official sequence requires formal partnership, unique site credentials, certification, then live environment. Distribution inventory described as hotels; do not substitute it for consumer flight search.
- https://developer.travelokapartnersnetwork.com/api-docs: raw HTTP 200 JS shell; exact contracts require further extraction.
- https://atlas.traveloka.com/developers/docs: supplier-side specification implemented by accommodation providers; not a consumer inventory API.
- https://developer.connect.traveloka.com/: connectivity API pushes property availability/rates/content; wrong direction for shopper search.
- https://gist.github.com/tuhuynh27/7730dfa983c8fee10762eac669a203c7: old flight fetch example suggests /api/v2/flight/search/oneway. Historical candidate only; must capture current traffic and replay before adoption. Never copy embedded credentials.
- https://github.com/hhtrieu0108/Crawl_Traveloka: Selenium crawler for hotels/flights. Public issues query returned []; absence of issues proves no reachability claim. No maintained official shopper SDK found in targeted npm/PyPI search.

## Reachability Risk
High/unresolved for dated inventory; homepage 200 does not prove APIs work. Need anonymous browser network capture and replay. Do not bypass WAF or CAPTCHA. Auth-only account/order features out of scope.

## Top Workflows
1. Resolve a named city/airport/property to source IDs with ambiguity visible.
2. Search future one-way/return flights with passenger and cabin context.
3. Search future hotel stays with complete occupancy and child ages where required.
4. Inspect offer details and compare like-for-like quotes, retaining exact policies and price bases.
5. Hand off using source canonical URLs under matching search context.

## Table Stakes / competitors
Traveloka's own site: dates, party, market/currency, filtering, flight legs, accommodation cards, room policies and booking links. punitarani/fli and MikkoParkkola/trvl offer dated flight CLI/JSON, return flights, sorting and handoff links; use their primary README/CLI docs for comparison only, not inventory substitution. Bright Data markets Traveloka extraction but requires third-party access; no access assumed.

## Pain Points
Marketing rates can mismatch dates/party; per-night versus total price can mislead; access failures can masquerade as empty inventory. Exact policies, provenance and retrieval time must accompany comparisons.

## Data Layer
Primary entities: location, airport, property, flight itinerary/leg/fare, hotel room/rate plan, quote. Only catalog/reference data may be cached routinely; availability quotes need timestamps and original query context. Never relabel cached/indicative rates as fresh. Local quote comparison is useful after live retrieval succeeds.

## Product Thesis
Make Traveloka's actual dated offers inspectable through compact JSON and canonical handoff, without booking. Preserve source meaning instead of deriving imaginary totals or policies.

## Build Priorities
1. Prove replayable current location, flight and hotel search contracts.
2. Normalize minimally, retaining source IDs, legs, timestamps, units, tax status and unknowns.
3. Validate inputs, bound output, distinguish unsupported/auth/no inventory/upstream errors.
4. Live matrix on multiple future routes/destinations and differing parties/stays; cross-check source.

## Setup decisions
Printing Press 4.32.5 and skill 3.0.0 compatible. Go stdlib smoke passed. Global skill updater omitted because explicit user instruction forbids global installations and changes outside authorized Press artifact locations. Existing browser tools are available; no installs performed.

## Users
The explicit task describes three concrete research roles, without claiming a separate interview:
- A trip planner checking fixed future flight dates, return legs and baggage before opening booking pages. Their repeated ritual is route/date/party lookup, price comparison and manual handoff.
- A family/stay planner comparing hotel stays with multiple adults, children and rooms. Their repeated ritual is checking occupancy fit, full stay price and cancellation/payment terms.
- An assistant preparing reproducible Traveloka options for a human. Their repeated ritual is resolving ambiguous locations, retrieving matched-context offers and checking provenance rather than treating marketing prices as inventory.
These roles are inferred from the user-provided workflow and observed Traveloka controls, not fabricated customer testimony.

## Current Replay Evidence
Plain HTTP replay with a temporary Traveloka-only anonymous browser session works. Cookies alone with incorrect x-domain returned 404; the airport resolver requires x-domain flightDiscovery. Full captured headers plus legitimate browser request token returned 200. A stale client search ID returned empty results, while a new UUID (matching the website's client-generated search IDs) retrieved inventory. Fresh full return workflow succeeded using initial, bounded polling for journey 0, polling for journey 1 with an outbound ID/context, and read-only redirection/prefetch to obtain authoritative party totals. Evidence: discovery/standalone-full-return-workflow.json.
Hotel search initially returned 202 with an empty body until the browser's normal session was refreshed; then plain HTTP returned 200 with 15 catalog entries. Room search also returned 200, exposing occupancy, breakfast, cancellation and exact price fields. Evidence: discovery/standalone-hotel-replay.json and standalone-hotel-rooms-replay.json. No challenge was solved or bypassed.

## Runtime and Session Contract
Ordinary commands must be Go HTTP calls. One-time/session-refresh browser capture may use the already-installed browser-use CLI in an isolated temporary session; browser transport must stop after capture, and the session file must remain outside runstate/evidence/library/manuscripts. Source cookies and request tokens are explicitly authorized only for Traveloka-only read-only replay. Missing/expired clearance must be an access error, not no inventory. Setup must document the existing browser-use dependency and explicit private session-file path. No global install, resident browser sidecar, account login, booking, payment or publication.

## Correctness Notes
Flight poll responses are incremental: an empty poll must not erase earlier inventory. Return journey 1 is priced relative to selected outbound; displayed deltas must not be interpreted as the total. Use source redirection totalPrice and displayedPricePerPax and retain both legs and their local UTC offsets. Hotel finalPrice.totalPriceRateDisplay and perRoomPerNightDisplay are distinct source amounts; do not multiply teaser prices into totals. Membership/promo labels do not imply an agent is eligible. Tax inclusion for flights remains unknown unless explicitly exposed. Hotel cancellation timestamps lack an explicit time-zone field and must retain that unknown.

## Resolver replay correction
- Hotel autocomplete returned HTTP 200 after adding the normal same-origin Origin/Sec-Fetch/Accept headers absent from CDP requestWillBeSent. Credential-free proof: discovery/standalone-location-origin-replay.json. Missing Origin previously produced upstream 500; do not misclassify as empty inventory.
- Normal source hotel search with 2 adults, 1 room and one child age 8 created `https://www.traveloka.com/en-sg/hotel/search?spec=06-01-2027.07-01-2027.1.1.HOTEL_GEO.10000045.Bangkok.2&childSpec=8&sort=3`; matching API request explicitly contains numAdults=2, numChildren=1, childAges=[8], numRooms=1.

## Additional source evidence before generation
Normal source form and request were matched for two nights, two rooms, three adults and children [8,5]. Canonical hotel spec fields are check-in/check-out/nights/rooms/entity type/ID/name/adults, with comma-separated childSpec ages. Refreshed legitimate session replay returned hotel HTTP 200 in SGD and separately USD. Source rounded nightly multiplication differs from the returned stay total; preserve source totals. See research/traveloka-runtime-contract.md and the discovery proofs it names. These remain prototypes, not delivered CLI dogfood.
