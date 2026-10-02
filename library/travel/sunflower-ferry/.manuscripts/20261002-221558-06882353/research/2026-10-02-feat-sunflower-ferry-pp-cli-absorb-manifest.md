# Approved Sunflower Ferry absorb manifest

User preauthorized routine scope/research/category decisions and all read-only build work. No stubs. Exactly one fresh-context MAX reviewer is authorized. The explicit user instruction forbids extra brainstorm/coordinator agents, so the skill novel-features subagent is superseded by direct builder domain judgment. Authenticated/account/reservation/payment operations, Oarai–Tomakomai separate system and roundtrip discounts are outside scope.

### Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Route discovery | Official English navigation and portal line choices | sunflower-ferry-pp-cli routes list | Offline route/terminal/direction IDs and Japanese names |
| 2 | Published timetable | Official English route timetable | sunflower-ferry-pp-cli routes show | Expanded rowspan/weekday rules and source effective caption |
| 3 | Seasonal fare dates | Official cal-beppu/cal-kyusyu scripts | sunflower-ferry-pp-cli calendar | Exact date coverage and E daytime flag |
| 4 | Dated sailing discovery | Anonymous Reserve1020 result | sunflower-ferry-pp-cli sailings | Actual date/ship/time plus exact JST arrival rollover |
| 5 | Party/vehicle cabin fare simulation | Anonymous Reserve1030→1020 | sunflower-ferry-pp-cli quote | Input validation, source totals/discount, observed availability and unknown assumptions |
| 6 | Cabin occupancy and type | Official cabin pages | sunflower-ferry-pp-cli cabins | Compact structured category/occupancy facts; no guessed eligibility |
| 7 | Ports/access/checkin | Official terminal pages and reservation guide | sunflower-ferry-pp-cli ports | Terminal1/Terminal2 separated, addresses/access with source provenance |
| 8 | Cancellation and baggage conditions | English reservation and operator-wide linked carriage conditions | sunflower-ferry-pp-cli conditions | Concise source-backed rules and English day-before ambiguity |
| 9 | Canonical booking handoff | Official Web Reservations link | sunflower-ferry-pp-cli handoff | Validated planning inputs and booking URL without reservation mutation |

### Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Exact overnight itinerary | sailings | hand-code | Turns source boarding-date results into dated JST arrival; handles daytime E cruises without projecting ordinary timetables. | none |
| 2 | Party-sensitive fare matrix | quote | hand-code | Presents every source-eligible cabin fare for the entered party/vehicle without hand-made fare arithmetic. | none |
| 3 | Seasonal date comparison | calendar | hand-code | Filters bounded published date ranges with explicit coverage and special sailing warnings. | none |
| 4 | Cabin occupancy evidence | cabins | hand-code | Extracts source room occupancy so agents can distinguish dormitory capacity from private room occupancy. | none |
| 5 | Terminal-aware port planning | ports | hand-code | Keeps Osaka Terminal1 and Terminal2 distinct with concise source address/access/checkin evidence. | none |
| 6 | Condition-aware handoff | conditions | hand-code | Preserves source cancellation/baggage and quote assumptions with explicit missing English day-before band. | none |

## Boundedness and acceptance
Each public command uses at most 6 requests, quote/sailings use a fresh in-memory anonymous cookie jar and at most 8 requests including redirects; 45 second operation budget, 2MiB body limit, at most 93 calendar days and 40 cabin rows. Dates/party/mode/category validate before transport. No account/header/cookie tokens persisted or emitted. Tests target rowspan, date rollover, calendar direction/coverage, HTML quote error detection, occupied room extraction and invalid groups. Live matrix must include all route directions, car and child groups, E daytime, no-fare outcomes and conditions. Source table response is before Reserve1020/MoveNext; forbidden reservation stages are unreachable.
