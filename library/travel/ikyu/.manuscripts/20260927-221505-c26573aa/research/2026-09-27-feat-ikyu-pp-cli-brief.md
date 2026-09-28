# Ikyu accommodation CLI brief

## API identity and user vision
Ikyu Japan accommodation only: help an agent discover exceptional hotels and ryokan, then inspect the exact rooms and plans worth booking. Read-only search, detail, comparison and canonical booking handoff. Restaurant and spa products are future scope. Preserve Japanese names and source IDs. Separate property, room and plan; preserve unknowns and source freshness.

## Access and reachability
- https://www.ikyu.com/ and https://www.ikyu.com/00000600/ return anonymous HTTP 200 HTML with structured `__NUXT_DATA__`; verified 2026-09-27. The initial sandbox DNS failure disappeared with authorized network access.
- Homepage is 1,237,422 bytes. Its Nuxt payload includes operation names/variables and property prices, source IDs, Japanese names, and rating summaries. Prefer replayable JSON if browser discovery verifies it; otherwise lazily extract structured SSR over ordinary HTTP.
- No public accommodation developer contract, API price schedule, maintained Ikyu SDK or CLI has been located by the initial web searches. This is unofficial website integration, not a claimed public API.
- Anonymous discovery requires no key or paid API subscription in the tested path. Member eligibility, member-only offers and account coupons must remain explicit; do not imply anonymous quotes are universal checkout totals. Access/semantic findings: access-semantics.md when complete.

## Users and pain points
The user is an agent planning high-quality Japan stays. Property headline prices hide room and meal differences; room outdoor baths do not necessarily use hot-spring water; points-adjusted banners are conditional prices. A useful CLI makes these distinctions auditable without loading all room/plan details for every search result.

## Top workflows
1. Search a destination and dated party with supported budget/preferences; shortlist stable property IDs.
2. Inspect property details and source review category scores, then room sizes, beds, views and bath attributes.
3. Fetch selected room-plan offers and cancellation conditions before comparing.
4. Compare a bounded shortlist; label differences in dates, occupancy, room, meals and cancellation instead of declaring unlike offers cheaper.
5. Scan a bounded set of alternative dates for one property and return canonical booking handoffs with quote timestamps.

## Table stakes and adjacent tools
Ikyu's own site is the primary capability reference: destination/date/party search, themed filters, reviews, room/plan display, points mode, and canonical offer pages. Adjacent accommodation tools emphasize destination/date search, filterable rates and booking links. Broad travel coverage and automatic booking do not serve this scoped product.

## Data layer and model
Property, Room, Plan and DatedOffer are distinct entities. Stable source IDs and canonical URLs anchor joins. Cache anonymous responses by all normalized request inputs, with acquisition time, freshness, explicit refresh and bounded TTL. Availability is an observation, never a durable inventory claim. Store zero-valued facts separately from missing fields.

## Product thesis
Name: ikyu-pp-cli. Small, agent-friendly, source-grounded accommodation research with honest pricing and comparison constraints. Standard output is bounded compact JSON with pagination and field selection; diagnostics go to stderr.

## Build priorities
Verified query parameters and lazy details first; then price and bath semantics; then bounded comparison/date workflows, deterministic regression coverage, live hotel/ryokan proof, and request/output/latency/RSS measurements. The scope gate selects only workflows supported by live discovery. Build in Printing Press staging, pass all receipt gates, promote locally and deliver a standalone buildable workspace. Shared tool configuration stays unchanged.
