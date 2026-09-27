# TableCheck CLI brief

## User Vision
Build a TableCheck-only, read-only Japan trip planning CLI. Discover restaurants, inspect exact courses/prices/conditions, observe availability for bounded party/date queries and shortlists, and hand off canonical booking URLs. Preserve stable IDs and Japanese names; unknown values are explicit. Search listings are not proof of bookability. No booking, hold, payment, account, or cross-provider integration.

## Access and reachability
- Official API is partner approved, component scoped, with a secret_key in Authorization. Pricing is supplied during application; no verified free public API entitlement. Sources: https://tablecheck.atlassian.net/wiki/spaces/API/pages/44630975 and https://tablecheck.atlassian.net/wiki/spaces/API/pages/44729064 .
- Directory lists restaurants; Availability is separate. Official availability represents a precomputed observation, not a booking guarantee. Source: https://tablecheck.atlassian.net/wiki/spaces/API/pages/44860200/ .
- Public website https://www.tablecheck.com/en/japan returned HTTP 200 without cookies/auth on 2026-09-27. Network requires sandbox escalation; first sandbox DNS failure was an environment restriction, not evidence of site blocking.
- Landing HTML contains React Query hydration; live portal JavaScript exposes venue_type=tc, location/cuisine/budget/search/date/party fields. Actual search/reserve contracts require browser capture and HTTP replay verification before scope approval.
- Risk: unpublished website interfaces may change; broad directory coverage includes restaurants that cannot be booked. Runtime must report failed/unsupported checks separately from no availability.

## Product thesis
Name: tablecheck-pp-cli. Turn a bounded Japan restaurant shortlist into source-grounded booking choices with exact conditions and a user handoff. Compact agent output avoids downloading full detail for every search result.

## Top Workflows (pending verified scope gate)
1. Discover Japan restaurants by place/cuisine/budget; optionally restrict dates and party size.
2. Inspect one venue and separately its courses/conditions.
3. Query one venue for a date/party, preserving course identity and observed status.
4. Check a small venue shortlist over a bounded date range with partial results.
5. Return canonical TableCheck booking URLs with verified prefill parameters.

## Table stakes and ecosystem
TableCheck public UI is the primary incumbent. A commercial Apify TableCheck search scraper advertises structured discovery; it is not a runtime dependency. Searches found an API catalog/spec mirror (api-evangelist/tablecheck), not a mature SDK/CLI. npm/PyPI searches yielded no verified dedicated consumer library. No top wrapper exists to audit issue history. Source examples: https://apify.com/stealth_mode/tablecheck-restaurants-search-scraper and https://github.com/api-evangelist/tablecheck .
Pain points: listings mistaken for availability; course/party restrictions lost; noisy/unbounded multi-date searches.

## Data layer and correctness
Cache bounded public venue/search data and short-lived availability only with observed_at/fetched_at, source, cache hit/age/expiry and refresh. Never archive cookies/tokens. Model venue, course, money/price basis/charges, availability observation, and error separately. Preserve null unknowns, raw source status, Asia/Tokyo dates, Japanese names, and canonical URLs. Instant availability, reservation request, waitlist, sold out, unpublished inventory, and failed checks require distinct source evidence; absence never implies sold out.

## Build priorities and acceptance
Replayable HTTP runtime; compact JSON and field selection; separate summary/detail; default limits and pagination; lazy details; bounded requests/retries/concurrency/date scans. Deterministic status/price/time-zone/partial-failure tests plus live read-only source comparison. Measure cached/uncached output bytes, requests, latency, peak RSS. Local staged generation/promotion only; shared configuration unchanged.

## Execution
Requested gpt-6-astra lead now handles architecture/access review; gpt-6-sol max will own implementation and tests with disjoint files. Root session model identity is not exposed. Printing Press 4.32.5 passed preflight. Skip shared global skill updater per explicit user constraint. Workspace was empty.

## Additional live public probes
Observed 2026-09-27, anonymous HTTP GET, all 200 JSON:
- https://production.tablecheck.com/v2/shop_universes returns Japan ID `57e0b91744aea12988000001`, slug `jp`; Japan is not the string `japan` in this parameter.
- https://production.tablecheck.com/v2/cuisines returns stable `field` keys and translations, 191983 bytes.
- `/v2/shop_search` with that universe, Tokyo coordinates, 3000m radius, `per_page=2`, `venue_type=tc`, `locale=en` returns two venues, `meta.record_count=1395`, `meta.search_after` opaque cursor, `last_page=false`, 10383 bytes. Search defaults use caller geolocation unless explicit geographic inputs are provided. Empty availability with no date query is not a sold-out result.
- `/v2/shops/ricolo-tokyo?locale=en` returns one shop in a `shops` array, including `_id`, names/translations, address/stations, budget strings, currency, time zone, request-policy translations/flags, booking_page_mode, service categories. It does not directly contain course prices. 10616 bytes.
- https://www.tablecheck.com/en/ricolo-tokyo/reserve/landing redirects to `/en/ricolo-tokyo/reserve/message` with 200 HTML. Needs user-flow capture to locate course and availability contracts.
These are direct probes, not a substitute for the pending approved browser capture.

## Users
- Japan trip planner: checks a bounded restaurant shortlist across trip dates, compares menu prices and booking conditions, then completes booking on TableCheck. Their repeated planning ritual is narrowing by neighborhood/cuisine/budget, checking a party against dates, and rechecking before handoff. This persona is the user's explicit brief.
- Agent assisting that traveler: repeatedly discovers candidates, resolves Japanese and English venue identity, fetches only shortlisted details, and reports exact availability evidence and partial failures in compact structured output. Their ritual and constraints come directly from the user's agent-focused quality requirements.

## Research synthesis at scope gate
Official API approval uses a secret key and per-component entitlement; pricing is supplied in application, not publicly fixed. Public consumer endpoints replay anonymously and require no credential in our tested path. This is an undocumented website interface, not partner API entitlement. No service charge for these HTTP reads was observed; dining and reservation charges remain venue/course-specific.

The ecosystem search found nbw/tc-mcp, a proof-of-concept with search, cuisine list, calendar and reservation-link tools. Primary source inspected at src/services/tablecheck.ts and src/utils/url-builder.ts; anonymous fetch confirms endpoint family and cuisine/budget/date/party parameter names. Its issue API returns an empty list, not evidence of reliability. Its old calendar boolean shape differs from current v2 nested is_available shape. Its link helper leads to venue pages, so we use current website canonical booking paths instead. No matching official Claude plugin found. npm results are TableCheck UI/build packages; PyPI has no verified consumer SDK. DeepWiki was unavailable.

Party-specific calendar replay returns UTC slot timestamps indexed by local dates and an explicit Asia/Tokyo time zone. Multi-shop summaries can disagree: Oct1 18:00 was reported in summary while party calendar marked false. Therefore summary availability is discovery evidence only. Course catalog availability_status and conditions do not establish a named-course slot. Do not merge them into a synthetic booking guarantee. Unknown charges remain unknown; preserve fine print even when it conflicts with headline fixed-price fields.

## Orchestration correction
The user clarified root is gpt-6-astra and sole orchestrator. The redundant Astra child was stopped after retaining its API-access findings. Root owns research synthesis, architecture, planning, review and acceptance. Only concrete implementation/tests go directly to gpt-6-sol max. This explicit instruction supersedes the Printing Press novel-features subagent prescription; the three-pass scope exercise is performed by root and preserved in the run artifacts. No new managerial child is spawned.
