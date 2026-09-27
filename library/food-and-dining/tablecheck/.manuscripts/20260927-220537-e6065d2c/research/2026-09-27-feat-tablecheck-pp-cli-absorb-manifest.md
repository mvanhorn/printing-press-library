# TableCheck focused scope manifest

Scope status: approved by the user on 2026-09-27T14:36:22.537283+00:00. TableCheck only; anonymous public HTTP planning reads and canonical booking handoff. No stubs. Root owns intent, planning and review under explicit user correction.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Geographic/cuisine/budget/date/party restaurant discovery | Public search UI, verified v2/shop_search, nbw/tc-mcp source | tablecheck-pp-cli venues search | Default10/max50; cursor pagination; stable IDs, Japanese names; geography explicit; budget labeled venue average; discovery availability never authoritative |
| 2 | Venue detail and booking conditions | Public v2/shops/{slug} | tablecheck-pp-cli venues get | Lazy detail, canonical URL, explicit null missing values |
| 3 | Cuisine lookup | Public v2/cuisines, nbw/tc-mcp | tablecheck-pp-cli cuisines list | Bounded filter, Japanese and English labels |
| 4 | Course summaries | Verified read-only POST hub/menu_items | tablecheck-pp-cli courses list | Default10/max50, stable course IDs, exact decimal amounts and tax/service flags |
| 5 | Course details | Verified menu_items and public course display | tablecheck-pp-cli courses get | Exact fine print, date/time/quantity/payment/eligibility/cancellation rules, unknown charge values |
| 6 | Venue party-specific times | Verified read-only POST hub/availability_calendar_v2 | tablecheck-pp-cli availability check | Explicit date/party, Tokyo time conversion, bounded available time summary; unknown ≠ sold out |
| 7 | Canonical booking URL | Public v1/v2 booking links, official Web Booking docs | tablecheck-pp-cli booking-url | Validated date/time/party handoff, no reservation created |
| 8 | Bounded shortlist/date scan | Same verified calendar | tablecheck-pp-cli availability scan | Max5 venues, max14 inclusive dates, partial successes/errors, calendar reused per venue |

## Transcendence
| # | Feature | Command | Buildability | Why it matters | Long Description |
|---|---|---|---|---|---|
| 1 | Bounded trip scan | availability scan | hand-code | One bounded operation compares dates across the shortlist | Check a bounded shortlist; use availability check for one venue. |
| 2 | Evidence status | availability check | hand-code | Search summaries differ from party calendar; source evidence controls status | Check venue-level times; use courses get for course eligibility and conditions. |
| 3 | Course cost and conditions | courses get | hand-code | Headline price can differ from fine-print range; unknown charges stay unknown | Inspect a named course; use availability check for venue-level times. |
| 4 | Visible freshness | availability check | hand-code | Cache age is inseparable from an availability observation | none |
| 5 | Canonical handoff | booking-url | hand-code | Completes the planning workflow without booking mutations | Produces a URL for human completion; availability check provides a timestamped observation. |

These five improvements are behaviors of the eight focused command paths, not five extra product areas. All require custom Go atop generated endpoint/client scaffolding; zero automatically emitted novel features.

## Bounds and output contract
Compact JSON by default; --select field projection; summary/detail separated; explicit null unknowns. Dates and party mandatory for authoritative checks. Search location uses explicit lat/lon/radius; cuisine keys from cuisines list. Currency comes from source, never assumed. Search budget is dinner-average budget, not all-in course price.
Calendar slots are venue-level. Course catalog eligibility and stock are separate. Actual instant availability, request, waitlist, sold out and unpublished require source evidence; unrecognized or insufficient evidence is unknown/unavailable, network/schema problems failed. Static waitlist policy is a condition, never current waitlist availability. No course-specific slot guarantee.
Local availability cache at most30seconds by default, original fetched_at visible; --refresh bypasses it, upstream staleness remains possible. Details can cache longer with explicit freshness. Max request budget20 per command, timeout10seconds/request, at most1 bounded retry, concurrency≤2. Scan max5venues×14days; one calendar per venue when source returned coverage suffices. Truncation/pagination metadata explicit.

## Acceptance
Build and fresh deterministic Go tests; statuses/decimal money/timezones/partial failures; live venue identity, cuisine/budget/date/party and cursor checks; at least one CLI availability compared to public source; cached and uncached bytes/requests/latency/peak RSS; Printing Press shipcheck/review/dogfood/promotion receipts. Root reviews and accepts, sol max workers implement/test with disjoint ownership. Local workspace delivery; no shared configuration changes.

## Limits and excluded scope
Undocumented consumer surface can change; partner API approval and negotiated pricing not required for tested public path. Calendar may return a wider horizon than requested; output is filtered to the requested range. Exact-time discovery search is a preference, while calendar provides actual slot booleans. No holds/bookings/payments/accounts, no background watcher, no Tabelog, no full-country sync, no inferred all-in cost or named-course availability.
