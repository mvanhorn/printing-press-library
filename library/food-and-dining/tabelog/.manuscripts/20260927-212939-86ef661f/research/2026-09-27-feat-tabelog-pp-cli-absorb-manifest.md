# Tabelog feature manifest

Status: approved by the user on 2026-09-27: "Approve and build (Recommended)" for the full eight-core plus five-saved-list scope. Source contract: `website-replay-contract.md`. Intent and acceptance priorities: `2026-09-27-feat-tabelog-pp-cli-brief.md`. Full comparator evidence: `ecosystem-evidence.md`.

## Absorbed scope

| # | Feature | Best source | Our implementation | Added value |
|---:|---|---|---|---|
| 1 | Typed area and station lookup | Gurume suggestions; native English JSON | tabelog-pp-cli areas | Typed choices, source identifiers, explicit ambiguity; cached lookups |
| 2 | Cuisine and category lookup | Gurume genres; native English catalogs | tabelog-pp-cli cuisines | English taxonomy including bars; distinguish broad and specific categories |
| 3 | Area, cuisine and keyword discovery | Gurume search; native English form | tabelog-pp-cli find | Check effective geography/category and preserve exact source identity |
| 4 | Source ranking and pagination | Gurume sorting; native English Next links | (behavior in tabelog-pp-cli find) Highest-rated source order and bounded pagination | Small default result set; returned/scanned/page/has_more metadata |
| 5 | Meal budget filtering | Gurume filter concepts; native English dynamic form | (behavior in tabelog-pp-cli find) Lunch/dinner minimum and maximum in supported yen thresholds | Validated upstream parameters; source price brackets retain their meaning |
| 6 | Structured restaurant details | Gurume detail; native English HTML/JSON-LD | tabelog-pp-cli show | Practical facts from one detail request; listed and review-based budgets remain separate |
| 7 | Agent output, projection and failures | Gurume JSON/MCP; user requirement | (behavior in tabelog-pp-cli find) Compact JSON, selection, stable identifiers and typed failures, shared with show/catalog commands | Full names/links/precision, unknown values, source and freshness metadata; no per-result detail fan-out |
| 8 | Discoverable agent interface | Gurume skill; Printing Press CLI/MCP | (behavior in tabelog-pp-cli find) Short help, agent skill and optional stdio MCP exposure | Common workflows visible first; advanced generated surface hidden from default agent context |

## Transcendence

| # | Feature | Command | Score | Buildability | Why this approach helps | Long Description |
|---:|---|---|---|---|---|---|
| 1 | Trip notebook with personal notes | lists add | 8/10 | hand-code | Local snapshots, memberships and notes preserve trip context; supporting show, note and remove complete the notebook lifecycle | Save fetched restaurants and personal notes in a named trip list. To update source facts, use lists refresh. |
| 2 | Offline factual comparison | lists compare | 9/10 | hand-code | Compare several saved venues and notes in one compact response, preserving separate meal budgets, unknown facts and retrieval times | Compare known facts and notes for saved candidates. For substitutes matching one candidate, use lists alternatives. |
| 3 | Bounded refresh with fact changes | lists refresh | 9/10 | hand-code | Compare old/new source facts while preserving personal notes and retaining valid snapshots for failed records | Fetch current source facts for selected saved restaurants and report changes. For a network-free check of old or missing evidence, use lists audit. |
| 4 | Saved alternatives with explicit constraints | lists alternatives | 7/10 | hand-code | Match saved candidates to an anchor's verified source area/category and optional known meal-budget ceiling, with explicit reasons | Find saved substitutes matching an anchor's source area and category, with optional cached meal-budget constraints. For general factual comparison, use lists compare. |
| 5 | Missing-evidence and freshness audit | lists audit | 7/10 | hand-code | Find which candidates need detail inspection and which facts remain unknown, avoiding unnecessary detail fetches | Inspect missing or old evidence in a saved list without network access. To retrieve current facts, use lists refresh. |

All five added workflows require handwritten Go; none is automatically implemented by the four raw source endpoints. The required three-pass brainstorm evaluated eleven candidates and retained these five. Persona evidence, killed candidates, exact alternative/audit semantics, persistence invariants and high-signal cases: `2026-09-27-novel-features-brainstorm.md`, Survivors section. Its semantic constraints are binding; the positive scope descriptions in this table are the final help copy.

Alternatives compare the saved set, using exact verified source-area identifiers and normalized source-category overlap. They preserve list order and expose unmatched/unknown counts; they do not imply live availability, proximity, or a new quality score. Audit distinguishes details not fetched, source-unknown facts, and data older than the requested threshold. Refresh labels newly acquired evidence separately from a proven change in the same source field.

## Runtime and accuracy contract

- Default `find`: five candidates, source highest-rated order, one listing page after location/category resolution; no automatic detail fetches. Explicit page/limit options bound additional work.
- `find` means live source discovery with an explicit cache/offline policy. The generated local `search` command, if retained, is advanced and names its local scope.
- Resolve named locations by kind and source metadata. Return choices for ambiguity. Canonical URLs/typed source choices provide a direct path without guessed geography.
- Budget options accept source-supported yen thresholds and an explicit meal. Preserve original range and parsed bounds; avoid bill guarantees. Optional local filtering reports its scanned-set scope.
- Facts carry source URL and retrieval time. Unknown values remain unknown. Station meters are labeled as distance from that station, not from the traveler.
- Notes/list membership are local user data. Refresh modifies source snapshots while retaining that data. A failed refresh leaves the last valid snapshot available and reports the error.
- Plain Go HTTP, bounded bodies/timeouts/retries/cache/concurrency, no running background process. Disable generator learning; optional MCP uses stdio only. Store initializes when needed.
- Parser drift, blocks, unexpected geography and unrecognized empty pages are explicit failures. Source facts and user data are not partially overwritten on parse failure.
- E2E uses the actual executable against independent real-source oracles, failure replay, and live city/bar workflows. Output tokens/bytes, latency, CPU, RSS and request counts are measured. Detailed acceptance: `acceptance-plan.md`.

## Comparator inventory outside the proposed scope

These are inventoried for an explicit focused scope decision, not shipping placeholders.

| Observed comparator capability | Source | Scope rationale |
|---|---|---|
| Rich terminal TUI and keyboard suggestions | Gurume | Structured CLI and agent workflows serve the requested usage with less interface/runtime surface |
| Date/time/party reservation search and booking-specific filters | Gurume API/MCP | Core source contract is discovery and practical details; live seat availability needs separate evidence |
| Full review pagination, review bodies, menu/course subpages | Gurume | Detail facts and source links support v1 decisions; extended text fetching would need explicit output limits and additional source tests |
| Photos/review-image retrieval | Historic python-tabelog API | Legacy manual is404; current discovery has no image-processing requirement |
| Nationwide catalogs, bulk crawler/database ingestion and multithreading | Ruby scraper; Gurume catalog | On-demand typed lookup and saved trip candidates fit the user's resource constraint |
| Legacy API-key/XML wrapper | python-tabelog | No working current official API established; public English HTTP is verified |
| Paid gateway/payment integration and chat-bot hosting | npm landscape | No value established for the requested standalone trip-discovery workflow |

All approved shipping rows will have working implementations. This proposal contains no stub commands.
