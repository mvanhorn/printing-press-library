# Agoda browser-sniff report

Backend: browser-use 0.13.8 (CLI/heredoc mode), attached to the user's running,
logged-in Chrome via CDP. Capture method: `Fetch.enable` request-stage interception
(page-level `window.fetch` patching was insufficient — Agoda issues its search calls
from a Web Worker, whose global scope a page-context patch cannot reach).

## Runtime classification
`probe-reachability` => **mode: standard_http**, confidence 0.95,
needs_browser_capture=false, needs_clearance_cookie=false.
Verified by anonymous replay below. The printed CLI ships plain HTTP.
No browser sidecar. No clearance cookie. Cookies are needed only for
member-priced/authenticated surfaces.

## Endpoints discovered

### GraphQL (three separate endpoints, operation-dispatched)
| Endpoint | Operation | Purpose |
|---|---|---|
| `POST /graphql/search` | `citySearch` | Primary hotel search |
| `POST /graphql/search` | `priceTrendSearch` | Per-property price trend over a date window |
| `POST /graphql/search` | `propertyAllotmentSearch` | Room/rate availability calendar |
| `POST /graphql/property` | `propertyDetailsSearch` | Full property detail (composed multi-request query) |
| `POST /graphql/npc` | `SimilarCitySearch` | Similar-destination recommendations |
| `POST /api/activities/graphql` | `search` | Activities — OUT OF SCOPE (hotels-only run) |

Introspection is disabled on all GraphQL endpoints, so full query documents were
captured from live traffic and saved verbatim:
- `citySearch.graphql` (30,509 chars)
- `citySearch-request.json` (full variables tree, PII-scrubbed)
- `graphql-capture.json` (all intercepted requests, PII-scrubbed)

### REST
| Endpoint | Method | Purpose |
|---|---|---|
| `/api/cronos/search/GetUnifiedSuggestResult/{sf}/{langId}/{platform}/{n}/{locale}/` | GET | Destination autocomplete (text -> cityId) |
| `/api/cronos/property/review/ReviewComments` | POST | Paginated reviews |
| `/bff/trips/save-to-trip/retrieve` | POST | Saved/wishlist properties (auth) |

## Replayability proof (Cardinal Rule 5)
- `citySearch` replayed **anonymously with zero cookies**: HTTP 200, 776-798 KB,
  **49 properties** returned for Tokyo (cityId 5085).
- `GetUnifiedSuggestResult` replayed anonymously: HTTP 200, "Tokyo" -> cityId 5085.
- `ReviewComments` replayed anonymously: HTTP 200 with real comment objects.
=> Shippable surface is replayable plain HTTP. No resident browser required.

## Pricing model (the headline finding)
```
pricing.offers[].roomOffers[].room.pricing[].price
  .{perNight | perRoomPerNight | perBook}
    .{exclusive | inclusive}
      .{display, crossedOutPrice, originalPrice, cashbackPrice,
        displayAfterCashback, rebatePrice, pseudoCouponPrice,
        loyaltyOfferSummary.basePrice.{exclusive, allInclusive}}
```
Agoda returns BOTH the advertised (`exclusive`) and true all-in (`inclusive`)
price in the same response; the website displays `exclusive` by default.
Request-side levers: `searchCriteria.requiredPrice` (Exclusive|Inclusive) and
`searchCriteria.requiredBasis` (PRPN | ...).

Measured hidden markup, Tokyo, 2 nights, USD, anonymous:
| Property | exclusive | inclusive | hidden |
|---|---|---|---|
| The Tokyo EDITION, Ginza | 2228.68 | 2824.34 | 26.7% |
| Kimpton Shinjuku Tokyo By IHG | 1269.16 | 1654.27 | 30.3% |
| Tokyu Stay Shinjuku | 625.11 | 756.38 | 21.0% |
| Hotel Sunroute Plaza Shinjuku | 391.46 | 473.66 | 21.0% |
| The Prince Park Tower Tokyo | 2189.46 | 2769.68 | 26.5% |
| Hotel Metropolitan Tokyo Ikebukuro | 341.24 | 424.14 | 24.3% |

## Currency control gotcha (spec-critical)
Currency is set by TWO independent request fields that must agree:
1. `CitySearchRequest.searchRequest.searchCriteria.currency`
2. `PricingSummaryRequest.pricing.currency`
Setting only (1) silently returns the geo-default currency (IDR from an ID-origin
IP) while appearing to succeed. Neither the `CR-Currency-Code` header nor the
`agoda.version.03` cookie `CurLabel` overrode it. The generated client MUST set both.

## Response envelope (citySearch)
`data.citySearch.{ properties[], featuredPulseProperties[], searchResult, searchEnrichment, aggregation }`
- `properties[].content.informationSummary.displayName` — property name
- `properties[].propertyId` — stable id
- `properties[].pricing` — see pricing model above; also `pointmax`, `benefits`,
  `isEasyCancel`, `isInsiderDeal`, `priceChange`, `childPolicy`, `suppliersSummaries`
- `searchResult.{sortMatrix, searchInfo, urgencyDetail, histogram, nhaProbability}`
- `aggregation.matrixGroupResults` — filter facets

## Auth signals
- Public surfaces (search, autocomplete, reviews, property detail) need NO auth.
- Authenticated surfaces observed on the logged-in session: `window.params` carries
  `vipProgress`, `loyaltyProfileInfo`, `rewardsMember`, `promoWalletResult`,
  `isPointsMaxEligible`, `isLoyaltyEarnEnabled`, `isLoyaltyCashEnabled`.
- `searchCriteria.isUserLoggedIn` + `searchContext.memberId` change pricing =>
  member/VIP rates are a real, measurable delta vs anonymous.
- Cookie names (values never captured): `agoda.version.03` (DLang, CurLabel),
  `agoda.search.01`, `agoda.prius` (PointsMaxTraffic), `agoda.user.03`,
  `agoda.cid`, `agoda.familyMode`, `ASP.NET_SessionId`, `t_pp`, `t_rc`.
- Request headers observed: `AG-LANGUAGE-LOCALE`, `AG-PAGE-TYPE-ID`, `AG-CID`,
  `AG-REQUEST-ID`, `AG-CORRELATION-ID`, `AG-ANALYTICS-SESSION-ID`,
  `AG-REQUEST-ATTEMPT`, `AG-RETRY-ATTEMPT`, `CR-Currency-Code`, `CR-Currency-Id`,
  `AG-Language-Id`, `x-gate-meta`. Header VALUES were never recorded.

## Caveats
- `authenticated_bookings_not_captured`: the `/account/bookings.html` page was
  visited but produced no distinct XHR within the capture window; the trips surface
  is likely SSR (`window.params`) like Booking.com's. Modelled from the observed
  `/bff/trips/save-to-trip/retrieve` call plus SSR params keys, not from a fully
  observed populated-state response.
- `city_ids_are_opaque`: cityId must be resolved via autocomplete; 13170 = Ho Chi
  Minh City, 5085 = Tokyo, 9395 = Bangkok. There is no public city-id table.
- `service_worker_present`: scope `/js/assets/cronos/Assets/massaging-client` only;
  unrelated to data endpoints. It was unregistered during capture and re-registers
  on the user's next normal visit.
- `competitor_staleness`: birariro/agoda-review-mcp extracts hotelId from a
  `script-initparam` element that no longer exists on the plain-HTTP property page.

## PII handling
All artifacts in this directory were scrubbed post-capture: UUIDs, IPv4 addresses,
memberId, rawUserId, searchId, correlationId and any cookie header values were
replaced with REDACTED placeholders. Verified: zero residual UUID/IP matches.
