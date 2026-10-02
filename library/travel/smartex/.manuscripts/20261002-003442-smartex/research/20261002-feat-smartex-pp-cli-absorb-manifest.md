# Approved smartEX absorb manifest

User authorization: batch2-brief.md explicitly preauthorizes routine scope choices, own research and implementation, public read-only sources, one dedicated reviewer. This overrides phase08's extra brainstorming agent and routine gate prompts. No extra agent was spawned; the required single fresh-context MAX reviewer remains pending. No stubs.

Landscape search found no applicable public smartEX developer SDK, MCP or CLI. Results were unrelated GitHub handle/product names and a2019 Gmail-to-calendar gist, which accesses personal email and contributes no feature within this authorized scope. Official plugin catalog contained no smartEX integration. Incumbents are smartEX website/app and linked JR sources.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Resolve station names and corridor order | smartEX / linked JR official sources | smartex-pp-cli stations | Bounded JSON, Japanese names, explicit unknowns and source links |
| 2 | Identify corridors and train categories with booking links | smartEX / linked JR official sources | smartex-pp-cli route | Bounded JSON, Japanese names, explicit unknowns and source links |
| 3 | Compare fresh dated adult fares and membership restrictions | smartEX / linked JR official sources | smartex-pp-cli fare | Bounded JSON, Japanese names, explicit unknowns and source links |
| 4 | Compare discount deadlines and unresolved eligibility conditions | smartEX / linked JR official sources | smartex-pp-cli products | Bounded JSON, Japanese names, explicit unknowns and source links |
| 5 | Compute precise JST advance request and confirmation boundaries | smartEX / linked JR official sources | smartex-pp-cli window | Bounded JSON, Japanese names, explicit unknowns and source links |
| 6 | Check baggage size and required reserved seat class | smartEX / linked JR official sources | smartex-pp-cli baggage | Bounded JSON, Japanese names, explicit unknowns and source links |
| 7 | Retrieve practical boarding, changes and refunds guidance | smartEX / linked JR official sources | smartex-pp-cli policy | Bounded JSON, Japanese names, explicit unknowns and source links |
| 8 | Find current official basic timetables and validated regular service examples | smartEX / linked JR official sources | smartex-pp-cli timetable | Bounded JSON, Japanese names, explicit unknowns and source links |
| 9 | Prepare canonical booking links and an exact travel checklist | smartEX / linked JR official sources | smartex-pp-cli handoff | Bounded JSON, Japanese names, explicit unknowns and source links |
| 10 | Inspect source freshness and planning versus inventory boundaries | smartEX / linked JR official sources | smartex-pp-cli sources | Bounded JSON, Japanese names, explicit unknowns and source links |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Identify corridors and train categories with booking links | route | hand-code | Join compact public source facts with caller-specific date/class/party inputs and preserve uncertainty | none |
| 2 | Compare fresh dated adult fares and membership restrictions | fare | hand-code | Join compact public source facts with caller-specific date/class/party inputs and preserve uncertainty | none |
| 3 | Compare discount deadlines and unresolved eligibility conditions | products | hand-code | Join compact public source facts with caller-specific date/class/party inputs and preserve uncertainty | none |
| 4 | Compute precise JST advance request and confirmation boundaries | window | hand-code | Join compact public source facts with caller-specific date/class/party inputs and preserve uncertainty | none |
| 5 | Check baggage size and required reserved seat class | baggage | hand-code | Join compact public source facts with caller-specific date/class/party inputs and preserve uncertainty | none |
| 6 | Prepare canonical booking links and an exact travel checklist | handoff | hand-code | Join compact public source facts with caller-specific date/class/party inputs and preserve uncertainty | none |

## Acceptance and limitations
All10 domain commands implemented; generated14 public reference GET endpoints retained. Timetable is authoritative publication discovery plus explicitly bounded validated basic service examples, not complete or dated inventory. Fare is adult one-way public navigator, current month + next two; children unsupported with null totals. Discount route/exclusion/seat conditions remain unknown, with official price-matrix links. Any class/train options the source cannot express are rejected. No booking mutation or publication.
