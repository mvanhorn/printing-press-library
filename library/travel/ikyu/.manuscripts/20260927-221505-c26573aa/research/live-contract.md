# Ikyu anonymous accommodation live contract

Verified on 2026-09-27 using anonymous read-only HTTP requests. No login, cookies, account actions, reservations, or challenge solving were used. Research helpers only were created; no product implementation exists.

## Feasibility

**Native Go and lightweight JSON access both work.** A standalone Go 1.27.1 `net/http` client using its default transport and no explicit User-Agent fetched a dated property page with HTTP 200 / HTTP/2, 860,225 decoded bytes, in 1.983 seconds. The page contained `__NUXT_DATA__`. Ordinary curl GETs also returned current HTTP 200 responses. The root's initial Press/browser 403 results are environment/route specific; their exact cause remains unproven.

After the root captured browser traffic, two exact public GraphQL operations were replayed without credentials at `POST https://www.ikyu.com/graphql?lang=ja-JP`, using only `Content-Type: application/json`:

| Operation | HTTP / GraphQL result | Compressed bytes | curl network time |
|---|---|---:|---:|
| `PlansAndRooms` | 200, JSON data, no errors | 6,734 | 0.234 s |
| `RoomPlanDetailAlt` | 200, JSON data, no errors | 4,444 | 0.354 s |

These operations are a proven efficient alternative to 0.7–1.2 MB property/list HTML. Their full captured documents and variables are preserved in `discovery/probes/`. A Go POST replay was not separately tested; a Go default anonymous GET and curl JSON POST were both tested. Honor HTTP status **and** the GraphQL `errors` array.

## Verified capabilities

| Capability | Verified source and exact response path | Limits / evidence |
|---|---|---|
| Property metadata | SSR `AccommodationIkyu.accommodation`, `AccommodationMeta.accommodation`, `AccommodationInformation.accommodation`, `AccommodationMap.accommodation` | Hotel `00000600` / 八ヶ岳高原ロッジ / `RESORT_HOTEL`; ryokan `00002889` / 創業大正十五年 蓼科 親湯温泉 / `INN`. IDs remain strings with leading zeros. Names, areas, coordinates, amenities, bath details, check-in/out and notes are present. |
| Dated property rooms / plans | `PlansAndRooms.accommodation.searchRooms2.rooms` and `.searchPlans2.plans`, each with `edges[].node` and `totalCount` | SSR previews return first 3 rooms or plans and first 2 plan/room amounts. Ryokan had 8 available rooms / 8 plans for 2026-10-18, while `top=rooms` returned 3 rooms. Never imply preview completeness. |
| Lightweight room / plan listing | Replayed `PlansAndRooms` JSON operation, same paths as SSR | Captured 2026-11-17 request with `roomAttributes:["18"]` returned 1 room, totalCount 1, and plan totalCount 7. Query declares `roomsFirst` / `plansFirst` default 3 and `roomsOffset` / `plansOffset` default 0. Offset execution remains for implementation tests. |
| Full room / plan details and cancellation | `RoomPlanDetailAlt.accommodation.roomPlan.room`, `.plan`, `.children` | Exact public route `/{accommodationId}/{planId}/{roomId}/` is exposed in JSON-LD and fetched successfully for both hotel and ryokan. JSON replay also succeeded. Full room attributes, bath-related room facts, beds/capacity/size, plan restrictions, settlement type, notes, cancellation rules and child pricing are present. |
| Dated amount and inventory | SSR `RoomPlanDetailAmount.accommodation.roomPlan.booking.amount`; `RoomPlanDetailInventory...booking.amount.inventory` | Ryokan room `10193741`, plan `11055986`, two adults / one room / one night: amount 30,800 JPY, baseDiscountAmount 30,800, discountAmount 24,640, discountAmountEarn 30,800, instantPoint 6,160, inventory 4. Preserve server integers and currency. |
| Other rooms for one plan / plans for one room | SSR `PlanDetailRoomList.accommodation.plan.amounts.edges` and `PlanDetailPlanList.accommodation.room.amounts.edges` | Chosen ryokan plan returned 8 rooms; chosen room returned 5 plans. These connections did not include totalCount in this document. They do not establish complete property-wide plan enumeration. |
| Dated destination search | SSR `ListPageDataIkyu.listPageIkyu.accommodations` | `/tokyo/140000/` returned totalCount 587, 20 edges, and pageInfo `{hasPreviousPage:false,hasNextPage:true}`. Nodes include stable ID, name, type, amount2, rating2, coordinates and nested room prices. GraphQL destination document was not captured/replayed in this subtask. |
| Destination pagination | `pn=2` redirects to `/tokyo/140000/p2/`; `ListPageDataIkyu` variables `first:20,offset:20` | Page 2 returned 20 different property IDs with totalCount 587. Page 1 uses offset 0. |
| Reviews and rating breakdown | SSR `AccommodationReviewListReviews.accommodation.reviews`; `AccommodationReviewListRating.accommodation.rating2` | Reviews use `first:20,offset:0,sortItem:"RECOMMEND",site:"IKYU",showOldReview:false`. Connection returned totalCount 107 and hasNextPage true. Rating2 had count 184 and average 4.53, illustrating distinct upstream populations; do not substitute one count for the other. Review pagination JSON replay remains untested. |
| Availability calendar | Browser-captured operation `AccommodationAvailabilityCalendar` | Captured by root, not replayed here. Variables include `calendarInput`, `onlyPlanSpecified`, `planId`, `onlyRoomSpecified`, `roomId`. Calendar date-window/default behavior needs implementation tests. |

Review node fields observed: `reviewId`, `comment`, `postDateTime`, `handleName`, `manufacturedTitle`, `totalRating`, `images`, six component ratings, `checkinDate`, `mealCd`, `roomName`, `usePeopleCount`, `roomCount`, `reply`, `site`. Omit `memberHashSha1` and interactive like/account fields from product and saved specimens.

## Search URL and GraphQL inputs

| User concept | Public GET query observed | Exact upstream input observed |
|---|---|---|
| Check-in | `cid=20261018` | `checkInDate:"2026-10-18"` |
| Check-out | `cod=20261020` | Server derives `lodgingCount:2`, canonical `lc=2`; checkOutDate is absent from the observed amount input. |
| Nights | Canonical `lc=1` / `lc=2` | `lodgingCount`; conflicting explicit lc and cod precedence was not tested. |
| Adults | `ppc=2` | `peopleCount:2`, separate from child counts |
| Rooms | `rc=1` | `roomCount:1` |
| Child categories | `cac`, `cbc`, `ccc`, `cdc`, `cec`, `cfc` | `childACount` through `childFCount`, independently preserved; sending all six as 1 left peopleCount at 2 and returned zero matching plans. |
| List page | `pn=2` → canonical `/p2/` | `first:20`, `offset:20` |
| Property presentation | `top=plans` / `top=rooms` | `onPlans` / `onRooms` booleans |
| Selected room | `rm=10193741&top=room` | Does not add a full room-detail SSR operation. Use a known room/plan combination's detail route or captured JSON query. |
| Budget | `bll=20000`, `bul=80000` | `budgetLowerLimit:20000`, `budgetUpperLimit:80000` |
| Meals | `mtc=003` | `meals:["003"]`; codes observed: 000 none, 001 breakfast, 002 dinner, 003 dinner+breakfast, 004 breakfast+lunch, 005 lunch, 006 three meals, 007 lunch+dinner |
| Room amenities | Property link `acr=18`; browser filter request | `roomAttributes:["18"]` = outdoor bath in the room; `"16"` = hot spring outdoor bath; `"20"` = nonsmoking. Keep room and property amenity scope separate. |
| Property amenities | Filtered destination response | `accommodationAttributes:["16"]` observed, distinct from roomAttributes; do not infer its meaning solely from the number. |
| Accommodation type | `accommodation_types=INN` | `accommodationTypes:["INN"]` |
| Destination | `/tokyo/140000/`; filtered Hakone capture | `areaIds:["140000"]` / `["160418"]` |
| Sort / price mode | `si=1` and route defaults | `sortItem:"1"` / default destination `"6"`, `sortOrder:"1"`, `discount:true`, `sortAmountTarget:"USE"`; detail request also uses `"EARN"`. Full public sort enum labels remain unverified. |

Budget, meal, property-amenity, and type variables above are **observed in SSR operation inputs**, but their combined search response redirected to `/search`. They are not a verified portable destination GET filter route. The test URL `/hakone/160418/?...&aca=16&acr=65536&accommodation_types=INN` normalized to `/search?...&acr=16&are=160418...`; it was not followed with further probing after this was identified. `/search` is excluded from future helper requests. Canonical SEO path masks such as `acr65536` and `acr262144` are a different encoding from GraphQL attribute IDs; do not place path mask integers directly into GraphQL roomAttributes.

Not verified: exact `roomTypes` / `planAttributes` query parameter behavior, keyword matching input, minimum-rating filter, full sort labels, exchange-rate conversion, destination JSON replay, public-filter path combinations, or max permitted first/page sizes. No auth requirement, published rate limit, or stable official API guarantee was inferred.

## Occupancy unit boundary

Every priced live specimen in this subtask uses `rc=1`. For one room, `ppc=2` echoes `peopleCount:2`, with child counts independent, and amounts describe that selected one-room stay. **Whether ppc is adults per room or party total when rc>1, and whether amount is per room or the aggregate multi-room booking total, are unverified.** Merely echoing roomCount would not establish price units. Until a controlled multi-room comparison/source explanation proves them, restrict v1 priced searches to rooms=1 or clearly reject unsupported multi-room requests; do not silently multiply/divide prices. No additional broad probes were made.

## Child pricing and cancellation semantics

Plan HTML and `RoomPlanDetailAlt.children` jointly establish the source categories:

| Kind / query | Source label | Meaning |
|---|---|---|
| A / cac | 小学生高学年 | Older primary-school children |
| B / cbc | 小学生低学年 | Younger primary-school children |
| C / ccc | 乳幼児(食事あり・寝具あり) | Meals and bedding |
| D / cdc | 乳幼児(食事あり・寝具なし) | Meals, no bedding |
| E / cec | 乳幼児(食事なし・寝具あり) | Bedding, no meals |
| F / cfc | 乳幼児(食事なし・寝具なし) | Neither meals nor bedding |

Exact numerical ages are not established here. The sample ryokan plan returns A/B `RATE` 80%, C/D `IMPROPRIETY`, E `FIXED` 5,500 JPY, F `FIXED` 3,300 JPY. Preserve the union type and raw value; unavailable is not zero cost.

Cancellation is a per-plan typed rule list, not one property-wide percentage. Sample ryokan policy `001` contains no-show 100%, day 0 100%, day 1 70%, day 3 50%, day 7 30%, day 10 20%; hotel policy `004` contains no-show 100%, day 0 80%, day 4 50%, day 10 20%. Each day rule includes nullable `hourMinutes` (null in these specimens). Preserve rule types/order, day and hour fields; do not invent a free-cancellation deadline or collapse absence into free cancellation. Cancellation date arithmetic/timezone semantics require explicit implementation verification.

Keep `amount`, `baseDiscountAmount`, `discountAmount`, `discountAmountEarn`, `discountAmountWithoutCoupon`, `point`, `instantPoint`, rates, inventory, currency and occupancy distinct. The root's official FAQ research identifies a 2026-09-09 points-rounding change: use upstream monetary values rather than deriving them from rates. The sample ryokan's property notes separately disclose accommodation tax 200 JPY and bath tax 150 JPY per person/night, whereas plan notes mention included consumption tax/service charges; expose source notes rather than asserting an all-inclusive total.

## Failure and default behavior

- `ppc=0` returned HTTP 200 after dropping ppc from the canonical URL and using `peopleCount:2`. CLI must validate positive adults locally and confirm returned occupancy.
- Impossible dates `cid=20261340&cod=20261341` returned HTTP 200 after removing both dates. PlansAndRooms had no checkInDate and returned undated offers. CLI must validate calendar dates before requesting and verify echoed date/nights to avoid presenting undated prices as dated availability.
- Valid cod two days after cid and no lc produced canonical `lc=2` and `lodgingCount:2`.
- An unsupported six-child occupancy returned normal empty matching plans, not a transport error.
- Property previews are partial even on HTTP 200. Preserve totalCount and explicitly represent pagination/collection completeness.
- Generated `booking.urlPath` values contain opaque booking parameters. They are excluded from saved specimens and product contracts. Only stable public property/plan URLs are suitable for handoff. No booking URL was visited.
- Initial browser/Press transport 403 results should remain in the root's reachability evidence alongside successful native HTTP and JSON results.

## SSR decoding and evidence artifacts

`__NUXT_DATA__` is a devalue reference table, not ordinary object JSON. Arrays/dictionaries contain table indexes, Nuxt Reactive/Ref wrappers must be unwrapped, and negative sentinel indexes must be handled. Do not treat node table index numbers as IDs or financial values. Data keys encode exact operation name and variables as `{"o":"Operation","v":{...}}-fetch`; skipped entries can be null. Root data/state repeat objects, so select named operation data rather than serializing the whole Nuxt root.

Owned artifacts under the current run:

- `discovery/probes/nuxt_probe.py`: local SSR decoder/research projection, verified against real property, plan and destination captures.
- `discovery/probes/fetch_probe.py`: public GET capture wrapper using the mandatory fetch-docs helper; captures URL, status, bytes and helper duration, excluding header values. Redirect behavior is retained.
- `discovery/probes/transport_probe.go` and `go-transport-result.json`: standalone native Go feasibility probe/result.
- `discovery/probes/capture-metadata.jsonl`: exact recent requested/effective URLs, status, decoded body bytes and duration. Helper duration includes redirects/cache overhead and is not a pure network benchmark.
- `discovery/probes/ssr-evidence.json`: selected public operations and schemas for hotel, ryokan, destination pages, children, invalid input and plan/cancellation cases; opaque booking paths and member hashes excluded.
- `discovery/probes/PlansAndRooms-replay-request.json` / `PlansAndRooms-replay-response.json`: exact captured read-only query/variables and sanitized real response.
- `discovery/probes/RoomPlanDetailAlt-replay-request.json` / `RoomPlanDetailAlt-replay-response.json`: exact captured read-only query/variables and sanitized real response.
- `discovery/probes/graphql-replay-metadata.json`: timings, compressed/decoded byte counts, status, errors absent and credentials absent for both JSON replays.

Raw response captures remain in the fetch helper's temporary directory, linked from metadata. Do not commit complete raw SSR documents: they include unrelated review identifiers and generated booking selection links.
