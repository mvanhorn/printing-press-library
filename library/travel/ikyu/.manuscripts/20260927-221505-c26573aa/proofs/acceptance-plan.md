# Acceptance plan

Root Astra accepts behavior directly; Sol implements tests and fixes. Full read-only live verification was authorized in the user brief and approved scope.

| Feature | Positive evidence | Negative/bound evidence |
|---|---|---|
| Destinations | source Japanese name/ID/path, supported alias | unknown/ambiguous input, bounded output |
| Search | dated Tokyo IDs and echoed occupancy, pagination | incompatible budget/meal/bath, exact page coverage |
| Property | hotel and ryokan details, source category scores | unknown ID/schema failure, optional null facts |
| Rooms | real room IDs, source size range/bed/bath/view evidence, plan preview count | source offsets, room vs shared bath, unknown facts |
| Offer | exact stable selection, source integer prices, meal/cancellation and public URL | invalid dates/adults, sold out vs error, child/multi-room units |
| Compare | same known terms can be compatible; price/term differences visible | different room/meals/cancellation/date and unknown terms never equivalent |
| Dates | exactly all requested dates, timestamp per observation | capped input, sold out and partial errors retain rows |
| Cache/transport | cold request counts and warm zero network | refresh, stale opt-in, bounds, typed429, cancellation, unrelated files preserved |
| Output | compact parseable JSON and actual field projection | diagnostics stderr; errors not empty success |

Run required Printing Press structural/runtime gates and inspect real output independently. Measure all representative workflows cold and warm with output bytes, request attempts, decoded response bytes, wall latency and maximum RSS; include OS/Go/date/results/cache state. Compare live amounts and policies with an independent fresh public source capture for the identical stay/room/plan. No value in a fixture is described as live evidence.
