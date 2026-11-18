# EDEKA browser-sniff report (2026-11-17)

## 1. User Goal Flow
- Goal: order groceries for pickup from one EDEKA market (doerflinger-ulm.edeka.shop) without placing the order.
- Steps completed: (1) market finder JSON on edeka.shop; (2) market home; (3) offers `/Angebote/`; (4) search `milch` and the elastic suggester; (5) category page; (6) product detail; (7) add to basket from the search grid, raise quantity, add a second product; (8) basket page; (9) open the slot calendar, pick the first free pickup slot, confirm it; (10) checkout step 1 address (`cl=user`, submitted unchanged); (11) payment step (`cl=payment`, chose cash); (12) order review (`cl=order`) - stopped before "JETZT KAUFEN".
- Steps skipped: final order (not authorized), order cancel (no order exists; the shop shows no cancel UI), recipes (the shop has none).
- Secondary flows: account order history (empty), shopping lists (Einkaufszettel, empty), offers page 2, search page 2.
- Coverage: 12 of 13 planned steps (all except the final order).

## 2. Pages & Interactions
- https://edeka.shop/ and `index.php?cl=MxMarketSearch&fnc=searchMarketsByZipOrText&search=Ulm` - JSON market list.
- `https://doerflinger-ulm.edeka.shop/`, `/Angebote/`, `/Angebote/?pgNr=1`, `index.php?cl=search&searchparam=milch[&pgNr=1]`, `/Kuehlprodukte-EDEKA/Eier-Kaese-Molkereiprodukte/`, product page `/Kuehlprodukte-EDEKA/.../Weide-Milch.html`.
- Search grid: clicked "In den Warenkorb" on the first tile, then "+" on the same tile; search `butter`, clicked "In den Warenkorb".
- `/warenkorb/` (basket); clicked the delivery-time box, tab "Selbstabholung", first free appointment, "Termin auswaehlen".
- "Zur Kasse" -> `cl=user` -> "Weiter zu den Zahlungsarten" -> `cl=payment`, chose "Barzahlung", "Weiter zum Bestellabschluss" -> `cl=order` (review only).
- `index.php?cl=account_order` (Bestellhistorie), `index.php?cl=AccountMultiWishList`.

## 3. Browser-Sniff Configuration
- Backend: the AWG agent browser (Chromium over CDP) driven by playwright-core scripts that record document/xhr/fetch request+response pairs. Pacing: one page at a time, 1.5-5 s waits, no 429 seen. Proxy envelope: not detected.

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | edeka.shop `/index.php?cl=MxMarketSearch&fnc=searchMarketsByZipOrText&search=` | 200 | application/json | public |
| GET | `<market>/index.php?cl=ith_oxelastic_suggester&elastic_type=json&term=` | 200 | JSON (text/html header) | public |
| GET | `<market>/index.php?cl=search&searchparam=&pgNr=` | 200 | text/html (product grid) | public |
| GET | `<market>/Angebote/[<Category>/]?pgNr=` | 200 | text/html (offer grid) | public |
| GET | `<market>/<Category-Path>/` | 200 | text/html (category grid + nav tree) | public |
| GET | `<market>/<...>/<Product>.html` | 200 | text/html (schema.org microdata) | public |
| POST | `<market>/widget.php?lang=0` body `cl=MxCalendarWidget` | 200 | text/html with embedded `window['pickupData']` JSON | public |
| GET | `<market>/index.php?cl=mx_pickup_controller&fnc=getMxDeliveryColText` / `getMxReservationPopUpData` | 200 | JSON | session |
| POST | `<market>/index.php` `cl=mx_pickup_controller&fnc=setCookie&data[date]&data[time]&data[type]&data[timespan][from|until|selected]&data[premium]&data[ignoreDeliveryArea]` | 200 | JSON `{"success":true}` | session |
| POST | `<market>/index.php` `cl=mx_pickup_controller&fnc=changeshipping&sShipSet=mxpickup` | 200 | empty | session |
| POST | `<market>/index.php` `cl=mx_pickup_controller&fnc=setDeliveryArea|unsetPickUpDate|setPickUpType` (from calendar.js) | - | JSON | session |
| POST | `<market>/index.php` `cl=alist&fnc=changeBasket&aid&anid&am&stoken&response=true` | 200 | JSON (`productCount`, `NewBruttoSum`) | session + stoken |
| GET | `<market>/widget.php?lang=0&cl=oxwminibasket` | 200 | text/html (basket count) | session |
| GET | `<market>/warenkorb/` (`cl=basket`) | 200 | text/html (items with `aproducts[...]`) | logged-in |
| POST | `<market>/index.php` `cl=user&fnc=changeuser` (address step) | 302 | - | logged-in + stoken |
| POST | `<market>/index.php` `cl=payment&fnc=validatepayment&paymentid=` | 302 | - | logged-in + stoken |
| GET | `<market>/index.php?cl=order` (review) | 200 | text/html, form `fnc=execute` with `sDeliveryAddressMD5`, `ord_agb=1` | logged-in |
| GET | `<market>/bestellhistorie/` (`cl=account_order`) | 200 | text/html | logged-in |

## 5. Traffic Analysis
- Protocols: `rest_json` and `html_scrape` (traffic-analysis.json). Reachability `standard_http` (0.65): no Cloudflare, Apache, no challenge pages.
- Auth signals: cookies only - `sid`, `sid_key`, `language`, `oxid_1`, `oxid_1_autologin`, `mxpickupdate`, `mxreservationid` (names only). CSRF: hidden form field `stoken` (session-bound) required for every `fnc=` call; without it the shop returns an HTML page.
- Parameter evidence: `aid`/`anid` = article id from the product tile form; `am` = absolute quantity (second call with `am=2` gave `productCount:2`); `searchparam` = search box name; `pgNr` = zero-based page; slot fields from calendar.js `onClickAppointment`.
- Warnings: the sniffer collapsed every OXID controller into `GET /index.php`; the spec is hand-authored from this report instead.

## 6. Coverage Analysis
- Exercised: markets, offers, search, suggester, category, product, basket, slots (pickup), checkout up to review, order history (empty), shopping lists (empty).
- Missed: delivery slots after choosing a delivery area (only the area list was captured), order detail/cancel (no orders on the account), weight articles (meat counter items need `persparam[Gewicht]` grams).

## 7. Response Samples
- Market search: `[{"oxid":"...","mxactive":1,"mxdomain":"doerflinger-ulm.edeka.shop","mxtitle":"EDEKA ...","mxstreet":"...","mxzip":"89077","mxcity":"Ulm","mxdelivery":1,"mxpickup":1,"mxexpress":0,"mxlat":"48.39","mxlng":"9.98","mxdeliveryplz":"89073, ...","mxopeningtime":"07:00 - 22:00",...}]`
- changeBasket: `{"success":true,"productCount":2,"productChangedBasketId":null,"message":false,"NewBruttoSum":3.18,"limit_message":null,"autoclosemessage":true}`
- Suggester product: `{"label":"Weide <mark>Milch</mark>","price":"1,59","category":"Produkte","type":"product","id":"5915407005MXST","link":"/Kuehlprodukte-EDEKA/.../Weide-Milch.html","basic_price":"1,59 € / 1l","isinbasket":false,"basketAmount":0,"amount_limit":-1}`
- Calendar slot: `{"available":true,"time":"15:00","timestamp":1796652000,"timespan":{"from":"15:00","until":"16:00","selected":"15:00 - 16:00"},"puffer":480,"slotPricelist":[],"pickupdategroupid":"..."}` keyed by date `YYYY-MM-DD`.

## 8. Rate Limiting Events
- None. About 70 requests over 15 minutes.

## 9. Authentication Context
- Authenticated session from the AWG agent browser (cookies shared in the browser context). Cookie replay validated outside the browser with Node https: account page with all cookies -> logged in; with no cookies -> login form; with only `oxid_1` -> logged in and a new `sid` that works on its own (silent renewal). Basket is saved per user (seen from a fresh session). Session state stayed in the owner-only session dir outside the run dir and is not archived. HAR has no cookie headers, session ids, CSRF tokens or personal pages.
