# Rakuten Travel scope manifest

Status: approved by user on 2026-09-27. Backend: credential-free public Rakuten Travel HTML over standard HTTP. Official APIs were researched first and remain documented, but no unverified credentialed backend will ship in this scope. No booking or account mutations. Root Astra owns planning/review/acceptance; Sol max owns concrete implementation/tests, at most two concurrent workers.

## Customer model

An agent is helping a traveler find suitable Japan accommodation and compare actual rooms for explicit dates and party. It needs compact candidates, an inspectable offer identity, comparable amounts and an honest booking handoff. It must explain unknowns without mistaking a static catalog entry for bookable inventory. User's supplied use cases and current public-page probes are the evidence.

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| A1 | Area discovery | Rakuten public area hierarchy; MCP area resources | rakuten-travel-pp-cli areas list | Bounded area names/source path IDs; parent browsing; Japanese names and literal source codes |
| A2 | Hotel search | Public keyword and area pages; mrslbt MCP | rakuten-travel-pp-cli hotels search | Japanese/English literal queries or explicit area; hotel IDs, ratings, URLs; page/limit/selection |
| A3 | Property details | Public _std.html facility page; official detail API parity | rakuten-travel-pp-cli hotels show | Amenities, access, parking, source ratings, notes and property policy; unknowns explicit |
| A4 | Date/party inventory | Public dated plan page; vacant API and travel MCP workflows | rakuten-travel-pp-cli offers search | Actual available room/plan tuples only; explicit rooms, adults per room and six child categories; meals and source quote basis |
| A5 | Focused offer inspection | Public plan/room DOM IDs and booking form | rakuten-travel-pp-cli offers show | Exact hotel/plan/room tuple for a date/party query; full plan description/policy where reliably associated; canonical dated source page |
| A6 | Bounded pagination/output | Rakuten travel skill page bounds; user's quality brief | (behavior in rakuten-travel-pp-cli offers search) JSON, --limit, --page, --offset, --select; next-page/offset metadata and truncation | Source-page and within-page boundaries stay explicit; compact JSON default; stderr diagnostics |
| A7 | Lightweight transport and caching | User quality brief; public HTTP replay | (behavior in rakuten-travel-pp-cli hotels search) serial paced requests, bounded timeout/body/retries, metadata cache | Inventory fresh by default; --refresh; opt-in inventory cache with timestamp; per-run request/byte/latency statistics |
| A8 | Source attribution and booking handoff | Rakuten canonical property and room-anchor URLs | (behavior in rakuten-travel-pp-cli offers show) return source URL, hotel_id, plan_id, room_id and observed_at | Browser opens to exact dated offer context; no booking POST, no invented reservation GET |

## Focused workflow improvements

All five are hand-code, implemented within the focused command families above plus compare. They are not claims of industry-first invention. Scores rank relevance to this user (10 maximum).

| # | Feature | Command | Buildability | Why it matters here | Score | Long Description |
|---|---|---|---|---|---|---|
| N1 | Date alternative matrix | compare | hand-code | Max 9 explicit hotel/date cells with equal nights and party, each retaining availability/error/coverage status | 10 | Compare a bounded set of hotel IDs and check-in dates; use hotels search to resolve properties first. |
| N2 | Whole-stay price evidence | offers search | hand-code | Source 2-night quote differed from twice its first-night quote; preserve JPY whole-stay per-room amount and separate per-person amount | 10 | none |
| N3 | Child-aware uniform room party | offers search | hand-code | Six source child categories and per-room occupancy labels verified; reject unequal allocations rather than silently reinterpret them | 10 | none |
| N4 | Policy and fee inspection | offers show | hand-code | Observed property excludes accommodation tax despite tax-inclusive offer label; distinguish consumption tax, other taxes and property/plan policies | 10 | Inspect a shortlisted offer with its property notes before recommending booking. |
| N5 | Identity-preserving handoff | offers show | hand-code | Hotel/plan/room tuple survives selection; dated source URL plus observed room anchor avoids accidental offer substitution | 9 | none |

## Candidate cuts

Ten candidates were considered. The five retained above serve the user's core task. Removed: blanket bulk sync/offline inventory (freshness/requests), watch/alert daemon (scope expansion), recommendation ranking API (officially stale), automatic translation/geocoding (unverified and extra integration), coupon/member discount optimization (eligibility and total-price ambiguity). No stubs proposed.

The user explicitly assigns all architecture/planning to root and disallows coordination children; root performed this synthesis directly rather than spawning the skill's usual novel-feature planner. The user explicitly requested focused Travel-only read-only scope; generic all-service absorption and mandatory bulk SQLite workflows are excluded.

## Exact boundaries

- Public backend only. No credentials needed for proven source routes. Keep official API findings and registration instructions in research/docs; do not ship pretend API support.
- Areas use stable source website path IDs (e.g. tokyo/E), not guessed official API codes. Search by explicit source area or literal keyword. English terms can yield a different subset; no promise of translation.
- Explicit YYYY-MM-DD dates. Uniform party per room: rooms, adults-per-room and six child counts per room. Unequal allocations return unsupported_query. Validate bounds against source controls before accepting them.
- Offer identity: hotel_id + plan_id + room_id, with date/party context. IDs remain strings. Search defaults to a single source page and bounded output; paging/offset explicit. No silently truncated claim of completeness.
- Price: source amount only, integer JPY, per-room whole-stay when labelled; separate per-person whole-stay amount when explicitly shown. Never multiply per-night values. No total across multiple rooms unless explicitly supplied. Consumption-tax inclusion does not imply accommodation tax or optional fees included. Coupon/member-point promotional prices are not baseline quotes.
- Property policies are labelled property-level. Plan cancellation steps require proven correspondence; otherwise null with source link. Notes and policy caveats are retained.
- Coordinates remain null where public-source datum/units are unverified. No guessed WGS84 conversion and no radius search in this delivery. Deterministic tests cover coordinate unit/datum rejection and any supported conversion.
- Compare explicit hotel IDs × check-in dates, same nights and party, maximum 9 cells and a hard request budget. Hotels can be selected from nearby area searches; automatic geographic area expansion is excluded.
- Default live inventory uncached; optional cache TTL capped at 60 seconds and clearly labelled. Metadata TTL 24 hours, capped cache size/entries, refresh flag. Shared configuration unchanged.
- Default one outbound request at a time, at least 1 second between requests, at most two retries for transient conditions with Retry-After and deadline bounds. Body size bounded. Challenge/parse/mismatched-query pages are errors, not empty inventory.
- No account access, reservation POST, payment, cancellation, affiliate rewriting, other providers, permanent browser, or published/shared tool installation.

## Acceptance

1. Every feature above implemented, help paths work, build/test/vet and Printing Press shipcheck pass; no mock-only promotion.
2. Deterministic tests cover one/multi-night source totals, per-person/per-room units, tax/fee unknowns, child counts/per-room occupancy, malformed query echoes, coordinate contracts, source and within-page pagination, no inventory versus auth/challenge/throttle/parse failure, and identity matching.
3. Live read-only E2E covers Japanese and English property lookup, property details, actual dated offers with a child category and one/two rooms, next-page traversal, explicit no-results case, and bounded date comparison. Live inventory may change; tests verify semantic consistency, not fixed prices.
4. Measure output bytes, request count, latency and peak RSS for representative cold/cache-hit metadata and live/explicit-cache inventory commands; record environment and dates, distinguish fixture benchmark from live network measurement.
5. Concise README/help and agent skill authored using writing-for-agents. Document registration requirements for official APIs, public backend fragility, coverage/price/occupancy limitations and working commands.
6. Promote only after acceptance gates; archive manuscripts and leave a standalone buildable checkout at workspace root. No public publishing and no shared configuration change.
