# Source contract

Checked against Ikyu's anonymous Japan accommodation pages and read-only GraphQL queries on 2026-09-27. This integration is unofficial; source field meanings below come from observed data and linked Ikyu guidance.

## Access and coverage

[Domestic accommodation terms](https://www.ikyu.com/terms/service/) describe free search/information access. [Membership is optional and free](https://faq.ikyu.com/app/answers/detail/a_id/1107), while some plans and benefits require a member stage. Anonymous results cannot establish personalized inventory or coupon eligibility. No public accommodation API contract or published programmatic quota was established.

Public property/room/plan reads worked without keys, cookies or a resident browser. Some homepage clients returned 403 while dated native Go HTTP and anonymous JSON requests succeeded. Treat denial as an access error; keep successful cached observations timestamped. Published [crawler preferences](https://www.ikyu.com/robots.txt) and the consumer terms are separate from proof that a route responds. This checkout does not assert commercial authorization.

The CLI covers domestic accommodation. Restaurant/spa products, overseas stays, account actions, coupon acquisition and reservation/payment/cancellation actions are outside its scope.

## Identity and evidence

Property, Room, Plan and dated Offer are separate. Preserve the eight-digit string IDs, original Japanese names and canonical public URLs. The plan URL orders IDs as `/{property_id}/{plan_id}/{room_id}/`, even when a CLI command accepts property, room, plan. Opaque booking action URLs are not handoff links.

A property-level facility does not prove an amenity exists in a selected room. Source room attribute `18` denotes an outdoor bath; `16` denotes a hot-spring outdoor bath; `20` denotes nonsmoking. SEO path masks such as `acr65536` are a separate encoding. Keep bath access/location, outdoor/semi-outdoor exposure and hot-spring water independent. Missing evidence stays unknown.

Review category values and counts retain their source population. The overall rating count can differ from a paged review list count; neither replaces the other. This product needs category scores rather than personal review text.

## Prices

Keep integer JPY amounts and their units. The original amount and the displayed headline can differ after a sale. Preserve each source scenario:

| JSON field | Meaning |
|---|---|
| `source_amount` | Original source amount, which may be struck through on the page |
| `headline_before_points` / `base_discount_amount` | Amount after base sale discounts and before immediate points; headline in the observed earn-points display |
| `earn_points_payable` | Source payable for saving/earning points |
| `instant_points_payable` | Conditional source payable for immediately using points |
| `points_earned` / `points_applied` | Alternative point scenarios; do not subtract both |
| `payable_without_coupon` / `source_coupon` | Separate no-coupon scenario and coupon evidence |
| `checkout_confirmed_payable` | Null until final booking checkout is known; this CLI does not perform checkout |

The `price.scenarios` list separates earn-points and use-now cases. Exact offer inspection currently uses the upstream default point variation; summaries retain `source_point_variation` so variants are not silently collapsed. The anonymous HTML pages observed during verification displayed earn-points prices by default. For one hotel, the original 57,540 JPY became a 54,664 JPY sale headline with 4,919 points earned; the JSON's separate use-now quote was 49,745 JPY. A conditional display total is not a checkout-confirmed final payable amount. Additional accommodation or bathing taxes can remain payable at the property.

The source's “use now” and “save” modes describe alternative point scenarios. [Guests can use immediate points at regular-stage rates](https://faq.ikyu.com/app/answers/detail/a_id/872), while displayed rates can depend on payment and [membership stage](https://faq.ikyu.com/app/answers/detail/a_id/853). Avoid subtracting both earned and applied points.

[Points/coupon rules](https://faq.ikyu.com/app/answers/detail/a_id/192) changed on 2026-09-09 and contain exceptions. Use source-calculated amounts; a displayed percentage is not enough to reconstruct the yen result. [Coupon types and eligibility](https://faq.ikyu.com/app/answers/detail/a_id/837) differ, and a visible coupon is neither an applied discount nor verified entitlement.

An undated “from” price is a lower bound, not availability for requested dates. A missing source amount is null, never zero. No matching inventory, invalid input, an access error and a changed response schema are distinct outcomes.

## Occupancy and cancellation

Adult and child counts are per room. Multiple rooms share the same type, plan and occupancy inputs; quoted amounts are totals across the requested rooms and nights. [Domestic multi-room guidance](https://faq.ikyu.com/app/answers/detail/a_id/94) explains this search convention. Unequal occupancy per room requires booking-screen adjustments and is outside this CLI. A controlled source quote doubled from 30,800 to 61,600 JPY when room count changed from one to two with adults fixed at two.

Ikyu child categories are source categories, not universal numeric age bands:

| Category | Japanese source label |
|---|---|
| A | 小学生高学年 |
| B | 小学生低学年 |
| C | 乳幼児(食事あり・寝具あり) |
| D | 乳幼児(食事あり・寝具なし) |
| E | 乳幼児(食事なし・寝具あり) |
| F | 乳幼児(食事なし・寝具なし) |

Preserve child pricing as fixed amount, rate or unavailable. Unavailable does not mean free. Validate actual calendar dates and occupancy locally: the website can silently replace invalid adults or dates with defaults. Validate returned stay conditions before labelling a quote dated.

Cancellation is a plan-specific ordered rule list. Preserve no-show/day rules, percentages and nullable time fields. Missing rules do not mean free cancellation. Interpret dates in Japan time; do not invent a cancellation deadline when the source supplies no exact one.

## Meal filters

The `--meals` filter accepts source codes separated by commas: `000` no meals, `001` breakfast, `002` dinner, `003` dinner and breakfast, `004` breakfast and lunch, `005` lunch, `006` three meals, `007` lunch and dinner. Preserve the returned Japanese meal name alongside its code.

## Comparison

Compare known equal stay dates/nights, full occupancy and room count, property and room identity, meals, cancellation, payment/price scenario and relevant eligibility. Unknown values never prove equivalence. Different rooms, properties or dates remain useful alternatives with explicit differences; their price difference alone is not equivalent-offer savings.

Observed price differences show both source values and units even when offer terms are incompatible or eligibility is unknown. These observations are not savings claims. Missing amounts remain null, and values with different currencies or units have no computed delta. Monetary differences do not by themselves make otherwise verified equal terms incompatible.

Unfiltered search retains source properties without a dated price, with `from_price: null`, canonical dated links and explicit detail gaps. These candidates do not establish availability. Budget or preference searches still exclude candidates whose requested criteria cannot be verified.

Source summaries can be previews. Preserve total counts, continuation and detail gaps. Apply local filters only within the reported page coverage. Room budget and meal filters apply to each returned plan together; plans outside those conditions are omitted. Plan totals and scanned counts still describe the source window, while returned counts describe matching plans. Without budget or meal filters, an empty plan window retains the room identity and nested pagination; it does not imply that the room is absent. Availability is a timestamped observation; refresh selected offers before booking handoff.

Bulk room-plan summaries currently return no nightly date details even when requested. They can verify occupancy and retain the requested stay, but their date echo remains explicitly unverified. The exact offer endpoint supplies nightly dates and is required for a date-verified quote.
