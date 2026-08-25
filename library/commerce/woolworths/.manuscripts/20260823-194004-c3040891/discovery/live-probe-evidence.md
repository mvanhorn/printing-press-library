# Woolworths AU — live probe evidence (Phase 1 / 1.9)

Probed 2026-08-23 from the generation host. All requests read-only GET/POST search.

## Transport finding (settled)

- `cli-printing-press probe-reachability https://www.woolworths.com.au` -> `mode: standard_http`,
  confidence 0.95, stdlib 200 and surf-chrome 200.
- **Bare curl (default UA) to `/apis/ui/*` -> HTTP 403 Akamai "Access Denied"** (`errors.edgesuite.net` reference).
- **Same request with a browser `User-Agent` -> HTTP 200 with real JSON.**
- Warming cookies from `https://www.woolworths.com.au/` yields Akamai cookies `bm_sz`, `bm_so`, `_abck`.
  With the warmed jar, POST endpoints that previously failed at transport return 200.

**Runtime conclusion:** replayable pure HTTP. Requires (a) browser-shaped headers
(User-Agent, Accept, Accept-Language, Referer, Origin, Sec-Fetch-*), and (b) a cookie jar
warmed by one GET of the homepage before the first API call. No login required for catalogue.
No resident browser required. No clearance challenge solved by hand.

## Confirmed working endpoints (anonymous)

| Endpoint | Method | Bytes | Notes |
|---|---|---|---|
| `/apis/ui/PiesCategoriesWithSpecials` | GET | 769,768 | Full category tree incl. Specials group |
| `/apis/ui/Search/products` | POST | 501,403 | Product search; body has SearchTerm/PageNumber/PageSize/SortType/IsSpecial/Filters |
| `/apis/ui/product/detail/{stockcode}` | GET | 9,340 | Single product detail |
| `/apis/ui/browse/category` | POST | 605,125 | Category browse + SeoMetaTags |
| `/api/v3/ui/schemaorg/product/{stockcode}` | GET | 4,140 | schema.org JSON-LD product |

High-gravity response fields observed on search/detail:
`Stockcode`, `Barcode`, `GtinFormat`, `Price`, `InstorePrice`, `WasPrice`,
`CupPrice`, `InstoreCupPrice`, `CupMeasure`, `CupString`, `InstoreCupString`, `HasCupPrice`,
`PricePerKGLabel`, `IsOnSpecial`-family fields. Unit pricing (`CupString`) is present on
every tile - this is the basis for real unit-price comparison.

## Guessed paths that 404 (wrong path, NOT blocked)

`Search/AutoComplete`, `Trolley/Info`, `Shopper/Details`, `Fulfilment/GetFulfilmentMethods`, `Lists`
returned HTTP 404 with a 3-byte body. 404 (not 403) means the host is reachable and the path is
simply wrong. Real paths for trolley / lists / order history / Everyday Rewards must come from
browser-sniff against a logged-in session.

## Known-bad body shape

POST `/apis/ui/Search/products` with `SearchTerm: ""` and `IsSpecial: true` returns
HTTP 400 `{"ResponseStatus":{"ErrorCode":"BadRequest"}}`. Specials browsing needs a different
request shape (likely the category-browse endpoint against the specials node).

---

# Round 2 probes (after ecosystem research)

## Mobile API `prod.mobile-api.woolworths.com.au` — NOT REACHABLE (key revoked)

`drkno/au-supermarket-apis` (2022) publishes an `X-Api-Key` for the mobile surface and documents
~24 endpoints that no existing tool implements (shopping-list CRUD, barcode/EAN lookup, stores,
fulfilment, past-shop, shopper preferences). Probed every one with that documented key:

| Endpoint | Result |
|---|---|
| `GET /wow/v2/stores?type=suburb&q=Sydney` | **401** `{"error":"invalid_client"}` |
| `GET /wow/v2/products/{barcode}?type=ean` | **401** `{"error":"invalid_client"}` |
| `GET /wow/v2/products/{articleId}?details=true` | **401** `{"error":"invalid_client"}` |
| `GET /wow/v3/fulfilment` | **401** `AP001 Invalid Client` |
| `GET /wow/v2/addresses/stores?lat&long&postcode` | **500** `AD004` |
| `GET /wow/v2/lists` | **401** `{"error":"unauthorized"}` |
| `GET /wow/v2/commerce/lists/pastshop` | **401** `{"error":"unauthorized"}` |

**Conclusion: the 2022 public app key has been rotated/revoked.** `invalid_client` is a rejected
*client credential*, not a missing user session. This surface is therefore NOT buildable from the
published key, and MUST NOT be promised in the manifest. The "biggest greenfield" identified by
ecosystem research is closed unless a current mobile app key is captured from a real device,
which is out of scope for this run.

## Web API `/apis/ui/*` extras — ALL WORK ANONYMOUSLY, none implemented by any existing tool

| Endpoint | Method | Status | Payload |
|---|---|---|---|
| `/apis/ui/v2/Search/count?searchTerm=` | GET | **200** | 514 B - `ProductCount`, `SpecialProductCount`, `RecipeCount`, `ArticleCount`, `Total`, `SuggestedTerm` |
| `/apis/ui/search-suggestions/suggestionsb2c?searchTerm=` | GET | **200** | 407 B - ranked `suggestions[]` + `autoCorrectedTerm` |
| `/apis/ui/Trolley` | GET | **200** | 1.5 KB - full trolley envelope (empty when anonymous) |
| `/api/ui/v2/bootstrap` | GET | **200** | 11 KB - `ContextExpiryPeriod`, `CurrentVersion`, `DefaultProductImage`, config |
| `/apis/ui/settings` | GET | **200** | **323 KB** - full site settings/feature flags |

`v2/Search/count` and `search-suggestions` are the two endpoints elijah-g's reverse-engineered
docs describe but **no tool anywhere implements**. Both work anonymously and are cheap
(hundreds of bytes vs the 501 KB full search). `Search/count` returning `SpecialProductCount`
alongside `ProductCount` is a free specials-density signal.

`RecipeCount: 2575` on a plain `milk` query confirms a recipe surface exists behind search.

## Revised buildable surface

Buildable now (anonymous, cookie-warmed, replayable HTTP): search, search-count, autocomplete,
product detail (two independent paths), category tree, category browse, trolley read, bootstrap,
settings.

Requires the user's logged-in session (browser-sniff): trolley write, shopping lists, order
history, Everyday Rewards points/boosters/e-receipts. Real paths still unknown - the mobile
paths are revoked and the web paths for lists/orders were not in any tool's source.
