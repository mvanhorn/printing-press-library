# Approved focused JR East manifest

## Absorbed
| # | Feature | Best Source | Our Implementation | Added value |
|---|---|---|---|---|
| 1 | Area/service summaries | JR East public web | jr-east-status-pp-cli areas | Explicit observation time and unknown source update |
| 2 | Line discovery | JR East Japanese/English pages | jr-east-status-pp-cli lines | Native IDs, names and group identity |
| 3 | Multiple disruptions | JR East public web | jr-east-status-pp-cli status | Bounded direction/section/cause facts |
| 4 | Official source links | JR East public web | (generated endpoint) sources guide | Relevant canonical handoffs |
| 5 | Planned notices | JR East planned-work source | jr-east-status-pp-cli planned | Keeps raw dates when year is unestablished |
| 6 | Certificate handoffs | JR East certificates | jr-east-status-pp-cli certificates | Published URLs only, route maximum caveats |

## Transcendence
| # | Feature | Command | Buildability | Why useful | Long Description |
|---|---|---|---|---|---|
| 1 | Bilingual line facts | status | hand-code | Reconcile native Japanese IDs with English row labels, keeping multiple notices and source uncertainty. | none |
| 2 | Itinerary line impact | impact | hand-code | Compare selected itinerary lines with per-region source timing and affected sections. | none |
| 3 | Planned closure handoffs | planned | hand-code | Inspect construction evidence without promoting future notices to current service suspension. | none |
| 4 | Published certificate routing | certificates | hand-code | Return actual published route/time-slot links and source coverage limits. | none |
| 5 | Reporting coverage classifier | coverage | hand-code | Explain reporting hours, threshold conflicts, midnight service-day rollover and remaining unknowns. | none |

## Candidate evaluation and scope
Customer model: travelers and agent itinerary planners. Candidate pool: bilingual facts (9), itinerary impact (9), planned notices (8), certificate routing (8), reporting classifier (9), push notifications (killed: external mutation), exact train lateness (killed: source does not establish it), reroute planner (killed: no routing contract), historical incident statistics (killed: no source history), automatic ticket claims (killed: outside authorization). Scores are direct product judgments on a 10-point scale; no uniqueness/superiority guarantee. The user expressly forbade extra agents; no brainstorming subagent was spawned. Five surviving capabilities require hand-written code; none is a stub.

Approval: batch3-build-brief.md preauthorizes focused scope and routine research decisions. No external accounts/notifications/tickets. No expansion into exact train locations or full timetable history. Certificate support is current published links and source handoffs only. Planned dates are extracted as source-provided date text; ISO year is left unknown where absent.
