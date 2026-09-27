# Ikyu accommodation scope v1 — approved

## Product and verified access
A local Go CLI for anonymous Ikyu Japan accommodation discovery and exact room-plan inspection. Public consumer discovery and optional membership are free. No API key is needed for verified routes. This is an unofficial website integration; member-only inventory and account coupon eligibility are not covered. Ordinary Go HTTP and anonymous GraphQL replay work. Browser discovery is complete and is not a runtime dependency.

Sources: access-semantics.md, live-contract.md, browser-sniff-report.md and sanitized replay metadata. Searches of GitHub, npm, PyPI, MCP directories and the official Claude plugin catalog found no maintained Ikyu accommodation CLI/SDK to absorb. Ikyu's current site is the source of the five core capabilities. Restaurant/spa integrations and unrelated projects did not contribute features.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Resolve destinations | Public Ikyu destination names/paths | ikyu-pp-cli stay destinations | Japanese names and source paths; explicit supported aliases, bounded results. |
| 2 | Dated destination discovery | Public destination SSR; date/party variables | ikyu-pp-cli stay search | Date, adult and six child-category inputs, room count, budget and verified preferences; explicit pagination and filter coverage. |
| 3 | Property and review categories | AccommodationIkyu SSR/GraphQL | ikyu-pp-cli stay property | Original name, canonical URL, property facilities and source review category scores. |
| 4 | Available rooms and plan summaries | PlansAndRooms anonymous replay | ikyu-pp-cli stay rooms | Source room size, bedding, views, baths, meal/price summaries and source IDs; lazy detailed plans. |
| 5 | Exact room-plan conditions and booking link | RoomPlanDetailAlt/Amount/Inventory | ikyu-pp-cli stay offer | Full source cancellation, meals, occupancy and price scenarios, canonical public link with stay inputs. |

## Transcendence (workflow refinements)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Offer equivalence | stay compare | 10/10 | hand-code | Fetch at most five explicit property:room:plan selections and compare exact known terms; return differences and unknowns. | User brief; source cancellation and occupancy fields | Compare exact offers for one stay; use stay dates for alternative dates. |
| 2 | Date alternatives | stay dates | 9/10 | hand-code | Check one exact property/room/plan for at most seven explicit check-in dates and fixed nights/party. | User brief; verified dated query inputs | Inspect alternative dates for one room and plan; use stay compare for a shortlist on one stay. |
| 3 | Room fit | stay rooms | 9/10 | hand-code | Apply supported room preferences using room-level evidence, report unknowns and coverage. | Room attribute18 outdoor bath versus16 hot-spring outdoor bath | none |
| 4 | Price explanation | stay offer | 10/10 | hand-code | Expose original/source total, conditional payable, points earned/applied, coupons and eligibility without guessed rounding. | Official FAQ192/872/837; live amount fields | none |
| 5 | Shortlist readiness | stay search | 8/10 | hand-code | Return compact property summaries with total/next-page/missing-detail/freshness indicators, without room-detail fan-out. | Source totalCount and preview truncation; user efficiency brief | none |

All five refinements require Go implementation after generation; zero are assumed auto-emitted. Three refine core commands; only compare and dates add workflow commands. There are no planned stubs.

## Input, output and runtime contract
- Seven stay commands above; all read-only against Ikyu. Standard framework health/help/version support may remain, but headline help and skill focus on stays. No reservation, restaurant, spa, overseas, account or coupon-claim command.
- Separate Property, Room, Plan and dated Offer. Keep Japanese source text and stable string IDs with leading zeros. Missing values are JSON null or explicit unknown, never false/zero by inference.
- Compact JSON by default. Bounded limit and page/offset; field projection via --select (with --fields alias if practical). --agent remains available for Printing Press examples. Diagnostics stderr; errors have stable nonzero exit codes.
- Destinations use source names/IDs/paths and documented aliases; an unsupported ambiguous destination is a validation error, not a guessed area. Child categories follow Ikyu A–F; do not pretend these are universal age bands. Validate all inputs and verify echoed stay/occupancy to catch source normalization to defaults.
- Source-verified filters only. Budget refers to a documented JPY total/points scenario. If a preference/budget filter is applied locally to one page, report that coverage and the upstream continuation; never imply exhaustive matches.
- Rooms: size, bed/view source descriptions and independent private/shared, outdoor/semi-outdoor and hot-spring attributes. Room-level proof controls room claims. Preserve original attribute labels. No inferred spa booking functionality.
- Prices: integer JPY with explicit stay/per-room/per-person/source units. Preserve source headline/base/display amounts and points/coupon fields. Show conditional quote assumptions; checkout-confirmed payable is null where final taxes or eligibility remain unknown. Undated from-rates never count as dated availability.
- Offer equivalence requires all comparison dimensions known and equal: stay, full party and room count, property+room identity, meals, cancellation, payment/price basis and relevant eligibility. Different properties/rooms/dates are alternatives with explicit differences, not an apples-to-apples savings claim. Two missing terms do not establish equivalence.
- Default10/max50 returned results; at most5 selected offers; at most7 explicit check-ins; concurrency≤2; retries≤2 for transient reads; max20 request attempts per invocation; timeout15s/request and120s/command; response body≤8MiB. These are product limits, not claimed upstream quotas.
- Lazy detail. Prefer verified focused GraphQL reads; destination SSR fallback is acceptable. Never keep a browser running, crawl the whole site, or retry access-denied responses with rotating identities.
- Bounded local public cache, key includes all request inputs/schema version. Availability TTL5min, static property/destination TTL24h; --refresh bypasses cache; expose fetched_at, cache age and freshness. Stale return requires explicit opt-in. Bound cache to256 entries/128MiB.
- Partial shortlist/date failures preserve successful rows with item-level errors and partial=true; total failure is a nonzero error. No empty-success fallback for transport/schema failures.

## Acceptance and delivery
1. Standalone go build and fresh tests pass in staging, then after local promotion/workspace delivery.
2. Deterministic tests cover point amounts, equivalent/incompatible/unknown offers, child occupancy and source defaulting, room baths, partial errors, pagination/fields and cache/bounds.
3. Live anonymous E2E covers a hotel and ryokan, dated rooms and exact offers, displayed amounts and source cancellation/meal conditions; canonical links preserve stay and party. Invalid/empty cases remain distinct.
4. Measure cold and warm representative search/property/rooms/offer/compare/date workflows: output bytes, requests, response bytes, latency and peak RSS. Expect zero network on valid warm cache. Publish measured values rather than guessed performance claims.
5. Printing Press shipcheck reaches ship; required late reviews/live dogfood gates pass; receipts close through promotion. Generated artifacts use staging first, then promote locally. Deliver verified source and executable/build command in requested workspace; preserve all prior work and shared tool configuration.
6. Concise README, help and a local agent skill follow Writing for Agents. Include command choice, price/bath pitfalls, bounds, access, freshness and actual limitations; keep deep source contracts in references.

## Explicit exclusions and risks
Personalized member rates, coupon acquisition, booking/payment/cancellation mutations, restaurants, spa booking, other OTAs and broad crawls. Undocumented GraphQL/SSR may change or deny requests; fail visibly and keep canonical handoff links. Final availability and payable amounts are confirmed on Ikyu. No public API/commercial authorization claim is made.

User approved full seven-command scope via the scope gate on 2026-09-27T14:34:09.616484+00:00.
