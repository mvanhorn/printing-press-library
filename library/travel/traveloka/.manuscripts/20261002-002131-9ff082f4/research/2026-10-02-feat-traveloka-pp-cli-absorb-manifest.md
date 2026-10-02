# Traveloka absorb manifest

Run: 20261002-002131-9ff082f4. Status: approved by the user after the complete 14-feature readout. No generated CLI, implemented-feature claim, or delivered-CLI dogfood result yet.

## Supported operations and access requirements

The shipping scope is the nine core features below plus five grounded comparison features. No stubs are proposed, and no core capability is removed. Normal search commands use direct Go HTTP with explicitly imported, legitimate Traveloka-only anonymous browser cookies and operation request-token profiles. A browser is needed for initial capture or refresh, not as an ordinary command sidecar. Existing browser-use is the available capture backend; the workflow must not install it globally or bypass a challenge. Manual scoped JSON import is also supported. Cookie/header/token values remain in a private session file, never in snapshots, logs, proofs, manifests, or source.

| Operation | Observed source interface | Evidence and remaining acceptance |
|---|---|---|
| City/airport resolution | POST /api/v2/airport/search-nexus | Browser and standalone HTTP 200; source city versus airport IDs and types. Delivered Go validation pending. |
| Hotel destination/property resolution | POST /api/v1/hotel/autocomplete | Browser and standalone HTTP 200 with categorized source city/property matches. The replay requires normal same-origin POST headers (Origin and Sec-Fetch); without them upstream returned 500. Delivered Go acceptance remains required. |
| One-way flights | POST /api/v2/flight/search/initial, /poll, /redirection | Standalone complete HTTP workflow returned 153 candidates and source-confirmed total. |
| Return flights | Same endpoints, both journey indices, selected outbound context | Standalone complete HTTP workflow returned 122 outbound and 117 return candidates with both legs and combined source total. |
| Dated hotel catalog | POST /api/v2/hotel/searchList | Standalone HTTP 200 with actual inventory entries and source total/nightly price bases. |
| Room/rate-plan details | POST /api/v2/hotel/search/rooms | Standalone HTTP 200 with room, meal, occupancy, payment and cancellation fields. |
| Comparison and canonical handoff | Source-backed retrieval snapshots and observed Traveloka URL shapes | Local comparisons preserve context and units. Normal source search and property handoff are verified for one and two nights, one and two rooms, 2/3 adults and one/two children (ages 8 and 5). The extended spec records nights, rooms and adults separately, with explicit childSpec ages. SG/en-SG HTTP hotel responses were verified in SGD and USD; delivered-CLI matrix remains required. |

These are consumer website interfaces, not a vendor-supported public API. They can change. The official partner portal requires partnership credentials and certification; supplier connectivity APIs address a different workflow. Neither replaces the requested consumer flight/hotel scope. HTTP 401/403, empty protection response 202, upstream 500, and successful empty inventory must remain distinct. Expired sessions require an explicit normal-browser refresh; no fabricated tokens, proxy rotation, CAPTCHA solving, or automatic challenge bypass.

## Community features and exclusions

Traveloka's current live contracts are the primary source. Crawl_Traveloka contributed flight/hotel discovery and field coverage; fli and trvl contributed agent-facing date/JSON/comparison/handoff patterns. This scope adds current typed consumer replay, both complete return legs, explicit price bases, room policies, and source-backed local comparisons to the historical crawler's extract-and-save approach. None is evidence that our not-yet-built CLI works.

- Crawl_Traveloka: absorb flight extraction, airline/date/duration/route/price fields, hotel catalog extraction, and saved structured results. Coaches are outside the user's explicit scope. No maintained shopper SDK was found in targeted npm/PyPI/GitHub searches.
- fli: absorb explicit one-way/return search inputs, bounded structured offers and useful itinerary comparison fields. Other travel providers are not added.
- trvl: absorb dated travel research and booking handoff; booking/payment and other providers are excluded by the user.
- Bright Data MCP source inspected at https://raw.githubusercontent.com/brightdata/brightdata-mcp/main/server.js: generic api.brightdata.com request/discover/dataset endpoints and Bearer API_TOKEN; no typed Traveloka consumer contract in the entry point. Its paid proxy/unblocking/CAPTCHA and unrelated generic browser tools are excluded. It contributed no shipping feature, so it is not credited as an alternative.
- api-evangelist Traveloka catalog is partner-contract research, not an operating consumer MCP integration. Historical flight gist is not the current search contract. Supplier APIs, developer project bootstrap packages, OCR/chatbots and other products are outside scope.
- Official Claude plugin directory contained no Traveloka plugin. DeepWiki Crawl_Traveloka was unavailable; no semantic findings are claimed.

### Absorbed (9 core features)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Resolve cities, airports and properties | Traveloka live airport search-nexus and hotel autocomplete; hhtrieu0108/Crawl_Traveloka discovery | traveloka-pp-cli resolve | Source IDs, type/ambiguity, bounded JSON, explicit market/locale/currency |
| 2 | One-way flight search | Traveloka live ONE_WAY initial/poll/prefetch | traveloka-pp-cli flights search | Explicit dates/passengers/cabin, source total/per-passenger price, legs/UTC offsets, policies, bounded results |
| 3 | Return flight search | Traveloka live ROUND_TRIP initial + both journey indices + redirection | (behavior in traveloka-pp-cli flights search) --return-date obtains both legs and authoritative combined price | No guessed fare sums or delta-as-total mistakes |
| 4 | Inspect available flight offers | Traveloka live redirection inventory, segment facilities and restriction fields | traveloka-pp-cli flights inspect | Inspect source-backed quote detail with timestamp and original search context; snapshots labelled as snapshots |
| 5 | Dated hotel catalog search | Traveloka live searchList; hhtrieu0108/Crawl_Traveloka | traveloka-pp-cli hotels search | Required dates, adults/children/ages/rooms, exact source stay and nightly amounts, occupancy match, canonical property links |
| 6 | Room/rate-plan offer inspection | Traveloka live search/rooms | traveloka-pp-cli hotels rooms | Room and rate IDs, meal plan, occupancy fit, cancellation/payment conditions, tax/fee detail, unknowns preserved |
| 7 | Like-for-like offer comparison and booking handoff | Traveloka price basis; fli and trvl primary command docs | traveloka-pp-cli quotes compare | Grouping by context, currency and price unit, no invented conversion or inferred totals, source timestamps and canonical links |
| 8 | Agent help/output/input/errors | User objective; fli/trvl primary CLI docs | (behavior in traveloka-pp-cli flights search) and all core commands | Compact JSON/projections, validation, pagination when exposed, distinct access/upstream/no-inventory/unsupported errors |
| 9 | Reproducible anonymous session setup | Actual Traveloka-only browser capture and HTTP replay | traveloka-pp-cli auth import-session | Private Traveloka-only import plus auth capture bootstrap using an already installed browser tool; normal commands HTTP; no browser sidecar; protection errors remain errors |


### Transcendence (5 hand-written Go features)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|---|
| 1 | Explicit flight date grid | traveloka-pp-cli flights date-grid | hand-code | **8/10 — 3/2/2/1** | This uses Traveloka ONE_WAY/ROUND_TRIP initial search, journey polling and read-only redirection/prefetch to compute a bounded comparison of retrieved authoritative trip totals across explicit date combinations with no external dependencies. | Use this command to compare live flight results across explicit date combinations. Do NOT use this command to rank offers from one existing search; use 'traveloka-pp-cli flights shortlist' instead. |
| 2 | Explicit hotel stay grid | traveloka-pp-cli hotels date-grid | hand-code | **8/10 — 3/2/2/1** | This uses Traveloka hotel searchList and search/rooms to compute a bounded comparison of source stay totals for one property across explicit equal-length stays and fixed occupancy with no external dependencies. | Use this command to compare live offers for a property across explicit stay dates. Do NOT use this command to compare cancellation price differences within one stay; use 'traveloka-pp-cli hotels flexibility' instead. |
| 3 | Flight trade-off frontier | traveloka-pp-cli flights shortlist | hand-code | **8/10 — 3/2/2/1** | This uses local source-backed flight quote snapshots and their itinerary legs to compute a nondominated set over authoritative trip total, stop count and known elapsed time with no external dependencies. | Use this command to identify price, stops and duration trade-offs within an existing matched-context flight search. Do NOT use this command to search alternative dates; use 'traveloka-pp-cli flights date-grid' instead. |
| 4 | Same-room cancellation price difference | traveloka-pp-cli hotels flexibility | hand-code | **7/10 — 3/2/1/1** | This uses local search/rooms snapshots to pair explicit cancellation classifications within identical property, room, occupancy, meal, payment and currency context and compute differences between source stay totals with no external dependencies. | Use this command to compare cancellation price differences among retrieved, comparable rate plans for the same room and stay. Do NOT use this command to compare alternative stay dates; use 'traveloka-pp-cli hotels date-grid' instead. |
| 5 | Quote snapshot changes | traveloka-pp-cli quotes diff | hand-code | **7/10 — 3/2/1/1** | This uses two local, secret-free snapshots from real core flight or hotel retrievals to compute exact-identity price and field changes under identical query context with no external dependencies. | Use this command to compare earlier and later retrieval snapshots with identical search context. Do NOT use this command to rank current flight offers; use 'traveloka-pp-cli flights shortlist' instead, or to compare current same-room cancellation options; use 'traveloka-pp-cli hotels flexibility' instead. |

## Implementation and correctness contract

All five novel features are hand-code; none is spec-emits. The two date grids reuse complete live retrieval and have bounded explicit cells and per-cell outcomes. The three snapshot comparisons use secret-free local snapshots, labelled with their original retrieval timestamps. The brainstorm audit is research/2026-10-02-novel-features-brainstorm.md; inferred personas and unproven usage frequency remain clearly identified there.

- Flight search accumulates incremental polls without erasing prior inventory. Return selection obtains both journeys and calls source read-only prefetch for an authoritative total; return deltas are never summed or presented as whole-trip totals. Bounded candidate coverage is explicit, with no global-cheapest claim.
- Preserve every returned segment's local date/time, source UTC offset, marketing and operating airline, stops/connections, cabin, baggage and refund/reschedule fields. Unknown values remain null/unknown. Source precision and money units are retained without float rounding.
- Hotel catalog filters noninventory display entries, exposes source popularity ordering and offset/top pagination, and preserves occupancy-match indicators. Room inspection keeps each rate's room identity, meal, payment, cancellation and tax/fee fields. Never multiply a teaser nightly price into a guessed stay total.
- Prices expose source trip/stay totals separately from displayed per-passenger/per-room-per-night values, currency, decimal scale, available tax/fee amounts and inclusion, original context and quote time. A freshly fetched quote is still indicative and is not a guaranteed final booking price.
- Compare only compatible query context, occupancy, currency and units. Flight frontier retains ties and reports missing dimensions separately. Cancellation differences pair exact room/occupancy/meal/payment keys and explicit source policy classifications; unmatched/unknown plans stay unpaired. Snapshot diff calls missing offers 'not returned', never 'sold out'.
- Validate calendar dates, ordering, future travel dates, positive rooms/adults and nonnegative child/infant counts; hotel ages must match children. Market/locale/currency are explicit flags and recorded in each query. Do not invent vendor limits.
- Global compact JSON, field projection, help, dry-run validation, bounded limits and typed exit codes are part of the core behavior. Source errors, no inventory, unsupported modes, malformed replies and expired access have distinct machine-readable codes.
- SQLite persists normalized public query/quote snapshots only; secrets and opaque credential-like rate keys are excluded. Local data-source commands honour the generated stale/unsynced hint contract, drain rows before nested queries, and propagate transaction errors. No background price watcher or implicit volatile quote re-sync is promised.
- Session client pins www.traveloka.com and allowlisted read-only endpoint paths, validates scoped cookie domains, and rejects credential-bearing cross-origin redirects. Session import/capture creates mode-0600 files and never prints values. A browser-closed Go replay must pass before promotion.

## Live acceptance matrix planned against the delivered binary

Future dated Go CLI runs must cover SIN-CGK and another flight route, one-way and complete return travel, adults plus differing passenger mixes; Bangkok and Singapore hotel destinations, one- versus multi-night stays, differing rooms/adults and a child with explicit age. Resolve and inspect must run live. Representative prices, legs, room plans and handoff URLs must be matched to the normal source under identical market/currency/dates/party. Each approved grid and local comparison must be exercised using actual retrieved snapshots; sparse/non-pairable results are honest outcomes, with simulated tests covering additional algorithm branches separately.

Invalid inputs, partial/incremental responses, successful empty inventory, protection/expired-session, upstream errors, exact money precision and comparison-context mismatch need meaningful Go tests labelled simulated. No fixture or Python prototype counts as delivered CLI live dogfood. Required Printing Press shipcheck, skill/output review, local review, live matrix, polish, promotion/archive and receipts remain outstanding.

## Concrete runtime contract
Implementation must follow research/traveloka-runtime-contract.md, including the verified hotel URL field order, same-origin resolver headers, exact source totals and explicit session refresh behavior. No scope or stub changes.

Approval: user explicitly replied "Approved" for all 9 core and 5 comparison features; no stubs or scope reductions.
