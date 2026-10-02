# Airport Limousine absorb manifest

Scope authorized by the batch3 build brief. Public provider scope; no stubs or deferred shipping features. Direct builder implementation and exactly one fresh MAX reviewer override the skill's optional/additional discovery delegation.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Route area discovery and suspension state | Operator route list | airport-limousine-pp-cli routes | Stable IDs, both direction URLs, bounded substring filters |
| 2 | Stop/terminal catalog and search | Operator stop search | airport-limousine-pp-cli stops find | Exact English/Japanese names and IDs, source coverage |
| 3 | Stop boarding details and connected routes | Operator stop detail | airport-limousine-pp-cli stops get | Coordinates, address, maps, direction and route handoff |
| 4 | Dated timetables and fares | Operator dated timetable | airport-limousine-pp-cli timetable | Exact station columns and explicit unknowns |
| 5 | Canonical booking handoff | Operator timetable and reservation URLs | airport-limousine-pp-cli handoff | Read-only URLs, no reservation execution |

## Transcendence
| # | Feature | Command | Buildability | Why useful | Score |
|---|---------|---------|--------------|------------|-------|
| 1 | Terminal-specific dated journey planning with midnight rollover | timetable | hand-code | Agents can select exact origin/destination and get absolute JST times without conflating terminals | 9 |
| 2 | Current versus standard durations preserving source states | travel-times | hand-code | Unknown adjusting/retrieving rows stay unknown; known differences are arithmetic over source estimates | 9 |
| 3 | Both airport-transfer directions on the same date | transfers | hand-code | Compare operator evidence with exact six-terminal schedules and explicit partial failures | 8 |
| 4 | Party fare arithmetic from a selected served pair | fare | hand-code | Adult/child units and totals remain source-anchored with eligibility and price/inventory caveats | 8 |
| 5 | Source-fresh baggage and boarding facts | conditions | hand-code | Bounded useful limits, current notice links, child/seat rules and canonical details | 8 |

## Candidate audit
Traveler personas: airport transfer, first-time arrivals with luggage, parties with children and agent itinerary planning. Candidates cut: seat booking/payment (outside authorization); live browser runtime (unshippable); arrival guarantee (unsupported); airline connection safety scoring (unsupported); exhaustive crawl/offline current times (would hide freshness); multimodal fare comparison (different providers outside scope).

## Approval
All bounded provider scope and these feature choices preauthorized by batch3-build-brief.md. No new purchase, login, source substitution or account action. Framework local learn/search/store available; domain planning performs fresh bounded reads.
