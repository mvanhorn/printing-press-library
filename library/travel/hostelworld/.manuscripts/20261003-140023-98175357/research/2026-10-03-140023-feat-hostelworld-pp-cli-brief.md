# Hostelworld CLI brief

## API identity and user vision
Anonymous read-only Hostelworld hostel planning for backpacking and solo travel. Japan is the initial example; destinations should remain worldwide wherever the source supports them. Preserve property and destination IDs, canonical URLs, dates, occupancy, currencies, quantities, units, observations and source terms. No bookings, accounts, chats or payments.

## Workflows and priorities
1. Resolve a destination without relying on ambiguous text names.
2. Search properties for explicit check-in/check-out and guests, distinguish a source from-price from dated inventory.
3. Inspect a property: facilities, check-in/check-out, age/group restrictions and tax policy with source provenance.
4. Fetch dated offers: dorm type, bed count/capacity, actual nightly and whole-stay amounts, rate plans, deposit and cancellation deadlines.
5. Compare bounded properties and dates using identical occupancy and money units; offer booking handoff URLs.

## Primary evidence
Native public Japan page https://www.hostelworld.com/hostels/asia/japan/ observed 2026-10-03 lists separate Dorms From and Privates From, 360 properties and 67 cities. This is discovery data, not a quote.
Native public property https://www.hostelworld.com/hostels/p/67481/nui-hostel-and-bar-lounge/ observed 2026-10-03 exposes source rules: 16:00–23:00 check-in, until 11:00 checkout, Taxes Included, all guests over 6 and groups up to 10. Summarize/retain relevant rules, not complete editorial descriptions or traveler reviews.
Prior anonymous GET availability evidence supplied by earlier owner: prod.apigee.hostelworld.com/legacy-hwapi-service/2.2/properties/67481/availability/ with guests=2, num-nights=3, date-start=2026-10-04, show-rate-restrictions=true, application=web. Must reverify before generating.
Official booking terms https://www.hostelworld.com/legal/hostel-terms-and-conditions/ and flexible-booking explanation https://www.hostelworld.com/blog/how-to-save-money-using-hostelworlds-flexible-booking-options/ explain deposits, free-cancellation deadlines and nonrefundable distinctions. Each actual rate must carry its own source terms.

## Competitor parity and pain points
Booking.com has Japan/city hostel discovery with dormitory and private room listings: https://www.booking.com/hostels/country/jp.en-gb.html . Hostelworld's native UI provides bed-level room plans and separate private/dorm marketing prices. Existing generic hotel tools cover ordinary properties. The CLI's value is explicit bed versus room units, date and party scope, cancellation deadline/timezone and payment evidence in bounded structured output. Avoid social-atmosphere conclusions from ratings.
Pain points: from-prices conceal dates/occupancy; cheapest room plans may have different cancellation/payment exposure; a private-room total is incomparable to a dorm per-bed price without quantity and units.

## Reachability and bounds
Undocumented site service; no official API contract claimed. Source requests must be anonymous. Prior sandbox DNS failures were environmental; direct unsandboxed source GET succeeded. Fresh replay is pending. Bound bodies, retries, requests, pagination, latency and output. Changed schema is an explicit source error, not sold-out inventory.

## Data layer and product thesis
Product name hostelworld-pp-cli. Stable destinations and a bounded local shortlist snapshot deserve a store; dated availability always fetched fresh and stamped. Use local search/compare against saved snapshots with explicit observed_at and live versus stale status. Do not save traveler profiles, session state or full reviews. Product thesis: turn Hostelworld bed-level facts into a precise travel decision and canonical booking handoff.


## Observed source contract

# Hostelworld discovery evidence

## User goal flow and pages
Discover Japan destinations and compare dated dorm/private offers. Native Browser Use owned background tab browsed Japan country page, Nui67481, dated4–7Oct2guests room plans, Tokyo city search, Sakura15725 private plans, and destination autocomplete Osaka. No booking/add/chat controls used.

## Contract and replay
Native CDP used for a specific unmet need: the actual public anonymous search request/header names. Public city search and autocomplete send an api-key application header whose value is present in the public inline APIGEE_KEY config; only a boolean equivalence and header names were recorded. It is an anonymous application identifier; no account/session token used. Direct HTTP bootstrap parses only that known quoted literal, never executes JavaScript, and requests no analytics user-id. Identifier stays in memory and is never hardcoded or captured. Direct anonymous metadata/availability work without the header.

| Method | Path | Status | Content type | Access |
|---|---|---|---|---|
| GET | /autocomplete-service/v1/autocomplete/web?text=Osaka&v=control | 200 | JSON array | public anonymous app header |
| GET | /legacy-hwapi-service/2.2/cities/452/properties/ | 200 | JSON object | public anonymous app header |
| GET | /legacy-hwapi-service/2.2/properties/67481/ | 200 | JSON object | public anonymous |
| GET | /legacy-hwapi-service/2.2/properties/67481/availability/ | 200 | JSON object | public anonymous |
| GET | /legacy-hwapi-service/2.2/properties/15725/availability/ | 200 | JSON object | public anonymous |

## Native unit observations
Nui: Basic8BedMixedDorm, prices per bed, tax included. Source JPY16910 per bed/3night stay is distinct from 2-guest estimate33820.
Sakura:4BedPrivateSharedBathroom, sleeps4,2rooms available; native prices per room, entire room must be booked. Source stay44808.48 is whole room; avg14936.16 per room/night. Breakdown3862.80/3862.80/3476.52 is per occupancy slot/night: sum*numberOfGuestsPerRoom4 equals source room stay. Keep both units explicitly; if this relation fails mark nightly basis unknown. Female/mixed dorm types retained without inferring guest eligibility.

## Coverage and protections
Structured HAR contains sanitized actual direct replay samples, informed by native observed flows. It retains no header values, cookies, analytics IDs, images, commissions, personal contributors or reviews. Wrong legacy autocomplete timed out; city API without application header401; the public dated search HTML was a client shell, not inventory. These are not sold-out evidence. Native/current contract replay succeeded; no CAPTCHA/login/clearance required. No rate limit events observed. Runtime is bounded standard HTTP; browser only used for discovery.

## Source samples
See sanitized availability-dorm.json, availability-private.json, property-facts.json, city-search.json and destinations.json. Known-field summaries and terms only; source documents remain authoritative. No authenticated session used.

## Observed first-party pages
- https://www.hostelworld.com/hostels/asia/japan/
- https://www.hostelworld.com/hostels/p/67481/nui-hostel-and-bar-lounge/
- https://www.hostelworld.com/pwa/hosteldetails.php/Nui-Hostel-Bar-Lounge/Tokyo/67481?from=2026-10-04&to=2026-10-07&guests=2
- https://www.hostelworld.com/pwa/s?type=city&id=452&from=2026-10-04&to=2026-10-07&guests=2
- https://www.hostelworld.com/pwa/hosteldetails.php/Sakura-Hostel-Asakusa/Tokyo/15725?from=2026-10-04&to=2026-10-07&guests=2
