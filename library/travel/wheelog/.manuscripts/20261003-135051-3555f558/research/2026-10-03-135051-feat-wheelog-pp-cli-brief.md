# WheeLog CLI Brief

## API Identity
- Source: https://app.wheelog.com/?la=en ; official documentation https://docs.wheelog.app/en/info/
- Users: people planning wheelchair-accessible travel, assistants comparing recorded facility details, and travelers looking for toilets, elevators, ramps and reported barriers.
- Data profile: crowdsourced spot records, category-specific accessibility fields and aggregate accessibility answers. Personal contributors and TrackLogs are excluded.
- Observed public map, dated 2026-10-03: anonymous timeline, keyword/date/category search, canonical `?spotId=` detail links. Public loaded JavaScript identifies form POST `/timeline/custom/user/getWebTimelineList` and `/poi/custom/user/getWebSpotDetail` with `OS_CODE: 3`, `APP_VERSION: 1`; detail also sends `APP_LANGUAGE`. Exact replay and field shapes are pending browser discovery.

## Reachability Risk
- Public Chrome map renders without a login or CAPTCHA. Raw docs capture `/en/info/` HTTP 200; a sandbox-only `wheelog.com` DNS failure was not interpreted as a source block. Public observed script assets fetch HTTP 200 outside the network sandbox.
- Website API is undocumented and may change. Verify semantic result code as well as HTTP status. Preserve empty, unknown, unsupported, unavailable and stale separately.
- No documented public SDK/spec was resolved; wrapper hunting is inapplicable to this website source. Browser discovery owns the request contracts.

## Top Workflows
1. Search recorded accessibility spots by place/keyword, category, and explicit evidence date window.
2. Inspect source fields, source date, category, dimensions/equipment where supplied, and aggregate questions with unknown/conflicting answers preserved.
3. Compare a bounded shortlist against requested facts and explicitly show unmet/unknown requirements.
4. Keep a bounded local shortlist containing normalized public spot facts, canonical links and observed dates for offline lookup.
5. Inspect category definitions so a crowd label is understood in context.

## Table Stakes
- WheeLog categories: Restaurant, Station, Hotel, Other, Attraction, Restroom, Elevator, Parking, Ramp, Barrier. Category membership does not establish personal suitability.
- Wheelmap comparator: keyword/place filtering and explicit unknown/partial accessibility; its public FAQ documents separate toilet and entrance evidence (https://news.wheelmap.org/en/FAQ/). WheeLog's finer equipment facts should remain precise source claims.
- Pain points: coarse labels obscure individual needs; old evidence may no longer hold; missing dimensions should stay unknown rather than become zero or pass a requirement.

## Data Layer
- Highest-gravity entity: a public spot ID with canonical URL, original source name/address, category, recorded/update date, observed time, and typed accessibility facts.
- Small normalized local shortlist/cache only. No credentials, cookies, contributor identifiers/profiles, comments, photos or raw personal travel histories. Scope of offline support will be explicit; no bulk worldwide sync.

## User Vision
- Deliver useful accessible-travel decisions beyond ordinary Wanderlog discovery, initially Japan while keeping natural worldwide keyword coverage.
- Only read-only planning. Do not create posts, reviews, bookings, payments or user account mutations. Never guarantee an accessible route.

## Product Thesis
- Name: wheelog-pp-cli.
- A compact evidence-focused terminal/MCP interface to find and compare detailed crowdsourced accessibility observations, with freshness and missing evidence visible.

## Build Priorities
1. Replay public search/detail surfaces and validate exact category/data contracts.
2. Preserve detailed source facts and dates while removing contributor/profile and travel-history material before persistence or output.
3. Bounded search, inspect and compare workflows plus explicit categories and local shortlist.
4. Consequential parser/domain tests, full required live matrix, one independent fresh-context reviewer, and audited publication gates.

## Users
- A wheelchair traveler checking crowd-recorded toilets, elevators and entrances before an outing; their repeated task is to check equipment questions and stale/missing observations.
- A trip planner assembling a short list of attraction, hotel and station options for a Japan itinerary; their repeated task is to compare a few place IDs against explicit accessibility needs.
- An assistant maintaining that same public-place short list between planning sessions; their repeated task is to preserve factual evidence and notice when fresh source counts or fields change.
These are workflow personas inferred from the official app categories and the user brief, not retained contributor profiles.

## Verified Contract
- Anonymous browser request capture and four direct HTTP replays passed on 2026-10-03. Standard HTTPS transport suffices with browser-observed `OS_CODE`, `APP_VERSION`, `APP_LANGUAGE`, and `X-Requested-With: XMLHttpRequest` headers. No cookie/session import.
- Form POST search supports `word`, `type=spot`, `pagenum`, `isDetail=1`, `startDatetime`, `endDatetime`, `categoryList[]` and `questionList[]`. Source confirms date-window echo and categoryList=toilet; exactly one matching 166345 record for the requested 2026-10-02 JST window. Backend times are UTC; Japanese UI subtracts nine hours.
- Detail content is `content.spot`. Structured questions supply stable question IDs, labels and `totalGood` / `totalBad`; zero/zero is unknown, positive/negative together is conflicting evidence. Each question retains those source counts.
- Spot coordinates are public facility coordinates. Detail timestamps include UTC microseconds; list times are UTC to minute precision. List `updated` can be empty; preserve that as unknown.
- Numeric dimensions are not supplied as typed fields on the verified public web surface. Question labels may describe thresholds/equipment; they are not measured dimensions. Narrative notes stay at the canonical page and are not republished. Profiles, raw comments/photos and personal TrackLogs are excluded before output or persistence.
