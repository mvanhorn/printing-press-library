# Browser discovery

## User Goal Flow
Find a dated airport transfer and its exact terminals, source fares and conditions. Native Chrome anonymous pages were used before implementation.

## Pages & Interactions
Current Travel Time showed numeric estimates, source HH:MM and explicit adjusting/retrieving states. Homepage exposed routes, stops, timetable, guide and canonical inter-airport links. Timetable list supplied area IDs; Haneda-Narita detail showed six terminal columns, 2026-10-02 date and adult 3600/child 1800 JPY. Changing the date input to 2026-10-03 changed URL d=2026-10-03 and timetable. Stop search: clicked Search by stop name, entered Shinjuku, submitted; list contained distinct Shinjuku station and bus-terminal IDs. DOM form reads established POST /en/busstop/list/?/keyword, keyword field. Guide provided baggage and caution URLs.

## Browser configuration
Native Browser Use through mcp__cua_repl, isolated Chrome session and tab 448471666. One approved CDP network observation and one response body read identified SvelteKit __data.json, then ordinary DOM/HTTP only after the user requested fewer debug approvals. No logged-in session or cookies imported. CDP wall wait was unusually long. No resident browser runtime.

## Endpoints
Public GET HTML/SSR pages and SvelteKit data JSON at /en/timetable/list/__data.json, /en/timetable/detail/Haneda-Narita/__data.json?dir=1&d=2026-10-03, /en/busstop/detail/HanedaAirportTerminal3/__data.json and /en/guide/realtime/__data.json. Conditions __data.json is newline JSON; first data record contains content, later timetable chunks are unrelated. POST public stop search requires standard same-origin Origin/Referer headers, with keyword form body.

## Traffic Analysis
SSR HTML, SvelteKit devalue reference pools and JSON. Exact wire keys: dir=1/2, d=YYYY-MM-DD, keyword. Public source fields include Japanese station names, stable IDs, fare adult/child, arrival/departure values and platform labels. Runtime strips timetable seat inventory and reservation flags: schedules do not establish seats. Dates are JST, source current-time clock has no date; report that explicitly.

## Replayability
Plain curl returns CloudFront 403; Press probe returned browser_http. Chrome-compatible HTTP without cookies returned real 200 bodies for homepage, route list, date timetable, stop details, realtime and conditions. Evidence in replay-meta.json, replay-extra-meta.json, contracts-meta.json. Body bytes 3341–195274; requests around 0.3–1.5s. Not an official public API.

## Coverage
Provider-only public route/terminal/stop discovery, dated schedules/fares, current versus standard times, baggage/boarding guidance and booking handoff. No booking, payment, login/account interaction or availability promise. No 429 observed. No proxy envelope.

## Authentication context
Anonymous; no auth session captured. Stored captures contain public response bodies and safe metadata only. No telemetry captured.
