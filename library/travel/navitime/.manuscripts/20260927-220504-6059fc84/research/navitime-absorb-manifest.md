# NAVITIME focused absorb manifest

Status: approved by user (2026-09-27). Root authored synthesis/plan per the user's explicit sole-orchestrator correction. No further planning subagent. Two replayable website surfaces underpin the proposal: autocomplete JSON and route-result HTML (including its pass selector). The entry page returned a challenge in the final probe and is not a runtime dependency. Cookie-free Firefox HTTP is verified; paid API is excluded.

## Verified capability basis
Seven live GETs returned 200 with zero retries: arrive-by, first service, last service, JR Pass constraint, overnight station route, POI route, ambiguous Japanese station lookup. Effective shapeParams matched requested constraints. Depart-at and English lookup were already independently replayed. The pass catalogue is source-advertised; representative constraints are live verified, not every listed pass.
- Arrive-by: first Tokyo–Kyoto option arrived at 12:00 for a 12:00 deadline.
- First/last: source modes produced 05:45 and 21:24 departures for the test date; these are current sample outputs, not fixed service promises.
- JR Pass: first service changed to Hikari/Kodama; displayed fare stayed JPY 13,320. Preserve published fares and warnings; do not label the displayed amount pass-holder out-of-pocket cost or zero.
- Overnight: a 23:58 Tokyo–Shinjuku query produced calendar-anchored 00:06–00:20 on the next day.
- POI: goalCode resolved Tokyo Skytree with walking legs and source coordinate/code metadata.
- Ambiguity: 大久保 produced seven station IDs spanning several prefectures.

## Absorbed source capabilities
| # | Feature | Best source | Our Implementation | Added value |
|---|---|---|---|---|
| A1 | Bilingual station/location candidates | Captured NAVITIME autocomplete | (behavior in navitime-pp-cli places search) preserve source candidates and missing fields | Stable refs, ambiguity, compact bounded JSON |
| A2 | Scheduled alternative routes and leg/fare detail | Captured NAVITIME HTML; API-based MCPs are comparison only | (behavior in navitime-pp-cli routes search) use source modes and store all detail from one response | Explicit dates, fare assumptions and fresh source metadata |
| A3 | Source pass catalogue and one-pass route preference | NAVITIME pass list and form script | (behavior in navitime-pp-cli passes list) publish IDs, names and verification status | No inferred all-rail coverage or zero-cost claims |

## Agent workflows (transcendence)
| # | Feature | Command | Score | Buildability | How it works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Ambiguity-safe resolution | places search | 9/10 | hand-code | Normalize the live autocomplete candidates without choosing a same-name station automatically. | 大久保 seven-ID probe; English/Japanese live lookups | none |
| 2 | Dated service-window check | routes search | 9/10 | hand-code | Query depart/arrive/first/last modes and validate effective source parameters. | Successful four-mode probes | none |
| 3 | Fare and overnight audit | routes show | 9/10 | hand-code | Expand a saved route snapshot into source legs, dated times, through-fare groups and optional seat supplements. | Midnight calendar evidence; Nozomi base/seat alternatives | Use a route ID returned by routes search to inspect details without another network request. |
| 4 | Practical alternative comparison | routes compare | 8/10 | hand-code | Compare the returned alternatives by duration, displayed fare, walking and transfers, with optional local caps and explicit unknowns. | Returned alternative metrics; user's practical-itinerary goal | Compares this query's returned alternatives, not every possible journey. |
| 5 | Pass-constrained planning | passes list | 8/10 | hand-code | List source-supported single-pass IDs and apply one via routes search, retaining source warnings and fare basis. | JR Pass changes services while displayed full fare remains | The pass filter is applied by routes search --pass; displayed fares are not automatically pass-holder costs. |

All five agent workflows require handwritten normalization/command work after generation. The generator supplies the HTTP endpoint scaffold and framework. No approved feature is a stub.

## Delivery and quality included
`capabilities` reports verified, advertised, credential-gated and unavailable features. Compact JSON defaults, --select/--fields, bounded results, no invented pagination, separate summaries/details, source IDs/URLs, nulls/units, fetched_at and unknown source_updated_at. Explicit cache TTL/refresh/no-cache, state-free Firefox transport, timeouts and bounded retries. Tests cover consequential parsing; real built-binary E2E covers the accepted modes. Measure output bytes, request count, latency and peak RSS for uncached and cached commands. Concise README/help/agent skill using writing-for-agents. Run all remaining receipt gates and promote before delivering a standalone workspace checkout.

## Exclusions and access
No API credentials/subscription required for the verified website surface. No paid official API adapter, bookings, seat availability, live operational status, premium full-stop/timetable views, pass purchase optimizer, multi-pass anonymous query, resident browser runtime or composite itinerary app. Underlying HTML is undocumented and may change; fail clearly on mismatched source constraints/challenges rather than fabricate data. API access/pricing differences are documented in api-comparison.md.

## Candidate cuts and provenance
Root retained five focused workflows grounded in the user's brief and live source data. Multi-day optimization, broad local syncing, maps, hotels, booking, seat forecasts and other service integrations were cut as outside scope/unverified. GHagui/mcp-navitime-rust and asterism45/mcp-servers provide API-based route-detail baselines; no source code copied. Their RapidAPI timing claims are not adopted as timetable evidence.
