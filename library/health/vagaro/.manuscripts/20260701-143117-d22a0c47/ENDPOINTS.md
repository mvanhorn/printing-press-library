# Vagaro CLI — Confirmed Endpoint Reference (for Phase 3 build)

Base: `https://www.vagaro.com`. Send a Chrome `User-Agent` (already a required header in config). Most reads are PUBLIC (no auth). ASP.NET responses wrap payload in `{"d": <json-or-json-string>}` — unwrap `.d` (it is sometimes a JSON *string* that must be parsed again). Dates are OData `/Date(ms)/` → use `cliutil.ParseODataDate`.

Auth (only for `me`/account + booking submit): **composed** — send header `s_utkn: <jwt>` PLUS the session cookie jar. `auth login --chrome` already imports both (generated). Test account loginUserID = <REDACTED-USER-ID>.

## Slug → businessID resolution (REQUIRED first step for websiteapi calls)
`GET /{slug}` returns SSR HTML. The businessID appears in image URLs as `_<providerId>_<businessID>_` and in `businessid`-ish fields. Parse the numeric businessID (e.g. Central Barber `centralbarber` → 93458). Cache slug→businessID in the store.

## PUBLIC endpoints (no auth), POST JSON to /us02/websiteapi/homepage/ unless noted

### Services — getshopdetailcompositeservice
body: `{"businessID":"93458","loginUserID":"","bookText":"Book"}`
resp `.d` (or direct): `{"ServiceProviders":..., "Services":[{"ServiceCategoryID","ServiceCategoryTitle","ServiceList":[{"ServiceID","ServiceTitle","PriceText":"$52.00","ServiceLevel",...}]}]}`

### Staff/providers — getshopdetailcompositestaff
body: `{"businessID":"93458","loginUserID":"<or empty>","bookText":"Book"}`
resp: provider list (ServiceProviderID, name, etc.). Providers e.g. Ronnel Getz, George Kuhar.

### Reviews — getreviews
body: `{"currentPageIndex":1,"pageIndex":1,"PageSize":20,"businessID":"93458","SortType":"1","FilterType":"1","ReviewGUID":"","ServiceProviderId":"","ReviewID":0}`
(ServiceProviderId optional — filter reviews to one provider)

### Availability / SLOTS — getavailablemultiappointments  (CONFIRMED replayable, public)
body: `{"lAppointmentID":"","businessID":"93458","csvServiceID":"34098477","csvSPID":"<providerId CSV, empty=any>","AppDate":"Fri Jul-24-2026","StyleID":null,"isPublic":true,"isOutcallAppointment":false,"strCurrencySymbol":"$","IsFromWidgetPage":"false","isFromShopAdmin":false,"isMoveBack":false,"BusinessPackageID":0,"PromotionID":"","TIME_ZONE":-8,"CUSTOM_DAY_LIGHT_SAVING":true,"DAY_LIGHT_SAVING":"Y","CountryID":1,"CustomerTimezone":-8,"Customerzoneid":"","CustomerCulture":"1","CustIsDayLightSaving":true}`
- `csvServiceID`: service id(s), comma-sep. `csvSPID`: provider id(s), comma-sep, empty = any provider.
- `AppDate` format: `Ddd Mon-DD-YYYY` (e.g. `Fri Jul-24-2026`). Returns the week starting at AppDate.
- resp `.d`: HTML/JSON with time slots (e.g. "10:00 AM","01:00 PM"). Parse slot times + provider + date. NOTE: response may be an HTML fragment string inside `.d` — extract `\d{1,2}:\d{2} [AP]M` slots and their provider/date context.

### Search / listings — GET /listings/{service}/{city--state}  (SSR JSON-LD)
Extract `<script type="application/ld+json">` → `ItemList.itemListElement[].item` (LocalBusiness: name, url=slug, telephone, priceRange, address{addressLocality,addressRegion,streetAddress,postalcode}, aggregateRating{ratingValue,ratingCount}). GEO NOTE: from a normal user IP this scopes to their metro; the city--state slug is advisory (server uses IP). Document this.

### Classes — POST /websiteapi/homepage/getupcominglivestreamclasses (already in spec, works)
resp `.ObjLiveStreamResponse[]`.

### Public API (GET, https://api.vagaro.com/US02/api/v2/public/)
- addonservice/getaddonservice  body `{"ServiceId":34098477,"BusinessID":93458,"ServiceProviderID":0,"IsBundle":0,"IsOnline":0,"LastMinDealHours":3}`
- promotion/getallpromotiondetailsbybusinessid

## AUTH endpoints (composed: s_utkn header + cookie jar)
### My appointments — POST https://api.vagaro.com/us02/api/v2/myaccount/purchases/appointments
body: `{"pageSize":12,"pageNumber":1,"pastAppointment":false,"myOrSharedAppointments":1,"device":"Website","module":"MyAccount","version":"2.5.3","brandedApp":false,"merchantId":"","multiLocation":false,"appNo":null}`
resp: `{status,responseCode,message,data:[...]}` (response_path: data). pastAppointment:true = history.
ALSO: POST /us02/websiteapi/homepage/getupcomingappintmentdata (session-cookie appointments, alt source).

### Booking SUBMIT — NOT captured (Angular checkout widget resisted automation).
For `book`: (1) verify slot via getavailablemultiappointments, (2) with --confirm attempt submit — capture the real endpoint during a supervised live test-book, else fall back to printing the preselected book-now URL `https://www.vagaro.com/{slug}/book-now`. Gate per side-effect rules (print by default, --confirm to act, cliutil.IsVerifyEnv short-circuit).

## Test fixtures (Central Barber, Seattle WA)
- slug: centralbarber, businessID: 93458
- Skin Fade serviceID: 34098477 ; Men's Haircut serviceID: 9433955
- providers: Ronnel Getz, George Kuhar (a ServiceProviderId seen: 43931725)
- A date with availability: Fri Jul-24-2026
