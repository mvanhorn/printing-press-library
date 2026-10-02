# Drive Plaza focused absorb manifest

Approval: The user explicitly authorizes autonomous focused builds, routine scope gates and full supported scope in batch2-brief.md. Exactly one fresh MAX reviewer per CLI; no extra brainstorm/coordinator agents. This supersedes the Press's extra novel-feature-agent step. All implementation below is real shipping scope, no stubs.

## Ecosystem evidence
Targeted public searches for Drive Plaza CLI, MCP, Claude plugin/skill, automation, npm/PyPI SDK found no relevant dedicated implementation. GitHub official external-plugin page was not retrievable through web; no ecosystem feature claims are drawn from it. First-party English and Japanese forms are the ground truth. No third-party code absorbed or attribution invented.

## Customer model
Japan self-drive visitor: identify the right IC and road, compare source toll/timing alternatives with explicit vehicle/JST date, and plan directional stops. Trip planner: distinguish conditional ETC estimates and notices from guaranteed prices or current restrictions. Agent: retrieve compact facts with stable IDs, Japanese names, explicit unknowns, source timestamps, actionable failures and canonical handoffs.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | IC name/road/code discovery | Public IC XML | driveplaza-pp-cli interchanges | Stable IDs and Japanese names, bounded pages |
| 2 | Road catalog | SA/PA form | driveplaza-pp-cli roads | Bilingual code join |
| 3 | Vehicle/category/date/priority conditions | Route form | driveplaza-pp-cli conditions | Offline catalog plus strict10-minute JST validation |
| 4 | Source quote alternatives | SearchQuickEN HTML | driveplaza-pp-cli route | Separate standard/ETC/ETC2.0, distance and traffic timing |
| 5 | Up to5 waypoints and road exclusions | Route form | (behavior in driveplaza-pp-cli route) flags preserve source semantics | Assumptions echoed with quote |
| 6 | Directional stop/facility discovery | SAPAServResEN HTML | driveplaza-pp-cli sapa list | IDs include direction; icon availability respected |
| 7 | Facility enum catalog | SAPAServiceEN form | driveplaza-pp-cli sapa facilities | Source facility IDs and labels |
| 8 | Stop facility sections and hours | SAPA detail HTML | driveplaza-pp-cli sapa detail | Japanese name, adjacent stops and source weekday hours |
| 9 | Advisory notices | Official traffic RSS | driveplaza-pp-cli notices | Date/title/source URL; active status explicitly unknown |
| 10 | Official planning/restriction/live-map handoffs | Planned-work page and route result | driveplaza-pp-cli handoff | Canonical labeled URLs without status inventions |
| 11 | Compact selection and pages | User brief | (behavior in driveplaza-pp-cli sapa list) limit/offset and shared select | Summary/detail separation, bounded output |
| 12 | Native reference forms | Saved page contract | (generated endpoint) reference rest-form | Low-level source page inspection for maintenance |

## Transcendence
These are focused improvements to the provider workflow, not claims of unique technology.
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Quote assumptions stay attached | route | hand-code | Preserve JST/vehicle/priority/conditionalETC alongside compact alternatives | none |
| 2 | Bilingual IC identity | interchanges | hand-code | Join English result IDs with source Japanese code lookup; partial enrichment warnings | none |
| 3 | Direction and unavailable facilities stay distinct | sapa list | hand-code | Source active/gray icons decoded without treating absence as confirmed facility | none |
| 4 | Advisory status uncertainty | notices | hand-code | Preserve release/postponement titles and publication dates; no active-closure inference | none |
| 5 | Canonical restriction handoff | handoff | hand-code | Combine observed first-party planning, live-map, construction and ETC-lane URLs | none |

## Cut ideas / limits
No local discount formulas, comprehensive live traffic, active-closure inference, stale non-East facilities presented as current, code-only EN quote entry (404), paid data, purchases, accounts, resident browser transport, or unbounded nationwide facility crawl. Online read-through data needs no bulk store/sync; cache enrichment disabled. Daily/weekday hours are source text, not current-open guarantees.
