# NAVITIME itinerary CLI research brief

Research date: 2026-09-27. Status: discovery in progress; build scope awaits live website contract and user scope gate.

## User vision and source
Build a focused NAVITIME-only component for Japan itinerary practicality: resolve places/stations without silently selecting ambiguous names, compare scheduled journeys, expose dated legs and fare assumptions, and use only source-supported pass constraints. User has no existing API access/subscription and prioritizes a usable website surface. Deliver locally in the workspace; preserve shared configuration.

## Access and reachability
Primary: https://japantravel.navitime.com/en/area/jp/route/ . Plain curl and stdlib HTTP return 403 HTML. Printing Press probe-reachability succeeded using Chrome-compatible Surf HTTP (200, 491 ms), mode browser_http, no clearance cookie indicated. A page-level success is not yet proof of route-search replayability. Browser discovery is next. Domestic NAVITIME route search also returned plain HTTP 403.
Official docs https://api-sdk.navitime.co.jp/api/specs/ returned 200. API route_transit defaults train_data=average; timetable requires a direct-contract option and is unavailable on API marketplaces. Multilingual output and special_pass also have contract restrictions. The official API is credential-gated; no credentials are available. Exact costs and coverage live in api-comparison.md when completed. Website coverage must be independently verified; API options cannot be assumed to exist on the website.

## Product thesis
Name: navitime-pp-cli. A compact JSON interface to verified NAVITIME itinerary evidence. It should expose enough provenance and uncertainty for an agent to decide whether a proposed journey is practical without pretending scheduled routes prove live operation or seat availability.

## Top workflows for the scope gate
1. Resolve a Japanese or English station/location query to stable source candidates; surface ambiguity before route search.
2. Compare a bounded set of journeys leaving after a dated time or arriving before a deadline, then inspect a selected journey's legs and fares.
3. Check a late-night/first-service travel window with explicit Asia/Tokyo dates where source-supported.
4. Compare pass-constrained alternatives only for passes explicitly supported by the source, retaining exclusions and supplements.
All workflows are proposals until website discovery verifies their inputs and outputs.

## Table stakes and ecosystem
Jorudan official route search exposes departure, arrival, first/last train, ticket/IC fares and reserved/nonreserved/Green options: https://world.jorudan.co.jp/mln/en/ . Research benchmark only; never integrate it.
A small Python wrapper https://github.com/Zeletochoy/navitime is WIP cycling-focused, not evidence of public scheduled transit coverage. Its open issue list has no results; no known-healthy rail wrapper verified. npm/PyPI searches yielded no verified relevant transit package. Similar name Navitia is a separate product and excluded.
Pain points addressed by user requirements: ambiguous station names; hidden fare/supplement assumptions; overnight date ambiguity; unsupported inferences about pass coverage, live status or seats.

## Data layer and efficiency
Highest-gravity entities: source station/place IDs, dated route query, route alternative and legs. Prefer on-demand bounded response caching; no network-wide sync. Detail should reuse a prior response where available. Explicit refresh and TTL; retain fetched_at and source timetable freshness separately. Report absent values as null, units explicitly, canonical source URLs and provenance. Stable source IDs remain strings. Results default bounded; expose supported pagination only. Measure request count, latency, output bytes and peak RSS separately for cache miss/hit.

## Correctness acceptance
No average-time output labeled timetable. No schedule labeled live operation or seat availability. Dates and offsets explicit in Asia/Tokyo; reject ambiguous time input. Preserve fare components, alternatives and optional supplements. Pass membership must be source-explicit, not inferred from operator. Deterministic tests for dates, fares, ambiguity; read-only live journeys for each approved capability. No promotion without Printing Press live acceptance and ship gates.

## Build priorities
First verify replayable route/station contracts. Then present a small scope manifest for approval. Use gpt-6-astra for architecture/review and gpt-6-sol max for disjoint implementation/test ownership. Generate in run-scoped staging and promote only after gates pass.

## Users and repeated decisions
- An itinerary-checking agent resolves place names, checks each day's travel windows, and explains late arrivals or impractical walking/transfer requirements. The user explicitly asked for this workflow.
- A future composite author needs a NAVITIME-only component with stable IDs, small machine-readable output, and provenance, without routing guesses or extra service integrations. This is the user's stated downstream use.

## Verified implementation direction
Use the public Japan Travel website through cookie-free Surf Firefox HTTP. Do not use the paid official API or a resident browser. Root captured dated routes using native Firefox Computer Use, then the Sol worker replayed the request with the stock Firefox HTTP profile. The primary source is real timetable-based website routing, not the marketplace API's average-time mode. Only per-capability live probes can promote a proposed mode to verified status.

## Ecosystem scope comparison
- GHagui/mcp-navitime-rust (README): RapidAPI coordinate routing, rich leg/fare/transfer fields, MCP. It requires a key; its timetable assumptions are not adopted because the official API docs describe marketplace average-time restrictions. https://github.com/GHagui/mcp-navitime-rust
- asterism45/mcp-servers/navitime-transit-server (actual TypeScript source): RAPIDAPI_KEY, route_transit endpoint, start/goal/start_time; useful baseline for structured route output. https://github.com/asterism45/mcp-servers/tree/main/navitime-transit-server
- Zeletochoy/navitime: cycling-focused wrapper; no features absorbed outside this request's itinerary focus.
- Jorudan route planner is a UI benchmark only. No non-NAVITIME integration will be added.
Searches covered public MCP, CLI, plugin, SKILL.md, npm/PyPI, and automation references. No source code was copied. The website's own captured contract is authoritative.

## Root-owned scope synthesis
Per the user's topology correction, root owns intent, synthesis, architecture and planning directly. The installed skill's separate novel-feature planning subagent is superseded by that explicit instruction. No additional coordination child is spawned. Concrete implementation and test ownership remains gpt-6-sol/max.

Candidate cut: retain ambiguity-safe lookup (9/10), dated depart/arrive and first/last search (9/10), leg/fare audit with overnight dates (9/10), bounded alternative comparison (8/10), source-explicit single-pass filtering and exclusions (8/10). Reject multi-day itinerary optimization (future composites), seat availability/booking (unverified and outside scope), paid API adapters (no credentials; wrong default semantics), and hotel/place recommendations beyond source resolution (outside scope).
