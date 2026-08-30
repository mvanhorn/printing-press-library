# Novel-features brainstorm audit trail (Phase 1.5c.5 subagent output)
# Subagent session: ses_fbbe6371bffemMO17kLIMOdIAQ
# Preserved per novel-features-subagent.md output handling rule 3.

## Customer model

**1. Marcus — SFO-based management consultant, flies 2–3×/week.**
- Today: Sunday night and morning-of, opens flighty.com/airports in browser, finds SFO on meltdown map, clicks into detail for status/warnings/weather, keeps a third tab on departures board for his flight number. Redoes manually; nothing scriptable.
- Weekly ritual: Check status → departures board for his flight → decide whether to leave early or rebook.
- Frustration: Scans the departures board by eye for his flight number; no way to ask "which nearby airport is actually fine right now?"

**2. Priya — leisure travel agent managing ~40 active client itineraries.**
- Today: Opens /airports and /airports/tv, filters regions where clients travel, mentally tracks hubs showing MINOR/MAJOR_ISSUES and airlines with high cancellation percentages on detail pages.
- Weekly ritual: Morning sweep of meltdown map → per-hub detail → disruptedAirlines/disruptedRoutes → proactively warn clients or rebook.
- Frustration: Disruption data is per-airport only. Cannot ask "across all my European hubs, which airline is the worst offender today?" without 20+ page opens.

**3. Dan — aviation content creator who covers "meltdown days."**
- Today: Keeps /airports/tv open on second monitor, screenshots map, clicks into each degraded airport to copy warnings and raw METAR.
- Weekly ritual: On disruption days: TV dashboard → per-airport detail → compile the day's story.
- Frustration: No historical record; numbers reset — no way to compare what the state was earlier today vs now.

**4. Ada — automation engineer building a personal LLM travel assistant.**
- Today: Agent has no tool for Flighty's airport intelligence — the only community MCP (CPLX/flighty-mcp-server) requires the installed Flighty app, JWT from app DB, and protobuf against the private app API.
- Weekly ritual: "Is EWR okay for my 6am Thursday flight?" → agent needs one clean --json command.
- Frustration: No auth-free scriptable surface; data is public and SSR-embedded but locked inside self.__next_f.push chunks no tool parses.

## Candidates (pre-cut)

13 candidates generated (see session transcript in manifest context):
1. airports watch — KILLED (scope creep; duplicates framework tail --follow)
2. airports worst — KEPT (9/10)
3. airports compare — KEPT (7/10)
4. airports weather — KILLED (reimplementation subset of show --select)
5. airports route — KEPT (7/10)
6. airports airline — KEPT (8/10)
7. airports find-flight — KEPT (9/10)
8. airports nearby — KEPT (7/10)
9. airports diff — KEPT (6/10)
10. airports belts — KILLED (thin renaming of secondaryCorner)
11. airports pulse — KILLED (marginal sibling of list --status --compact + worst)
12. airports leave-by — KILLED (LLM/personal-context dependency)
13. airports alerts — KILLED (subsumed by airports diff)

## Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| airports watch | Scope creep (persistent monitor); duplicates framework tail --resource --follow --interval | airports find-flight |
| airports weather | Reimplementation: one-field subset of airports show; --select weather covers it | airports compare |
| airports belts | Thin renaming: one secondaryCorner field from a board arrivals already prints | airports find-flight |
| airports pulse | Marginal sibling: totals by status already emitted by list --status --compact | airports worst |
| airports leave-by | LLM/personal-context dependency: home location + drive time absent from every surface | airports find-flight |
| airports alerts | Subsumed: new-warning reporting is a strict subset of airports diff | airports diff |

All 7 survivors score ≥ 5/10. Killed-vs-kept: 6 of 13 cut. Every Long Description references only surviving commands.
