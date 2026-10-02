# Repark CLI brief

## API identity and product thesis
Repark publishes Japanese time parking discovery and lot details through same-provider public HTML and a JSON map-marker interface. This is an unofficial read-only CLI over the website, with plain HTTP replay and no resident browser or credentials. The CLI answers where to park and what source rules apply, preserving Japanese names, canonical REP IDs, source URLs and fetch timestamps.

## Evidence and reachability
Parent preflight and this builder's isolated native Chrome tab rendered REP0022209 (上汐４丁目第３), rate bands, repeating caps, capacity and vehicle limits. Native traffic observed GET detail 200. Replayed the exact detail URL with raw fetch-docs: 200 text/html UTF-8; public freeword st=1 word=東京駅 redirects to map lat=35.6812996 lon=139.7670658 and rendered SSR lots. Source form names and inline JavaScript establish the GET markers range grammar. The calculator form has a safe POST func=settime with explicit bay/date/hour/minute fields. Tests are anonymous. A sandbox DNS failure was a tool restriction; approved network fetch succeeded. Native CDP reload unexpectedly took 1,989 seconds, so further source contracts use observed public assets/HTML. No bypass or credentials used.

## Top workflows
1. Search by named landmark, station, address or parking name; report source-resolved location, ambiguity and bounded lot results.
2. Find parking near user-supplied coordinates or a known lot; show source occupancy category separately from declared vehicle fit.
3. Inspect and compare lots with hours, capacity, height/length/width/weight, daytime/nighttime/day-type rates and maximum charge rules.
4. Ask the provider calculator for an exact JST interval and explicit bay only when offered and verified. Preserve the source's estimate and caveats; perform no invented total arithmetic.

## Table stakes and user pain points
Native map discovery is inconvenient for agents, global live state changes, and price caps have complex date/time conditions. Times Parking native access failed in parent preflight; Repark is the assigned primary. Keep source rate applicability and repeating/one-time maximums visible; retain source text where normalization is incomplete. Source availability includes compact and size-restricted bays and excludes motorcycle spaces. Source limits can vary by bay: passing declared limits never guarantees remaining-bay suitability.

## Data layer
Primary entity: lot REP ID; no automatic persistent inventory cache. Optional user-written JSON snapshots are ordinary command output. Occupancy observed_at is fetch time; provider measurement time is unknown unless published. Distances from explicit coordinates are labelled straight-line calculations, not travel routes. Local storage, accounts, reservation and payments are outside scope.

## Build priorities and bounds
Implement search, nearby, detail, compare, quote, capabilities with agent-friendly bounded JSON and explicit unknowns. Request budgets, timeout/body limits, no automatic broad pagination, numeric units (m/t/JPY/minutes), JST calendar validation. Source quotes remain estimates and are never bookings or payments. No inference of current driver location.
