# Vagaro Browser-Sniff Discovery Report

Captured 2026-07-01 via chrome-devtools-axi (isolated Chrome Beta, geo=Port Orchard WA / San Francisco CA).

## Reachability
- mode: standard_http (probe reported browser_clearance_http but false-positive from embedded reCAPTCHA form widget)
- Public pages: HTTP 200 + full SSR content for Chrome UA, curl/8.1, and python-requests UAs.
- Incapsula present (visid_incap_*, incap_ses_*) but non-blocking during capture.

## Endpoints (replayable)
| Surface | Method Path | Format | Auth | Signing |
|---|---|---|---|---|
| Search results | GET /listings/{service}/{city--state} | HTML+JSON-LD ItemList | none | none |
| Business profile | GET /{business-slug} | HTML (BusinessID, reviews, breadcrumb JSON-LD) | none | none |
| Service menu | GET /{business-slug}/services | HTML (ServiceTitle/price/duration) | none | none |
| Availability summary | GET /{business-slug}/book-now | HTML (next available date) | none | none |
| Deals | GET /deals/{city--state} | HTML | none | none |
| Professionals | GET /professionals/{city--state} | HTML | none | none |
| Livestream classes | POST /websiteapi/homepage/getupcominglivestreamclasses | JSON (OData /Date()/) | none | none |

## Endpoints (AVOID — signed)
- POST /WebServices/MySampleService.asmx/PageMethodsProxyJson?token={Method}
  - Methods: GetMasterCategoryValues, GetAdminServices, getcitybylatlong, searchrelatedservice
  - Requires client-computed signing headers: h, val, k, i. Not replayable. SSR/websiteapi covers same data.

## Auth
- Public: none.
- Authenticated (own bookings/profile): cookie-based; user logged into Chrome. Not yet sniffed (pending scope decision).
- Booking: real mutation gated by Stripe/Affirm/Google-Pay payment iframes.

## JSON-LD structure (search)
schema.org ItemList > itemListElement[] > item (LocalBusiness): name, image, url (business slug), telephone, priceRange, address{locality,region,streetAddress,postalcode}, aggregateRating{ratingValue, ratingCount}

## Authenticated surface (captured with user's live session)
- Auth model: **composed** — JWT sent in custom header `s_utkn` (mirrors the `s_utkn` cookie, a ~30-day JWT) PLUS the full session cookie jar. `s_utkn` alone = 401; `s_utkn` + cookies = 200. `auth login --chrome` imports both.
- Response envelope on api.vagaro.com: `{status, responseId, responseCode, message, data}` → response_path: data.
- Confirmed authed endpoints:
  - POST api.vagaro.com/us02/api/v2/myaccount/purchases/appointments  body {pageSize,pageNumber,pastAppointment,myOrSharedAppointments,device:"Website",module:"MyAccount",version,brandedApp,merchantId,multiLocation,appNo}
  - POST api.vagaro.com/us02/api/v2/myaccount/account/profile (body required; 400 on empty)
  - GET  api.vagaro.com/us02/api/v2/myaccount/account/reviewinvoicecount
  - POST www.vagaro.com/websiteapi/homepage/getupcomingappintmentdata  (cleaner, session-cookie-only appointments source)
  - POST www.vagaro.com/websiteapi/homepage/getloginusergroupstring
- Account areas: /myaccount/{profile,appointments,bookmarks,reviews,packages,memberships,points,gift,paymentmethods,invoices,familyfriends}
- Test account had no appointment history (data:[] / null) — endpoints validated, shapes to refine during build with a populated account.

## Data model (confirmed)
- Booking = business × **service** × **provider/staff** × datetime. Service and provider are SEPARATE dimensions (Services tab vs Staff tab). Availability is PER-PROVIDER. slots/book/rebook must carry --provider.
- Business slug is the primary key (e.g. /centralbarber = Central Barber Shop, Seattle WA).

## GEO SCOPING (critical build constraint)
- SSR `/listings/{service}/{city--state}` geo is **server-side IP geolocation**, NOT the URL slug or proximitystate_v3 cookie. From THIS generation env (egress IP geolocates to San Francisco) every city/zip slug 301-redirects to san-francisco--ca, even with a correct-format proximitystate_v3 cookie.
- IMPLICATION: For a real end user running the CLI from their own machine, SSR listings WILL scope to THEIR location (their IP). The SF-everything behavior is an artifact of this env's IP.
- The search widget autocomplete (typing "Seattle, WA") DOES return geo-correct businesses by text — so an explicit-geo search API exists (api.vagaro.com public namespace or signed .asmx searchrelatedservice). Exact endpoint to be captured precisely at build time (fired behind widget interaction; not cleanly captured via network buffer).
- proximitystate_v3 cookie format: {"lat","long","countryid":"1","zip","city","state":<ABBREV e.g. WA>,"stateName":<FULL e.g. Washington>,"currencysymbol":"$",...} — state=abbrev, stateName=full (do not swap).

## Signed endpoints (AVOID)
- /WebServices/MySampleService.asmx/PageMethodsProxyJson?token={Method} needs client-computed h/val/k/i headers. Methods incl. GetFavoriteBusinesses, searchrelatedservice, getcitybylatlong. Use myaccount api / websiteapi / SSR instead.

## BOOKING / AVAILABILITY endpoints (captured via Performance API + interceptor, replayable)
All under www.vagaro.com/us02/websiteapi/homepage/ (session-cookie or public), ASP.NET {"d":...} envelope. NOT the signed .asmx.

- **Availability (slots) — PUBLIC, no auth**: POST /us02/websiteapi/homepage/getavailablemultiappointments
  body: {lAppointmentID:"", businessID:"93458", csvServiceID:"34098477", csvSPID:"<providerId CSV, empty=any>", AppDate:"Fri Jul-24-2026", StyleID:null, isPublic:true, isOutcallAppointment:false, strCurrencySymbol:"$", IsFromWidgetPage:"false", isFromShopAdmin:false, isMoveBack:false, BusinessPackageID:0, PromotionID:"", TIME_ZONE:-8, CUSTOM_DAY_LIGHT_SAVING:true, DAY_LIGHT_SAVING:"Y", CountryID:1, CustomerTimezone:-8, Customerzoneid:"", CustomerCulture:"1", CustIsDayLightSaving:true}
  CONFIRMED replays via curl (200, slot times in response.d). AppDate format: "Ddd Mon-DD-YYYY".
- Staff/providers: POST /us02/websiteapi/homepage/getshopdetailcompositestaff  body {businessID, loginUserID, bookText:"Book"}
- Services composite: POST /us02/websiteapi/homepage/getshopdetailcompositeservice
- Reviews: POST /us02/websiteapi/homepage/getreviews  body {currentPageIndex,pageIndex,PageSize,businessID,SortType,FilterType,ReviewGUID,ServiceProviderId,ReviewID}
- Online booking tab: POST /us02/websiteapi/homepage/getonlinebookingtabdetail
- Cart count: POST /us02/websiteapi/homepage/getcartcount
- Public API (GET, api.vagaro.com/US02/api/v2/public/):
  - addonservice/getaddonservice  body {ServiceId,BusinessID,ServiceProviderID,IsBundle,IsOnline,LastMinDealHours}
  - addonservice/getalladdonservicebybusinessnew
  - promotion/getallpromotiondetailsbybusinessid

## Captured IDs (Central Barber test)
- Central Barber businessID = 93458
- Skin Fade serviceID = 34098477
- A provider ServiceProviderId = 43931725
- Providers: Ronnel Getz, George Kuhar
- (auth) loginUserID = <REDACTED-USER-ID>
