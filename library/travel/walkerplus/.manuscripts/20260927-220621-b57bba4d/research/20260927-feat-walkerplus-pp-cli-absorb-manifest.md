# Walkerplus scope manifest
Status: APPROVED by user on 2026-09-27. Root Astra owns final scope selection. Brainstorm is advisory; focused user brief overrides generic breadth requirements.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse events by prefecture/city/category | Walkerplus listing routes and card+JSON-LD | walkerplus-pp-cli search | Japanese title, stable ID, exact source dates, bounded pages |
| 2 | Full event facts | Walkerplus base/data/price pages | walkerplus-pp-cli event | Venue/access/hours/admission/reservations/official links with nulls |
| 3 | Discover filters | Walkerplus navigation | walkerplus-pp-cli areas | Verified prefectures/cities with Japanese labels and aliases |
| 4 | Discover event kinds | Walkerplus category links | walkerplus-pp-cli categories | Festivals, fireworks, illuminations, exhibitions, markets, seasonal categories |
| 5 | Source-backed free/indoor | Walkerplus event admission and explicit venue prose | (behavior in walkerplus-pp-cli shortlist) --free / --indoor | Unknown and conditional attributes never pass strict filter |
| 6 | Agent output and bounded traversal | User brief | (behavior in walkerplus-pp-cli search) compact JSON, --limit, --page, --max-pages, --select | Coverage/source route/freshness metadata and stderr diagnostics |

## Differentiation (Astra accepted after adversarial brainstorm)
| # | Feature | Command | Buildability | Why It Helps |
|---|---------|---------|--------------|--------------|
| 1 | Trip-window shortlist | shortlist --from 2026-10-10 --to 2026-10-12 --prefecture kyoto | hand-code | Match exact editions/ranges against trip without assuming every envelope day is active |
| 2 | Starting/ending during trip | shortlist --timing starts or --timing ends | hand-code | Catch openings and last opportunities using source boundary dates |
| 3 | Schedule confidence | shortlist | hand-code | Handle recurrence/exclusions; unresolved or seasonal dates are possible, never confirmed by schema envelope alone |
| 4 | Evidence-backed constraints | shortlist --free or --indoor | hand-code | Enrich bounded candidates and require explicit source evidence; both flags mean AND |
| 5 | Explainable dedup/ranking | shortlist --sort relevance | hand-code | Stable IDs, stable tie ordering, reason fields and bounded detail enrichment |

## Contract
Search stays cheap and listing-only by default. Shortlist enriches only a bounded candidate set; event retrieves full facts. Original Japanese titles remain authoritative, English aliases apply to filter labels. Preserve date prose/exceptions/hours/weather/cancellation/postponement/reservation. Unknown price is null; child-free or free parking is not free event admission. Indoor-on-rain and mixed venues stay unknown. Retain source facts separately from derived trip matches. Asia/Tokyo dates, explicit edition year and source/fetch timestamps.

Native site date routes are month/day with no year selector or arbitrary range. CLI applies exact local year/range filtering to bounded source pages and records sampled routes. Undisclosed future editions stay undisclosed; cross-year coverage can be incomplete. Seasonal phrases override artificially exact JSON-LD endpoints. Source overall range is never daily-activity proof.

Default10 results,3 listing pages,10 detail candidates; explicit caps and continuation/coverage. HTTP retries/concurrency/response/cache bounded; TTL+refresh; no resident browser, no bulk database/sync, no additional runtime provider. No stubs or paid/auth dependencies. Runtime Go HTTP only.

## Acceptance
All six absorbed rows and five cohesive shortlist behaviors implemented, deterministic recurrence/exclusion/year-boundary/dedup/unknown-price tests, live multi-region/category date+location assertions, cold/warm bytes/request/latency/RSS measurements. Buildable verified workspace-root checkout plus concise README/agent skill. No publication/PR or shared config edits.

## Cut audit
Required three-pass brainstorm: novel-features-brainstorm.md. Astra accepts the five surviving behaviors above (scores8–9/10), consolidated into one shortlist workflow family plus event detail. Rejected: exhaustive native range search, automatic itinerary, inferred free/indoor classification, popularity score, persistent alerts. No scope expansion. All five require custom Go, zero automatically emitted complete product features.
