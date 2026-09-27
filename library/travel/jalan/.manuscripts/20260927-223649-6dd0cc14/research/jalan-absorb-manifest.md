# Jalan focused accommodation scope

Status: approved by user via scope gate on 2026-09-27.

## Boundaries
Jalan accommodation only, read-only anonymous public HTML, no shared tool settings. Existing legacy API keys and new Korean MCP are researched alternatives, excluded from runtime. Root directly owns customer model, research synthesis, architecture, planning and acceptance as the user explicitly requires; this overrides the skill’s novel-feature-planner delegation. gpt-6-sol/max handles bounded implementation and tests.

## Source capability classification
| Capability | Evidence | Scope |
|---|---|---|
| Public dated area search, property, offers and exact plan | HTTP 200 with real data on 2026-09-27; captures in discovery | Implement and verify across lodging types |
| Meals, smoking, bath filters | Rendered browser form and labeled values captured | Implement; live-test filter fidelity |
| Children, multiple rooms, pagination | Native form/link parameters observed | Implement conservatively, live-test and reject unsupported heterogeneous occupancy |
| Exact price, coupon offer, points, fees and cancellation | Sample plan 385995/03912759/0576806 exposes all separately | Preserve units/raw notes, explicit unknown final payable total where extras are unresolved |
| Japanese/English keyword input | Both HTTP 200, but English matches text and is not equivalent destination coverage | Use explicit alias catalogue for destination resolution; unknown aliases actionable, no implied translation |
| Legacy Web Service API | Docs 200, HTTP endpoint 400 missing key, HTTPS timeout; registrations closed | Credential-gated, not implemented or claimed live-verified |
| Jalan-hosted MCP hotel-search | Initialize, tools/list and one dated search work anonymously | Excluded: ko_KR only, no room/child inputs or pagination, price basis underdocumented |
| Booking, payment, account actions, coupon redemption | Out of user scope | Excluded |
| Exhaustive inventory and final checkout fees/discount eligibility | No reliable anonymous guarantee | Explicit limitation |

## Absorbed features
| # | Feature | Best source | Our Implementation | Added value |
|---|---|---|---|---|
| 1 | Destination/date/party search | Jalan native UI, ngs/yadosearch-api workflow | jalan-pp-cli stay search | Compact bounded results, exact echoed occupancy and freshness |
| 2 | Property detail/access/amenities/review categories | Jalan property HTML | jalan-pp-cli stay property | Lazy detail with evidence and source category labels |
| 3 | Room/plan inventory and handoff | Jalan dated offer and plan HTML; yadosearch workflow | jalan-pp-cli stay offers | Distinct IDs and decision-critical offer summaries |
| 4 | Exact room/plan terms | Jalan plan HTML | jalan-pp-cli stay plan | Tax/cancellation/meals/check-in evidence preserved |
| 5 | Source/coverage discovery | Native UI evidence and documented alternatives | jalan-pp-cli stay capabilities | Agent can discover supported searches and next action |

## Additions
| # | Feature | Command | Buildability | Why useful | Long Description |
|---|---|---|---|---|---|
| 1 | Bath distinctions | stay plan | hand-code | Separate room bath, outdoor bath, private reservable bath and hot-spring water; absent evidence stays unknown | Inspect exact room bath evidence; property facilities do not prove room facilities. |
| 2 | Quote interpretation | stay plan | hand-code | Distinguish whole-stay base quote, per-person/night amounts, conditional discounts, points and extra fees | Compare base quoted amounts only when scope and occupancy match; final payable may remain unknown. |
| 3 | Bounded alternatives | stay compare | hand-code | Equal-party date or plan comparison without unbounded crawling, with failed cells retained | At most five alternatives, bounded detail requests, no false zero prices. |
| 4 | Traceable extraction | stay property | hand-code | Preserve Japanese names and short source evidence with field provenance | Unknown attributes remain explicit; source links let callers inspect original terms. |
| 5 | Bilingual destination resolution | stay locations | hand-code | Explicit Japanese/English aliases avoid misleading English keyword searches | List supported aliases and IDs; unsupported locations instruct use of a source area code. |

All additions hand-coded; no stubs. Cache is observation reuse, never offline live availability. Defaults: limit 5, page 1, max results 30 per source page, inventory uncached unless caller explicitly requests max age; detail cache bounded and refreshable. Requests, timeout/retry/concurrency, response bytes and freshness are exposed. Final exact bounds subject to verified source contracts.

## Acceptance
All scoped commands resolve and compile; focused deterministic tests protect extraction, occupancy, pricing, pagination and partial errors. Live E2E includes regional ryokan/onsen and urban hotel, source fact assertions, filters, dated party variations, output selection, cached/uncached metrics and typed errors. Shipcheck and live dogfood acceptance then promotion/archive are required. No promotion while source correctness remains unresolved.
