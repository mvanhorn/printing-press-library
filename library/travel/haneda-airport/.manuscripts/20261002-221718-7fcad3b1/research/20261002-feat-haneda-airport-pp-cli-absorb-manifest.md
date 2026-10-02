# Approved Haneda absorb manifest

Approval basis: user's explicit direct-build authorization and preauthorization for routine scope choices in batch3-build-brief.md. The scope stays on Haneda first-party public flight tools, no account/reservation/payment actions. Exactly one fresh-context MAX reviewer; builder handles implementation and bounded feature selection directly under that user override of the skill's additional brainstorm subagent step.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Domestic/international departure/arrival boards and number/airline/destination/status filtering | Official flightSearch_v2 public UI/script and JSON | haneda-airport-pp-cli flights search | Bounded local slice, source timestamp, bilingual names and full source codeshare groups |
| 2 | Flight detail and codeshare alias lookup | Official flightDetail.js exactMatch contract | haneda-airport-pp-cli flights detail | Stable group ID, source primary separate from unconfirmed operator, canonical detail handoff |
| 3 | Delays/cancellations/diversions and summary | Official status filter and flight_status.json | haneda-airport-pp-cli flights disruptions | Source summary separated from returned service groups and unknown states |
| 4 | Airports/cities with Japanese names and provider search IDs | Public city_list_search JSON | haneda-airport-pp-cli catalog airports | Airport code versus city search value preserved |
| 5 | Airlines and aliases with Japanese names | Public company_list_search JSON | haneda-airport-pp-cli catalog airlines | Airline ICAO/provider versus IATA flight prefixes kept distinct |
| 6 | Published monthly schedules and weekday/period filtering | Official monthly schedule UI/script and feeds | haneda-airport-pp-cli schedule search | Explicit source feed coverage; HND JST event versus other-airport raw times |
| 7 | Source endpoints | Ten verified anonymous JSON paths in enriched capture | (generated endpoint) source | Generated typed raw read-only access retained behind source command group |
| 8 | Scheduled, changed, actual time distinction, staleness and unknowns | Official board field semantics and adjacent-day warning | (behavior in haneda-airport-pp-cli flights search) strict dates and separate fields | Cross-midnight changes use explicit change_date; actual remains unknown when unlabelled |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Timestamped complete scoped snapshot | snapshot save | hand-code | Agent can preserve a full source scope locally while output stays bounded | Save a dated source board snapshot for offline search and subsequent change comparison. |
| 2 | Offline snapshot search | snapshot search | hand-code | Saved groups remain searchable with observation time and stale warning | Query one saved snapshot without fetching the provider. |
| 3 | Material changes between compatible snapshots | snapshot diff | hand-code | Compares status, schedule, changed time, terminal, gates and counters by stable source group identity | Show what changed between two snapshots. Coverage mismatch is an error so filtered scopes are never mistaken for cancellations. |
| 4 | Terminal/counter/gate briefing and handoffs | plan | hand-code | Joins flight status, source facility links and verified terminal floor links into one bounded plan | Locate the flight and its published facilities; source plans do not guarantee connections, gates or seat inventory. |
| 5 | Service-day and midnight rollover inspection | flights rollover | hand-code | Explicit source service/change dates expose adjacent-day services and signed time changes | Return rows whose scheduled or changed date crosses the requested service day or midnight boundary. |

## Candidate audit
Retained the five features above because they serve flight collection, travel agents and terminal planning without new accounts. Killed inferred operating-carrier claims, actual-time inference, seat availability, connection guarantees, notification/account mutations, repeated automatic polling, global airline aggregation and boarding countdowns: the public source cannot establish their promises. Bilingual catalogs and schedule weekday filtering are baseline source parity, not inflated novel features.

## Scope limits
No stubs. Operating flight/carrier and actual time remain null unless explicitly identified by a verified source; source-primary/order is preserved. Blank status is unknown. Arrival exit gates are distinct from boarding gates. Published schedules are not real-time boards, quotes or seats. Dynamic data is fetched per live command and snapshots are explicitly dated. Pagination is a local slice of each refreshed source response.
