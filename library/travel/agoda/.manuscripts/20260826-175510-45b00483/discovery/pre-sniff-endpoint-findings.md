# Agoda pre-browser-sniff endpoint findings (plain HTTP, no auth)

## Verified working
| Endpoint | Method | Status | Notes |
|---|---|---|---|
| `/api/cronos/property/review/ReviewComments` | POST | 200 + real data | body `{hotelId,page,pageSize,sorting}`. Returns `comments[]`, `reviewPageUrl`, `reviewsSortOptions`. Verified live with hotelId=7708. |
| `/graphql` | POST/GET | 200 | Live GraphQL. Introspection DISABLED. |
| `/graphql/search` | POST/GET | 200 | Live GraphQL. Introspection DISABLED. |
| `/api/cronos/search/GetUnifiedSuggestResult` | GET | 200 (`{}`) | Needs correct params; likely destination autocomplete. |
| `/search?city=&checkIn=&los=&adults=&rooms=` | GET | 200 (389KB) | CSR shell only. No hotel data in HTML. |
| `/<slug>/hotel/<city>-<cc>.html` | GET | 200 (88KB) | CSR shell only. No JSON-LD, no script-initparam. |

## Real sort enum (from live ReviewComments response)
1 = Most recent, 2 = Rating high to low, 3 = Rating low to high, 7 = Most helpful
(NOTE: birariro/agoda-review-mcp mislabels 2/3 as POSITIVE/NEGATIVE.)

## Competitor staleness signal
birariro/agoda-review-mcp extracts hotelId from `<script data-selenium="script-initparam">`
matching `hotel_id=(\d+)`. That element is ABSENT from the current plain-curl property page
(Agoda moved to full CSR). That MCP's entry path is very likely broken today.

## Reachability
probe-reachability => mode=standard_http, confidence=0.95, needs_browser_capture=false,
needs_clearance_cookie=false. Both stdlib and surf-chrome returned 200.
CDN/WAF: Akamai (akamai-grn header). No active challenge observed.
Community reports (scraperly, Apr 2026): "Custom WAF + rate limiting", medium difficulty,
residential proxies advised at scale => RATE LIMITING is the real risk, not clearance.

## Cookie surface (from Set-Cookie on /search)
agoda.version.03 (CookieId, DLang=<locale>, CurLabel=<currency>)  <-- language + currency control
agoda.search.01 (SHist search history)
agoda.prius   (PriusID, PointsMaxTraffic)
agoda.cid, agoda.familyMode, agoda.landings, agoda.attr.fe, agoda.attr.03
ASP.NET_SessionId, t_pp, t_rc, agoda.user.03, agoda.analytics

## Property URL param vocabulary (from agoda-review-mcp test fixture)
countryId, finalPriceView=1, isShowMobileAppPrice, cid, numberOfBedrooms, familyMode,
adults, children, rooms, maxRooms, checkIn, childAges, numberOfGuest, travellerType,
currencyCode, los, searchrequestid, tspTypes, ds, tag
NOTE: `finalPriceView=1` is directly relevant to the all-in-price pain point.
