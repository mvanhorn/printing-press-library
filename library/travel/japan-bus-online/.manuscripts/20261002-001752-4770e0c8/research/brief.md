# Japan Bus Online CLI brief
Source: https://japanbusonline.com/en, investigated 2026-10-02 JST.
User scope: discover routes/stops, dated inventory, fares and party evidence, baggage/boarding/cancellation conditions and canonical booking handoff. Read-only; no reservation, account, or payment mutations.

## Identity and workflows
Japan Bus Online is a multilingual bus ticket aggregator. Travelers discover route directions and timetable stops, check a precise JST service day, select boarding/alighting stops, compare adult/child fares, and complete a booking on the canonical site. Primary entities: course IDs, directions, area IDs, service IDs, route-local stop sequence IDs, fare-plan/car IDs. Keep output live and stateless; ephemeral cookies stay memory-only. No bulk sync or local inventory cache.

## Reachability
Sandbox DNS initially failed (tool environment); network-enabled curl returns HTTP 200. Real isolated Playwright confirmed service selection first. After native-browser steering, iab creation returned unavailable; native Chrome extension isolated tab independently showed 2026-10-10 service 23:25 arriving 2026-10-11 07:05, >5 seats, JPY6100-from; stop fare table adult6300, child3150, max4. Browser-required runtime is unnecessary. Public GET AJAX replay requires X-Requested-With: XMLHttpRequest and an ephemeral CookieJar. Without that header SelectFABN returned HTTP200 empty body; with it full public stops response returned.

## Contract
GET /en/AllRouteList -> CourseSearch links and names. GET /en/CourseSearch/{course} -> directions, area IDs via Detail links, advertised fare range and timetable tables. Route timetable does not prove inventory.
GET /en/Detail/{course}/{direction}/{depArea}/{arrArea}/{YYYYMMDD} -> dated service rows with data-route, data-depdate, data-arrdate, times, availability text and from fare. Detect server-selected date; past URL may silently move forward.
GET /en/DetailAjax/SelectRoute/{course}/{service}/{depdate}/{deptime}/{direction}/{arrdate}/{arrtime}?time=99 -> fare-plan radio values and availability as-of.
GET /en/DetailAjax/SelectFABN/{planRadioValue}/{direction}?time=00 -> actual route-local stop IDs, names and source 24+/25+ times.
GET /en/DetailAjax/GetFareTable/{depStop}/{arrStop}/{direction}?time=88 -> adult/child fare labels, max transaction tickets and capacity display. Quote totals are arithmetic estimates for one way, no group discounts or roundtrip assumptions. Gender/adjacency/seat-map feasibility remains unknown.
GET GetCancelFee URL is derived from the source returned fare-table script; no reservation state is created.
Public search is anonymous. Account history and booking are excluded by user instruction.

## Semantics
Use explicit arrival/departure dates from dated attributes. Stop times >=24 normalize relative to service-day; arrival times below24 anchor to source arrival date. Route-local stop sequence ID is not a globally unique bus stop. Preserve name and map URL. Source language supports English, Traditional/Simplified Chinese, Korean; Japanese names may be absent and stay null rather than translated inventions.
Not Available is unavailable_unknown_reason, never sold_out by inference. Explicit sold out -> sold_out; outside source sales window -> not_on_sale_or_outside_window; no-services -> no_services_reported, not sold out. Party availability is sufficient/insufficient evidence or unknown; it never guarantees seat adjacency.

## Ecosystem and priorities
Focused web searches for Japan Bus Online CLI/MCP/SDK/GitHub/npm/PyPI returned no relevant wrappers. Incumbents are provider browser UI and generic Japan journey planners; named source must remain authoritative. Pain points: printed timetable mistaken for inventory, overnight midnight ambiguity, headline-from fares differ from actual boarding pair.
Product thesis: compact source-grounded bus planning with date/capacity/fare uncertainty explicit. Commands: routes list, bus route, bus services, bus quote, bus conditions. Generated Press framework supplies compact JSON, --select, MCP and doctor. Handwritten provider runtime supplies HTML extraction and sessions.
