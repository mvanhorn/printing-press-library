# NAVITIME API versus Japan Travel: access and capability decision

Checked 2026-09-27. Scope: a useful NAVITIME travel CLI without user credentials or a paid subscription. Research only; no provider implementation, authenticated API calls, purchases, or receipt changes.

## Decision

The official NAVITIME API does **not** meet the credential-free requirement. Its free marketplace plan still requires registration/subscription, and its route search does not include scheduled train times. Scheduled rail/bus routes and multilingual output are paid direct-contract options. A website-only CLI remains a candidate, conditional on the root agent's current anonymous route and lookup replay succeeding. A successful homepage fetch alone is insufficient.

Sources: [API introduction](https://api-sdk.navitime.co.jp/api/specs/), [access/pricing comparison](https://api-sdk.navitime.co.jp/api/specs/description/about_navitime_api.html), [average-time explanation](https://api-sdk.navitime.co.jp/api/specs/tips/averagetime.html).

## Capability/access matrix

| Capability | Official API | Japan Travel website evidence/status |
|---|---|---|
| Credentials | Contract or marketplace registration required. Direct contract uses issued CID and a signature; configured source restrictions may replace signing. | Current anonymous workflow verification belongs to root. No authenticated surface was tested here. |
| Depart-at / arrive-by routes | `start_time` / `goal_time`; basic routing uses average train times. | Older indexed route results show dated departures, arrivals and individual trains. Current replay pending. |
| First / last route | `first_operation` / `last_operation`; timetable only, unavailable through API marketplaces. | Current anonymous functionality pending. |
| Real scheduled rail/bus times | Paid direct timetable options. Average mode uses synthetic waiting, travel and transfer durations, so times can differ from consumer services. | Consumer services are documented as timetable-based. This does not establish live predictions or today's anonymous access. |
| Station departure timetable | `/transport_diagram/segment`: direct timetable/bus option; unavailable through marketplaces. | Older indexed result pages mark timetable links with premium locks. Do not promise free standalone timetables. |
| Japanese station resolution | `/transport_node` and `/transport_node/autocomplete`, separately packaged as NAVITIME Transport on marketplaces. | Current lookup replay pending. |
| English station/place resolution | The two station-query specifications advertise `ja` only and no `lang` parameter. English input is unverified. Route output translation is a separate capability. Spot data is an option or separate RapidAPI product with a different dataset; English POI query behavior was not audited. | Current English and Japanese input lookup must be tested independently. Bilingual result labels alone do not prove lookup works. |
| Fares | Structured ordinary, IC and seat/surcharge fields; see semantics below. | Older indexed results show JPY totals and seat-specific prices. Current response mapping pending. |
| Travel passes | `special_pass` is timetable-only; six documented domestic values, with JAPAN RAIL PASS absent from that list. | Older indexed results show JAPAN RAIL PASS and regional-pass labels. Actual anonymous pass filtering and fare treatment pending. |

Sources: [CID](https://api-sdk.navitime.co.jp/api/specs/description/about_cid.html), [authentication](https://api-sdk.navitime.co.jp/api/specs/description/about_signature.html), [route specification](https://api-sdk.navitime.co.jp/api/specs/api_guide/route_transit.html), [station query](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_node.html), [station autocomplete](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_node-autocomplete.html), [timetable](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_diagram-segment.html), [older indexed Japan Travel route](https://japantravel.navitime.com/en/area/jp/route/result/?goal=00006251&start=00007491).

The indexed website example was crawled in 2025, so it is historical evidence, not confirmation of current access or current schedules. A direct web-tool fetch of the route entry page returned HTTP 403. Root separately reported a successful Chrome-backed HTTP probe but has not yet supplied completed route replay evidence.

## Access and cost

The current official comparison page lists the following **per API product subscription**. These are published NAVITIME comparison-page prices checked today; checkout prices, taxes, and marketplace billing terms were not independently verified.

| Channel / plan | Published monthly price | Request allowance | Rate limit |
|---|---:|---:|---:|
| RapidAPI BASIC / API Hub Trial | Free | 500 | 50/min |
| RapidAPI PRO | USD 200 | 5,000 | 100/min |
| API Hub Basic | JPY 26,000 | 5,000 | 100/min |
| RapidAPI ULTRA | USD 300; USD 0.05 overage | 10,000 | 150/min |
| API Hub Standard | JPY 39,000; JPY 7 overage | 10,000 | 150/min |
| Direct contract | Quote; separate setup charge and paid options | Table starts at a 10,000-request cap | Consult sales |

A route product subscription does not include the separate Transport subscription needed for station lookup. Direct-contract prospects may apply for a 90-day trial; it is not a permanent anonymous free tier. Source: [official comparison](https://api-sdk.navitime.co.jp/api/specs/description/about_navitime_api.html), [trial description](https://api-sdk.navitime.co.jp/api/specs/).

Direct endpoints use `https://{HOST}/{CID}/v1/...`. The issued signing key must remain secret; public docs defer exact request-code/signature generation to material supplied during onboarding. Do not invent signing behavior. Sources: [CID](https://api-sdk.navitime.co.jp/api/specs/description/about_cid.html), [signature](https://api-sdk.navitime.co.jp/api/specs/description/about_signature.html).

## Exact parameters worth preserving

For `/route_transit`: `start` and `goal` accept node IDs, location JSON or `latitude,longitude`. Supply exactly one of `start_time`, `goal_time` (`YYYY-MM-DDThh:mm:ss`), `first_operation`, `last_operation` (`YYYY-MM-DD`). `train_data=average` and `bus_data=none` are defaults; scheduled mode uses `timetable`. `lang` supports `ja,en,ko,zh-CN,zh-TW,th` subject to the multilingual contract. `special_pass` is period-delimited: `youth18_pass_basic`, `youth18_pass_express`, `tokyo_metro_24hour`, `tokyo_one_day_ticket`, `enoshima_kamakura`, `hakone`. Source: [route specification](https://api-sdk.navitime.co.jp/api/specs/api_guide/route_transit.html).

For `/transport_node`: required `word`, optional `type`, `address`, `options=detail`, `offset`, `limit` (1–100). Autocomplete requires a 2–50-character `word`; `word_match=prefix|partial` (default `prefix`) and optional `coord`, `radius`, `link` narrow it. Sources: [query](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_node.html), [autocomplete](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_node-autocomplete.html).

For `/transport_diagram/segment`: required `node`, `link`, and exactly one of `date` or `start_time`. `term` is minutes, default 1440, range 1–2880, effective with `start_time`; `stop_node` filters trains stopping at every specified node. Source: [timetable specification](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_diagram-segment.html).

## Fare interpretation

`summary.move.fare` contains totals by fare category, not one universally payable amount. `unit_0` is ordinary fare; `unit_48` is IC fare. When equal, IC fare can be omitted: absence does not establish that IC is unsupported. Per-section fare continuity and surcharges matter; summing every displayed leg fare can double-count a through fare. Default seat surcharges normally favor unreserved seats, falling back to reserved-only services. Source: [fare calculation guide](https://api-sdk.navitime.co.jp/api/specs/tips/fare_calc.html).

`reference_fare.lowest_total_ticket` and `lowest_total_ic` include default seat surcharges, but exclude `unit_224` airport facility fees. Source: [route specification](https://api-sdk.navitime.co.jp/api/specs/api_guide/route_transit.html).

## Architecture acceptance recommendations

These are implementation recommendations inferred from the evidence, not promises made by NAVITIME:

1. Keep one website adapter with explicit capability flags. Do not add the official API as a fallback for anonymous requests; that changes access requirements and schedule semantics.
2. Accept the website transport only after a fresh anonymous process resolves names, submits a dated route, and parses a populated response. Save sanitized evidence of the effective request and result shape.
3. Prove English and Japanese station lookup, ambiguous names, and at least one place/POI separately. Return choices for ambiguity. Preserve source IDs; do not treat IDs from different surfaces as interchangeable without evidence.
4. Prove depart-at, arrive-by, first and last independently against the website UI. Preserve the effective query mode, service date/timezone and source URL. Do not silently downgrade unavailable modes or synthesize timetables from average durations.
5. Preserve source total, currency, fare basis and seat/surcharge alternatives. Missing price is unknown, not zero. Treat a pass label as applicability information until filtering and covered/out-of-pocket fare behavior are demonstrated.
6. Keep standalone timetables and pass features unavailable until their current anonymous workflow is verified. Return a clear access/unsupported error for premium gates, challenges or permission failures.

## Capture notes and remaining uncertainty

Exact official HTML was captured with `<home>/.agents/skills/printing-press/references/fetch-docs.sh` into its temporary cache. Fresh sandboxed fetches initially failed DNS resolution; the normal approved network escalation returned HTTP 200 for the route, pricing, CID, signature, station query, autocomplete, timetable, average-time and fare-guide pages. No access restriction was bypassed.

Unresolved: current anonymous website request/response contract, English/Japanese/POI lookup behavior, first/last access, pass filtering, premium timetable access, website fare mapping, and current marketplace checkout pricing. Root owns browser discovery and all other run artifacts.
