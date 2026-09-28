# Data semantics

## Dates

Inputs name a calendar date and Asia/Tokyo time (or an explicit offset converted to that zone). The website's calendar links provide whole-route UTC timestamps, including next-day journeys. Leg times must be placed in their ordered route context and checked against that span. Preserve source time text and label inferred leg dates. A late-night query may first depart after midnight on the next date. Calendar spans may include seconds while displayed leg clocks have minute precision; preserve that precision difference when checking chronology. `duration_seconds` is exact for the source span; `duration_minutes` is its whole-minute floor. Duration comparisons and caps use seconds when available. Departure/arrival bounds use exact calendar timestamps.

## Fares

Preserve the website's displayed cash total and IC amount separately. Base fares may cover several consecutive legs; a through-fare group is charged once. Seat supplements are alternatives, not amounts to add together. The selected seat option contributes to the displayed total; other options remain optional. Zero is a source-reported amount; `null` is unknown. Passenger assumptions are unknown unless the source explicitly states them. A source taxi estimate remains in `total_jpy` with an estimated-fare basis; it is not a published transit ticket price. Missing road estimates stay null.

A Japan Rail Pass query was observed to select Hikari/Kodama while still displaying JPY 13,320 cash fares. Pass selection therefore means the source considered that pass; it does not turn cash fares into pass-holder out-of-pocket prices. Retain “Covered by”, exclusions and extra-fare text exactly in meaning. The source's Nozomi/Mizuho extra-fare note is consequential. Catalogue membership is source-advertised support; representative live tests do not certify every listed pass or every journey.

## Scope and provenance

NAVITIME returns scheduled transit alternatives and may also include Car/Taxi estimates. Each route identifies its transport kinds and timing basis; a road estimate is not a scheduled service and cannot establish first/last train availability. Service status and seat availability are not verified. Comparisons rank only the alternatives returned for one query; they are not exhaustive global optimization. Displayed amounts can mix estimates with published transit fares, so inspect each row's `fare.basis`. Unknown metrics cannot satisfy a numeric cap or rank as the cheapest/shortest.

Station/place IDs are source IDs. Route IDs are local content identifiers for stored snapshots; source alternative positions are not globally stable IDs. Canonical source URLs, fetch time, cache age and explicit units travel with the result. `source_updated_at: null` means NAVITIME did not provide a verified update timestamp. A cache TTL is a reuse policy, not a promise that the timetable has not changed.

## Failures

Challenges, ignored constraints and malformed source pages are errors. An HTTP 200 alone does not validate a route. No-data results and local filter misses remain distinguishable from transport failures. Source lookup and routing have no verified cursor/page API; result limits do not imply full coverage of all matching places or journeys.
