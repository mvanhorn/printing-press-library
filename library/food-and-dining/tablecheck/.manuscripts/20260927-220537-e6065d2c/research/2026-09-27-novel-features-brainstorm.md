## Customer model
Japan trip planner: repeatedly narrows places by neighborhood/cuisine/budget, inspects courses, checks trip dates for a fixed party, then books on TableCheck. Today they compare browser tabs; the pain is mixing restaurant suggestions with bookable options.
Agent assisting the planner: repeatedly resolves venue identity, checks a short list, rechecks before handing over, and reports partial failures. Today large payloads and ambiguous empty results cause false certainty.
Root performed these three passes under the user's sole-orchestrator correction; no planning subagent was used.

## Candidates (pre-cut)
1. Bounded trip scan (persona): keep; combine per-venue calendar reads over trip dates.
2. Explicit evidence status (persona): keep; preserve unavailable/unknown/failed distinctions.
3. Course cost/conditions detail (service content): keep; preserve decimal prices, price basis and fine print instead of unsupported totals.
4. Visible cached observations and refresh (user brief): keep; cached availability needs explicit age and source.
5. Verified booking handoff (user brief): keep; canonical v1/v2 URLs and validated date/time/party, without a hold.
6. Cheapest-course ranking (local join): kill; charges/price variation prevent reliable ordering.
7. Persistent cancellation watcher (persona): kill; background unbounded traffic outside focused planning.
8. Tabelog quality join (cross-source): kill; user explicitly reserves it for future composite.
9. Automatic booking (persona): kill; read-only scope.
10. AI restaurant recommendations (persona): kill; agent can reason from compact evidence.

## Survivors and kills
### Survivors
| Feature | Command | Score | Buildability | How it works | Evidence | Long Description |
|---|---|---|---|---|---|---|
| Bounded trip scan | availability scan | 10/10 | hand-code | Uses party calendar once per venue, filters local date range, retains partial errors. | User shortlist requirement and calendar live replay | Check a bounded shortlist; use availability check for one venue. |
| Evidence status | availability check | 10/10 | hand-code | Reads explicit is_available and source statuses; keeps unknown and failure distinct. | Broad-summary/calendar discrepancy and user's correctness contract | Check venue-level times; use courses get for course eligibility and conditions. |
| Course cost and conditions | courses get | 10/10 | hand-code | Projects menu price and conditions with exact text and unknown missing charges. | Live19800.0 menu, tax included, percent service and variable fine print | Inspect a named course; use availability check for venue-level times. |
| Visible freshness | availability check | 10/10 | hand-code | Cache envelope retains original fetch time; refresh bypasses local cache only. | User freshness requirement; source summary cache timestamps | none |
| Booking handoff | booking-url | 9/10 | hand-code | Uses venue booking mode and source-documented prefill parameters. | Live v1/v2 links and official Web Booking docs | Produces a URL for human completion; availability check provides a timestamped observation. |
### Killed candidates
| Feature | Reason | Closest survivor |
|---|---|---|
| Cheapest ranking | Incomplete charges and variable course price | Course conditions |
| Watcher | Unbounded/background scope | Trip scan |
| Tabelog join | Explicitly out of scope | Evidence status |
| Automatic booking | Mutating scope | Handoff |
| AI recommendations | External inference unnecessary | Compact evidence |
