# Sunflower Ferry CLI brief

## Identity and authorized source scope
MOL Sunflower public Kansai–Kyushu ferry planner: https://www.ferry-sunflower.co.jp/en/ and source-linked https://booking.ferry-sunflower.co.jp/. Three routes, six directions: Osaka (Terminal 1)–Beppu, Kobe–Oita, Osaka (Terminal 2)–Shibushi. Preserve Japanese port names and portal line codes 21/22, 11/12, 31/32. Oarai–Tomakomai is a same-operator route with a distinct booking system; out of this focused build unless its interface is verified. Operator-wide legal conditions redirect to https://www.sunflower.co.jp/stipulate/index.html.

## User vision
Useful public-source planning within the approved batch: exact JST service dates, overnight arrivals, fare seasons, cabin/vehicle/passenger categories, genuine source fare simulation, port/checkin/baggage/cancellation facts, canonical booking handoff. No bookings, holds, account operations, or payments. Scope/research/native discovery/build/review/local promotion are preauthorized.

## Top workflows and table stakes
1. Discover the three routes, two Osaka terminals and direction IDs, then inspect published weekday timetables.
2. Query an exact boarding date through the official anonymous fare/availability lookup; return ship, departure and next-day arrival, available cabin fares and observed availability symbols without selecting a cabin.
3. Compare source seasonal-calendar dates, including E daytime cruises whose ordinary overnight timetable does not apply.
4. Simulate one-way travel for adult/child/toddler/infant party counts and foot/car/motorcycle/bicycle categories; preserve total-party fare and source discount/eligibility assumptions.
5. Inspect cabins and occupancy, ports/access/checkin and common conditions, then hand off to canonical booking URL.

The operator anonymous booking portal is the incumbent: it exposes dates, cabin fares, availability symbols, party/vehicle categories, and discount labels. The operator English public route pages expose timetables, cabin capacity/access and seasonal calendar. A focused public search for Sunflower Ferry CLI/wrappers found unrelated Ferry names rather than a matching wrapper/SDK; no matching wrapper issue tracker exists to inspect. Website mode therefore skips SDK/spec catalog search as instructed.

## Reachability and evidence
Native Chrome confirmed official fare page October 2026 calendars and then the anonymous portal flow: GET Reserve0000/IndexEnglish -> POST Reserve0000/Reserve -> Reserve1030 trip form -> POST Reserve1030/MoveNext -> Reserve1020 fare table. Check fares without logging in and Proceed to Check Availability are read-only discovery. Never invoke Reserve1020/MoveNext or select availability link. Tokens/cookies remain only in ephemeral memory. Direct HTTP read-only source pages all returned 200 using normal network access; initial sandbox DNS status 000 is a tool restriction.

Native observed example 2026-10-15, Osaka1→Beppu, one adult on foot: Season A, Web DC(Partcal 5%), SUNFLOWER KURENAI departure 19:05 arrival 06:55; cabin table includes Private bed 14620 JPY and various waitlist/full symbols. These are observed test evidence, not frozen current prices. Arrival rolls into 2026-10-16 +09:00. The fare result does not establish future inventory or hold a cabin/vehicle.

## Data layer
Stable local route/port registry enables offline route search. Dynamic HTML/calendar/quote data is fetched live, optionally retained only as explicitly dated planning snapshots; no persistent cookie/session store. Highest-gravity entities: route/direction/service-day/sailing/room/party/vehicle/fare-band/port. Output includes observed_at, source_urls, coverage, unknowns, units and canonical URLs. No need for a large replicated SQLite store.

## Source contract and pitfalls
Timetables use rowspan/colspan: Shibushi outbound Mon–Sat shares 17:55→08:55 while inbound Friday/Saturday differ. Handle table expansion and JST next-morning rollover. Effective caption says since July 1 2023, not a new validity guarantee. Removed HTML comments include stale 2024–2025 timetable PDF links and must be ignored. Seasonal fullcalendar event arrays are source scripts (cal-beppu.js and cal-kyusyu.js), per-direction, explicit date coverage only; E is daytime cruise, never infer a sailing from ordinary timetable. Intro marketing prices dated July 2025 are historical and excluded. Portal categories: adult junior-high-or-older, child elementary-school, toddler preschool aged >=1, infant <1 year; retain school-stage rule rather than pure age arithmetic. Cars strictly <3/<4/<5/<6m; over-6m/tractor/trailer/series900 require phone. Portal party adults+children max14, one vehicle/one room; special discounts/room charges/group/unaccompanied vehicles require telephone. Actual quote price is returned for input party/category, no invented additive arithmetic. Quote response fare/availability row count may differ by party/date.

## Product thesis and build priorities
sunflower-ferry-pp-cli: a bounded anonymous first-party planner that turns a cumbersome fare/availability form into precise JSON and never crosses into reservation operations.
1. Typed route registry and robust timetable/calendar parsers with honest date coverage.
2. Ephemeral HTTP session replay for anonymous date/party/vehicle fare lookup and sailing schedule.
3. Source cabin occupancy/terminal/access/conditions parsing with explicit uncertainty where English source is incomplete.
4. Agent-native bounded JSON, useful help/README/SKILL and deterministic tests for consequential parsing/date/validation logic.
5. Live read-only matrix on every route/direction, foot/car/child groups, E daytime and unsupported date cases; measured request/output/time/memory bounds; independent MAX review, fixes, all Press gates and atomic local promotion.
