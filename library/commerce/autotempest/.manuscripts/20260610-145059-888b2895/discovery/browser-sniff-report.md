# AutoTempest Browser-Sniff Discovery Report

## Reachability
- Mode: standard_http (plain curl works browser-free; HTTP 200, real JSON, NO Cloudflare/bot challenge on data endpoints)
- Required request headers: User-Agent (Chrome), `x-requested-with: XMLHttpRequest`, `referer: https://www.autotempest.com/`, `accept: application/json, text/javascript, */*; q=0.01`
- Auth: NONE (public search)

## Request signing (custom token — NOT auth)
`/queue-results` requires a `token` query param or returns `{"status":-2,"errors":["Invalid token."]}`.
- token = sha256_hex( decodeURIComponent($.param(params)) + SALT )
- SALT (hardcoded in bundle module 14266, export `hc`/`Fe`): `d8007486d73c168684860aae427ea1f9d74e502b06d94609691f5f4f2704a07f`
- Hash: CryptoJS SHA-256 (standard), lowercase hex.
- Param string = the request query string (minus `&token=...`), URL-decoded (so `%7C`→`|`, `%20`/`+`→space).
- VERIFIED: server validates token against RECEIVED param order — any consistent order works (curl test with custom order returned 15 real listings). The CLI builds the query, hashes the decoded form + salt, appends token.

## Endpoints
| Method | Path | Params | Notes |
|---|---|---|---|
| GET | /queue-results | make, model, zip, radius, originalradius, sort, sites=<code>, deduplicationSites, rpp, [searchAfter], token | Per-source listing fetch. Called once per source code. Returns {status, results[], searchAfter, searchAfterMismatch}. status 0/1=ok, -2=bad token. |
| GET | /api/facebookMarketplace | (same as queue-results, sites=fbm) | FB Marketplace source |
| GET | /api/searchtempest/direct | (same + keywords, make_moved, clBundleDuplicates) | SearchTempest source |
| GET | /sh | full filter set | Saved/recent searches state for a search |
| GET | /api/get-makes | popularMakes=true | {popularMakes:[[slug,Display],...]} |
| GET | /api/get-models/{make} | popularModels=true | {popularModels:[[slug,Display,_,yrMin,yrMax,bool],...]} |
| GET | /api/truecar/makes-models | - | TrueCar makes/models |
| GET | /xhr/get_vspec_url | site=kbb, make, model | KBB vehicle spec URL |

## Source codes (window.sitestosearch)
te=AutoTempest core, hem=Hemmings, cs=CarSoup, cv=Carvana, cm=Cars.com, eb=eBay, ot=Others(CarGurus/TrueCar/CarsDirect/Autobytel/dealer feeds via vast/dt/cd/cgu/tc), fbm=Facebook Marketplace, st=SearchTempest, extended.
deduplicationSites default = te|hem|cs|cv|cm|eb|ot|extended|fbm|st

## Search params (window.defaultSearchArgs — full canonical set + defaults)
make(""), model(""), make_kw, model_kw, trim_kw, keywords, zip(""), radius(-1), minradius(-1), originalradius(-1),
minprice(-1), maxprice(-1), minyear(-1), maxyear(-1), minmiles(-1), maxmiles(-1),
bodystyle(any), drive(any), fuel(any), transmission(any), cylinders(any), doors(any),
exterior_color(any), interior_color(any), title(any), saletype(any), saleby(any),
domesticonly(1), haspic(0), titlesonly(0), sort(best_match), rpp, sites, deduplicationSites, searchAfter([]), search_origin(web), simplified(0)

## Listing response fields (per result)
id, vin, externalId, title, make, model, backendModel, year, trim, price (string "$30,497"),
mileage (string "24,755"), location, locationCode (zip), countryCode, distance, dealerName, sellerType,
url (listing link), img/imgSource/imgFallback, sitecode (source), backendSitecode, sourceCodeString,
vehicleTitle ("Clean"/etc), vehicleTitleDesc, listingType, dealGauge/dealGaugeClass, isHotCar/hotCar,
currentBid/bids (eBay auctions), priceHistory[{date,mileage,price,trend}], listingHistory (JSON string),
priceRecentChange, priceHistoryDiff, date, ctime, phone, feesIncluded, pendingSale, detailsShort/Mid/Long/ExtraLong

## Pagination
Response `searchAfter` (array of numbers) → pass as next request's `searchAfter` param (URL-encoded JSON array).

## Replayability verdict
SHIPPABLE plain-HTTP CLI. Token signing is hand-coded (SHA-256 + known salt). No browser at runtime.
