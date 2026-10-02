# Airport Limousine CLI brief

## API identity
Airport Transport Service public website, https://www.limousinebus.co.jp/en/. Travelers and agents plan Haneda/Narita bus transfers. Anonymous, read-only runtime; no published OpenAPI. Browser discovery captures SvelteKit SSR/data, route identifiers, stop details, timetable service dates, fares, conditions, and real-time travel estimates.

## Reachability risk
Plain curl returns CloudFront 403. Actual Printing Press probe returns browser_http: Surf Chrome TLS fingerprint receives 200 without cookies (305ms), stdlib 403 (41ms). Browser Chrome anonymous pages display structured data and numeric times with unavailable states. Runtime must use the generated Chrome-compatible transport and bounded extraction. Sandbox DNS failures are environment restrictions.

## Top workflows
1. Find exact routes, terminals, stop IDs and both directions with canonical timetable URLs.
2. Query a service date in JST; retain stop-by-stop times, day rollover, adult/child fare assumptions and schedule notices; schedules do not imply seats.
3. Read current versus standard travel duration with source clock and observed timestamp, preserving adjusting/retrieving/unknown states.
4. Compare opposite airport transfers on the same date without assuming interchangeable terminals; return canonical booking handoff.
5. Check baggage and boarding conditions before travel.

## Table stakes and incumbents
The official mobile website is the incumbent: area selection, route lists, bus-stop search, date timetable, current times, guidance and booking links. Broad map/journey planners offer location search and multimodal routes but are outside this provider-only scope. No official SDK/API documentation or wrapper is claimed for this undocumented website; website-mode skips irrelevant wrapper searches.

## Data layer
Provider route/stop IDs and names are high-gravity entities. Live current times must remain snapshots with source timestamps. Cache/local search may serve route/stop discovery with explicit saved-at timestamp; never imply live seat availability. Request counts, output, body bytes, timeouts and retries are bounded.

## User vision
Useful complete public-source scope within the batch target; preserve Japanese names, exact terminals, direction, JST dates and overnight service times, fares and assumptions, fresh source URLs. No reservations, payments or account mutations. Browser-first discovery, actual HTTP replay, one independent gpt-6.1-sol MAX fresh-context reviewer, local promotion and report.

## Product thesis
Name: airport-limousine-pp-cli. Agent-friendly dated airport-bus planning using the operator's public evidence and canonical handoff. Installation is useful because JSON makes exact terminals, source freshness, unknown durations and fare assumptions inspectable without repeating the web flow.

## Build priorities
Route/stop discovery; dated timetable and fares; travel-time comparisons; checked-baggage/boarding terms; airport-transfer comparisons and booking handoff; consequential deterministic parser tests, all-command live read checks and bounded performance evidence.
