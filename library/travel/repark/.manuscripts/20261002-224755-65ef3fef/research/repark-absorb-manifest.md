# Repark approved absorb manifest

User authorized the complete focused scope through the shared batch brief; routine research and scope decisions are preapproved. All rows ship fully; no stubs. User's exactly-one-builder/one-reviewer rule overrides the skill's extra novel brainstorming agent. Same provider only, anonymous discovery and quote simulation.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Named landmark, station and address search | Repark public freeword form | repark-pp-cli parking search | Provider resolution with explicit anchor and bounded outputs |
| 2 | Explicit coordinate discovery | Repark JSON map markers | repark-pp-cli parking nearby | Small radius, straight-line distances, no inferred location |
| 3 | Lot detail by canonical ID | Repark SSR detail page | repark-pp-cli parking detail | Japanese names, source URLs and full source conditions |
| 4 | Occupancy category | Source status enum and native rendered labels | (behavior in repark-pp-cli parking nearby) Preserve source category without claiming exact free spaces | Occupancy independent of fit |
| 5 | Hours, capacity and vehicle limits | Source detail and JSON fields | (behavior in repark-pp-cli parking detail) Preserve hours and limits with units | Explicit absent/unknown fields |
| 6 | Day/night/day-type tariffs and maximums | charges groups and charge_note | (behavior in repark-pp-cli parking detail) Normalize bands and caps conservatively with source text | Overnight/calendar boundaries and repeats retained |
| 7 | Provider price calculator | Public calculator settime form | repark-pp-cli parking quote | Exact JST interval, explicit bay, source estimate only |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Compare full lot facts | parking compare | hand-code | Bounded live detail comparison across named REP IDs without recomputing totals | Compare source rules and declared limits across known lots; estimates require an explicit quote. |
| 2 | Declared fit assessment independent of vacancy | parking nearby | hand-code | Supplied dimensions compare separately to lot limits; bay restrictions remain uncertain | none |
| 3 | Vacancy shortlist | parking nearby | hand-code | Available and crowded source categories can be filtered without equating occupancy to fit | none |
| 4 | Auditable tariff facts | parking detail | hand-code | Raw source conditions accompany parsed day type, window and repeat rules | none |
| 5 | Provenance and resource bounds | parking capabilities | hand-code | Per-command request limits, source freshness semantics and incomplete coverage are explicit | none |

Scores: comparison 8/10, declared-fit 8/10, vacancy filtering 7/10, tariff provenance 8/10, capabilities 6/10. Five hand-coded capabilities, no unsupported features. Quote engine verified 1800JPY for REP0022209 bay1 2026-10-03 08:00..12:00, no totals calculated locally. Provider discounts and final charged amounts unknown.
