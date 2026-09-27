# Ikyu domestic accommodation: access and semantic contract

Research date: 2026-09-27, Asia/Tokyo. Use this reference when approving scope, modelling prices or amenities, and reviewing acceptance. These are source findings and proposed implementation rules; the user has not yet approved the workflow manifest.

## Access, cost, and coverage

| Finding | Evidence | Consequence |
|---|---|---|
| The consumer service includes hotel/ryokan search and information collection without a service fee. Guest use exists. | [Domestic accommodation terms, articles 1–2](https://www.ikyu.com/terms/service/) | Anonymous domestic accommodation discovery is a viable product premise. This is consumer-site access, not an officially documented API contract. |
| Registration, joining, and annual membership are free; registration is optional, with some plans/services unavailable to guests. | [Registration FAQ 1107](https://faq.ikyu.com/app/answers/detail/a_id/1107) | No paid key or membership cost is established for the proposed anonymous mode. Account-specific coverage must remain explicitly excluded. |
| Gold and higher members have special plans/private sales; Diamond benefits have conditions. | [Membership stages FAQ 853](https://faq.ikyu.com/app/answers/detail/a_id/853) | Anonymous results are not exhaustive inventory and do not establish a member's eligible best price. |
| Availability and rates may change without notice and may differ from information supplied elsewhere. Extra bath/accommodation taxes can be separately payable. | [Domestic accommodation terms, articles 4 and 7](https://www.ikyu.com/terms/service/) | Record retrieval time, source, stay conditions, and additional charges; link to Ikyu for current confirmation. |
| The generic robots group disallows `/search$`, `/search?`, `/member/`, `/booking/new-draft/`, reservation routes, and `/api/amp/accommodations/0*/coupon`. It also lists domestic accommodation/area/onsen sitemaps. | [robots.txt](https://www.ikyu.com/robots.txt), fetched HTTP 200 | Record these as published crawler preferences. They are not by themselves a legal conclusion. Prefer bounded public property/area reads; do not infer authorization to crawl or to access account/checkout paths. |
| Terms prohibit unapproved commercial-purpose use and permit service/specification changes. No explicit blanket automation clause was found in the reviewed accommodation terms. | [Domestic accommodation terms, articles 2 and 10](https://www.ikyu.com/terms/service/) | Do not represent this work as official API access or commercial authorization. Authentication, access challenges, and undocumented-interface breakage remain possible. |

No public accommodation developer specification, API key programme, documented quota, or API price was established during this research. This is an evidence gap, not proof that none exists. Network probes and browser discovery must separately establish the selected operations. HTTP 200 for a public page proves reachability of that page only.

Scope recommendation: Japan domestic accommodation listings on `www.ikyu.com`, including hotel/ryokan/property pages and their public room/plan offers. Exclude restaurant, spa booking, overseas (`/global/`), flight packages, account history, coupon acquisition, reservation creation, payment, cancellation, and unattended crawling. An area result page or bounded shortlist is not complete coverage of Japan.

## Price contract

1. **Display basis.** Ikyu exposes “use now” and “save” point modes. The former displays the total after applying points earned by that booking; displayed rates assume online card payment and vary with member stage. Keep original/headline amount, sale-adjusted amount, conditional point-adjusted amount, and amount basis separate. [Ikyu homepage display explanation](https://www.ikyu.com/)
2. **Points earned versus applied.** Domestic guest bookings can use immediate points at regular-stage rates, but guests cannot normally save earned points. Campaign uplift and immediate-use availability depend on the plan. Store earned-if-saved and applied-if-used as alternative scenarios; do not subtract both. [Points FAQ 872](https://faq.ikyu.com/app/answers/detail/a_id/872), [Point acquisition FAQ 875](https://faq.ikyu.com/app/answers/detail/a_id/875)
3. **Payment and membership.** Ordinary domestic card rates are 2/3/4/5% for Regular/Gold/Platinum/Diamond; on-site rates are 1/1.5/2/2.5%. The applicable stage is the booking-time stage. These are explanatory defaults, not a replacement for offer-specific server values. [Membership stages FAQ 853](https://faq.ikyu.com/app/answers/detail/a_id/853)
4. **Coupons and calculation.** Held-point and ordinary coupon use generally reduce the points calculation base. Furusato donation coupons are an explicit exception. PayPay-funded amounts have their own treatment. The FAQ records a calculation change for bookings made after 2026-09-09 12:00; preserve source-calculated integers rather than reconstructing prices with a universal percentage formula. [Points/coupon interaction FAQ 192](https://faq.ikyu.com/app/answers/detail/a_id/192)
5. **Coupon eligibility.** Coupons may require acquisition, a code, or selection from a list. Domestic guests can use code/list coupons, while acquisition coupons require membership. Conditions can depend on the facility, plan, dates, payment, and distribution source. One domestic booking can use one coupon. A visible coupon is not an applied coupon or verified user entitlement. [Coupon FAQ 837](https://faq.ikyu.com/app/answers/detail/a_id/837), [Single-coupon FAQ 1545](https://faq.ikyu.com/app/answers/detail/a_id/1545)
6. **Amounts payable.** Preserve whether the quoted amount includes tax/service charges and any explicitly excluded local taxes or charges. An anonymous page's conditional total is not a checkout-confirmed final payable amount. Use a conditional display total with its assumptions; leave final payable unknown when checkout or eligibility is unresolved. [Domestic accommodation terms, article 4](https://www.ikyu.com/terms/service/)

Prices must be integer JPY and identify whether they are a stay total, per-night value, per-person value, or undated “from” value. An undated lower bound cannot become an available quote for supplied dates. No returned offers is distinct from an HTTP/parser failure, a member-only restriction, and a partial page.

## Rooms, plans, and amenities

Keep `Property`, `Room`, `Plan`, and `Offer` separate. The offer joins property/room/plan identity with stay, occupancy, price, and availability. Preserve Japanese names and descriptive source text; translations, if added, are supplemental.

Ikyu distinguishes a property with outdoor-bath rooms, an individual room with an outdoor bath, and an individual room with a hot-spring outdoor bath. Public source categories include [property-level outdoor-bath rooms (`aca16`)](https://www.ikyu.com/onsen/000030/aca16/), [room outdoor bath (`acr262144`)](https://www.ikyu.com/onsen/000110/acr262144/t3928/), and [room hot-spring outdoor bath (`acr65536`)](https://www.ikyu.com/onsen/000110/acr65536/si27/). These labels establish different meanings; route/filter encoding still requires live contract validation.

Model bath location/access and water type independently: room-private versus shared/reservable, indoor versus outdoor/semi-outdoor, and hot spring versus non-hot-spring versus unknown. A shared onsen or property badge cannot establish a room amenity. A room outdoor bath cannot establish hot-spring water. Use explicit true/false/unknown values supported by room-level evidence; absent data means unknown. Keep shared facilities and room amenities in separate fields.

## Proposed architecture

- **Source adapter:** bounded anonymous public reads, transport timeouts/body limits, a versioned Nuxt decoder, and schema validation. Keep observed source field names inside this adapter. Missing expected result shapes produce a source/schema error, never a fabricated empty result. Root/live-contract findings must supply the verified request/query contract.
- **Domain layer:** typed property/room/plan/offer values; explicit unknowns; original Japanese text; quote assumptions; structured cancellation terms plus source wording; per-field provenance where interpretation matters.
- **Workflow layer:** bounded discovery → chosen property → chosen room/plan → compatible comparison → a source/booking link. Lists expose concise summaries; details are fetched lazily. No implicit fan-out over every property, room, or date.
- **Comparison:** partition offers by dates/nights, full occupancy including child categories, room count, requested room characteristics, meals, cancellation terms, payment and price scenario, and verified eligibility. Rank within compatible groups. Return differences and missing evidence for incompatible/unknown groups. Two unknown values do not establish equivalence. Matching Japanese plan names alone does not establish matching conditions.
- **Output:** compact JSON by default with stable schema/version, selected `--fields`, explicit limits/page metadata, `has_more`/continuation where supported, and `partial` reasons. Normal output omits raw HTML, image arrays, and unrequested prose. Reserve diagnostics for stderr.
- **Local cache:** bounded on-disk public response or normalized result cache keyed by request plus schema/parse version. Include fetched time and age; use short freshness for rates, longer freshness for static property information, and an explicit refresh option. Expired data is returned only under an explicit stale policy and remains marked stale. Never cache authentication/session data.
- **Request control:** small fixed concurrency, maximum request budget per invocation, capped retry/backoff for transient reads, `Retry-After` handling, and clean context cancellation. Authentication/challenge/denial responses produce actionable errors; no identity rotation or challenge bypass.

Proposed product bounds for the scope manifest: default 10 results, maximum 50; comparisons at most 5 explicit offers/properties; no automatic date-range expansion; concurrency at most 2 and retries at most 2. Exact request/body/cache limits should be fixed after the discovered SSR sizes are measured. These are CLI choices, not claimed Ikyu limits.

The official booking horizon allows check-in from today to at most 365 days ahead; validate in Asia/Tokyo. A facility can offer a smaller range. [Booking horizon note in FAQ 1534](https://faq.ikyu.com/app/answers/detail/a_id/1534). Source validation is still required for maximum nights, adults, child categories, and room count; document conservative CLI bounds as product bounds if no official limit is observed.

## Acceptance gate

Approve only a manifest in which each workflow names inputs, outputs, data source, authentication/cost, bounds, proof case, and explicit exclusions. Implementation and acceptance need the same manifest version.

Deterministic evidence must cover malformed/changed Nuxt shapes; empty versus unavailable versus partial responses; room/private/shared bath distinctions including explicit non-hot-spring text; unknown versus false; independent price/points/coupon/member fields; source amount preservation and integer arithmetic; undated “from” rates; child/room/date propagation; cancellation differences and unknown comparability; pagination; selected fields; cache freshness; maximum body/request bounds; transient retry versus authorization failure; and cancellation of pending work.

Live evidence must include one ordinary hotel and one ryokan, dated room/plan retrieval, matching echoed dates and occupancy, at least two materially different meal/cancellation or points conditions, Japanese names, and a source link that preserves the selected stay. Verify displayed amounts and conditions against the anonymous website. If current inventory lacks a required live case, report that limitation and use a clearly labelled fixture for the branch; a fixture cannot be reported as live proof.

Measure cold and warm executions for the approved workflows: HTTP request count, response bytes, output bytes, elapsed time, and peak RSS. Compare these against agreed budgets; record environment, date, result count, and cache state. Structural build/shipcheck success alone does not accept semantic accuracy or resource bounds.

## Raw-source receipts

All URLs below were fetched anonymously through `fetch-docs.sh` with final HTTP 200 on 2026-09-27: homepage, `/terms/service/`, `/robots.txt`, FAQ IDs `1107`, `192`, `853`, `1545`, `872`, `875`, `837`, `1534`. Captures reside in `$TMPDIR/printing-press-fetch-docs/` and were inspected locally. Initial sandbox DNS failures were environment restrictions (`status=000`); the authorized network retry returned 200 and they are not Ikyu rejection evidence.

The bath-category pages were checked through official-domain web results; preserve that weaker evidence distinction until raw/live contract validation is complete. This file does not claim any private GraphQL operation has been called or any booking executed.
