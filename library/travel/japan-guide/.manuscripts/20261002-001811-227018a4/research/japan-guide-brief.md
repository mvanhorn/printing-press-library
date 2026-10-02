# Japan Guide CLI brief

## API identity and user vision
Read-only single source: https://www.japan-guide.com/. Public travel editorial HTML, not an official API. Independent travelers and agents need concise planning facts, not reproduced articles. No authentication is needed for supported scope.

## Top workflows and pain points
1. Find destinations by source region, keyword and editorial recommendation.
2. Find attractions within a destination and optionally source interest categories; preserve side-trip destinations and events as separate kinds.
3. Inspect hours, closure days, admission and access with facility scope and exact seasonal/date qualifiers. Hours are source statements, never a computed live opening promise.
4. Discover source itineraries, inspect their bounded stop/day labels and links, and compare a short attraction list.
5. Use compact JSON, pagination, field selection and source timestamps to keep agent context small.
Pain points: tourism article length, stale dates without provenance, conflating rating types, multivenue schedules, and broken Japanese text when ignoring charset.

## Reachability and discovery
Native cua isolated Chrome tab 448471342: /e/e623a.html → clicked Tokyo → /e/e2164.html → clicked Sensoji → /e/e3001.html. Rendered real guide facts, no login/challenge. Direct sandbox curl failed DNS status 000; unsandboxed public fetch returned 200 text/html;charset=shift-jis for directory, Tokyo, Sensoji, interests, itinerary index. Runtime can use standard HTTP and structured HTML. Source mixes legacy charset with some UTF-8 decorations; parse declared encoding and avoid decorative text.

## Table stakes and ecosystem
Focused web searches found no substantive Japan Guide CLI/MCP/SDK wrapper. Source web UI is incumbent: regional directory, destination attractions, interest directory, visit facts, editorial recommendations, itinerary suggestions. Travel recommendation competitors are multi-source; excluded under explicit single-source scope.

## Data layer
Stable identity is canonical Japan Guide path (e3001; e3051_western_tokyo_full); local compact extracted JSON cache only on explicit cache/offline request, never whole articles. Default live read through. No bulk crawler, speculative hours, hotel inventory or bookings.

## Product thesis and build priorities
japan-guide-pp-cli: compact source-linked Japan sightseeing planning. Ship guide destinations, attractions, interests, inspect, itineraries, itinerary, compare. Bounded output and upstream request budgets; explicit partial failures and unknowns. Source summaries are concise factual labels; no full article body is returned or stored.
