# Implementation contract

Root Astra owns architecture, integration review and acceptance. Sol workers own concrete implementation/test files. The approved scope manifest is authoritative.

## Ownership
- One gpt-6-sol worker at max reasoning owns all staged Go implementation, fixtures, deterministic tests and live/measurement scripts. It proceeds in build-green slices: source/model, commands/workflows, acceptance tests and scripts.
- Root gpt-6-astra owns research synthesis, architecture, docs/skill, receipts, integration review and final acceptance. Code corrections are assigned to the Sol worker.
- A second Sol allocation was refused by the collaboration thread limit after the redundant Astra children were retired. Sequential Sol work preserves the requested model/effort and avoids shared-file conflicts. No further orchestration layer is used.

## Public adapter API
Use package `internal/ikyu`. The worker creates `types.go` first and reports the exact agreed signatures. Command implementation consumes it without defining duplicate domain types.

- `NewClient(Options) (*Client, error)`. Options supports injectable HTTP client/base URL for httptest, cache directory, refresh, explicit stale policy and request bounds.
- `Destinations(ctx, query, limit, offset)` returns bounded source destination entries.
- `Search(ctx, SearchRequest)` returns property summaries and source pagination. SearchRequest includes Destination, Stay, Limit/Offset and supported budget/meal/bath/nonsmoking filters.
- `Property(ctx, propertyID)` returns property facts and category review scores.
- `Rooms(ctx, RoomsRequest)` returns paged rooms with bounded plan summaries. Request includes PropertyID, Stay, Limit/Offset and room preferences.
- `Offer(ctx, OfferRequest)` returns separated property/room/plan/dated-offer fields. OfferRequest includes PropertyID, RoomID, PlanID and Stay.
- `Stats()` returns thread-safe request/byte/cache metrics for one command.
- `ValidateStay(stay, now)` validates exact dates in JST, positive adult count, six nonnegative child categories, room count and bounded nights. Source-echo checks happen before an offer is called dated/available.

Result types use compact JSON envelopes with data, pagination where applicable and freshness metadata. Details have clear source units. Exact field names must be pinned in types.go before the CLI worker proceeds. Common stay inputs: CheckIn/CheckOut strings ISO date, Adults int, Rooms int, Children [6]int. Canonical links include public path and validated stay/party; never emit opaque booking action URLs.

## Command layer
Use `stay` parent so generated framework `search` keeps its own meaning. Commands: destinations, search, property, rooms, offer, compare, dates. Preserve --agent and --select interoperability; implement compact JSON by default, --fields alias, --limit and --offset/page handling. Each public command supports honest --dry-run returning planned read requests with no network/cache writes, for generator narrative verification. --refresh and explicit --allow-stale are coherent across commands.

Comparison/date logic is application code, in separate stay_compare.go / stay_dates.go. Compare takes 2–5 explicit property:room:plan selectors and common Stay. Date alternatives take one selector plus ≤7 explicit check-in dates and fixed nights/party. At most two concurrent reads. Return successful rows and per-selector errors; partial outcomes are visibly partial and all-failed outcomes fail. Compare only within known compatible groups. Unknowns, different property/room IDs, dates, occupancy, meals/cancellation or relevant price/payment eligibility block equivalence. No inferred best/hotel-value score.

## Source strategy
Prefer minimal known GraphQL selections derived from captured read-only queries, with actual response validation and GraphQL errors checked. SSR fallback for destination catalog/search is acceptable when JSON document is unverified. Prune irrelevant fragments, photos, review text and booking actions; do not ship broad raw capture payloads. Preserve exact source financial integers; never recompute points from rates. Use fail-closed schema errors instead of silently empty arrays.

Read-through cache and request control follow manifest bounds. A cached hit remains a timestamped prior observation. Tests inject transport/time rather than depend on personal configuration. Shared tools/configuration stay unchanged.
