# Approved absorb manifest

The explicit autonomous user brief approves this focused read-only scope. These rows formalize the existing five workflows without a scope change. No bookings, payments or account operations.

## Absorbed features

| Feature | Our Implementation | Source and acceptance |
| --- | --- | --- |
| Route discovery | japan-bus-online-pp-cli routes list | Catalog course IDs, original names, canonical URLs, bounded query/pagination; a mismatching query returns [] |
| Direction and stops | japan-bus-online-pp-cli bus route | Course directions, advertised JPY fares, published schedule, map stops and overnight day offsets; schedule rows are not booking stop IDs |
| Dated services | japan-bus-online-pp-cli bus services | Exact requested JST day, IDs, departure/arrival timestamps, sale window, explicit availability; server date substitution returns no requested-day rows |
| Stop-pair fares and party | japan-bus-online-pp-cli bus quote | Actual route-local stop IDs and selected fares; source child age label, one-way arithmetic, pair capacity, transaction cap, optional cancellation fees |
| Operator conditions | japan-bus-online-pp-cli bus conditions | Source route/operator baggage, boarding and cancellation text with general policy links |

## Transcendence features

| Name | Command | Description | Acceptance |
| --- | --- | --- | --- |
| Dated inventory with overnight dates | bus services | Service-day guard and inventory classification | Preserve both dates and JST 24+ semantics; sold out, not on sale and unknown remain distinct |
| Stop-pair party fare evidence | bus quote | Exact boarding-pair price arithmetic and party uncertainty | 2 adults + 1 child arithmetic from selected fare table; numeric count lower bounds, independent transaction cap, no seat or gender guarantees |
| Schedule and stop identity | bus route | Published schedule/inventory separation and overnight stop normalization | Timetable kind explicit; stable course/direction identity, source display rows and map URLs |

Manifest transcendence rows: 3 planned, all three ship. Fixtures are deterministic logic tests only; live source proofs are separate. No stub rows, deferred features or spec-emitted novel rows.
