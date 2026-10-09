# Novel features brainstorm (subagent a362d5355ee1d0bcf, 2026-10-09)

## Customer model
- Mei: foreign visitor planning one Disney day and one USJ day for her family, trip in 6-8 weeks. Today: EN + JA TDR calendar tabs and USJ park-hour SPA; opens a modal per date/park/ticket; cannot tell "not on sale yet" from "no online sale"; cannot see USJ dated price. Ritual: checks every few days, near 14:00 JST release; compares TDL vs TDS, then USJ hours. Frustration: re-clicking modals and copying results by hand to compare price tiers.
- Kenji: Japan resident, TDS/USJ 4-6 times a year, picks dates by weekday. Today: queue-times pages and official apps; crowd calendars without data; no history he controls; no sample sizes. Ritual: background recording of waits; compare weekdays/hours; on park day check which high-wait rides are below usual. Frustration: no own history by weekday x hour with n; live list does not say whether 60 min is good.
- Sam: runs an LLM itinerary agent calling the user's Japan pp CLIs with --agent. Today: agent guesses availability/hours, cannot cite fetch time, cannot tell unknown from sold out. Ritual: per trip request, check candidate dates for all parks, write date recommendation and ride plan with sources. Frustration: no single machine-readable cross-park answer with meta.sources and explicit unknowns.

## Candidates (pre-cut)
C1 dates (keep) | C2 day (dup of dates) | C3 snapshot (keep) | C4 typical (keep) | C5 order (keep) | C6 sale-opens (field of dates) | C7 watch (poll loop, scope creep) | C8 sellout-log (needs weeks of syncs, no demand) | C9 cheapest (sort of dates) | C10 crowd (park row of typical; forecast risk) | C11 down (subset of order) | C12 trip (invented forecast) | C13 usj-price (store behind waiting room + robot check) | C14 ics (no planning decision).

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability | Persona |
|---|---|---|---|---|---|
| 1 | Cross-park date planner | dates | 10/10 (Fit 3, Pain 3, Feasibility 2, Research 2) | hand-code, data-source live | Mei, Sam |
| 2 | Typical waits from local history | typical | 9/10 (3,2,2,2) | hand-code, data-source local | Kenji, Mei |
| 3 | Wait snapshot recorder | snapshot | 8/10 (3,2,2,1) | hand-code, data-source live | Kenji |
| 4 | Ride order now vs typical | order | 7/10 (3,2,2,1) | hand-code (live + local) | Kenji, Mei, Sam |

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| day | duplicate input surface | dates |
| sale-opens | rule field of dates | dates |
| watch | long-running poll | dates |
| sellout-log | no demand, weeks of syncs | snapshot |
| cheapest | wrapper sort of dates | dates |
| crowd | park row of typical; forecast risk | typical |
| down | subset of order | order |
| trip | invented forecast | dates |
| usj-price | not reachable (queue + robot check) | dates |
| ics | no planning decision | dates |
