# Airline URL captures

Source-of-truth log for the `airlineTemplates` map in `booking_urls.go`.
Each row documents the carrier's booking-search URL pattern, classification
(`prefill` vs `landing`), the date captured, and the source.

**Refresh procedure:** when a URL stops working, recapture by visiting the
carrier's booking page in a browser, submitting a sample search, and
observing the resulting URL. Update both this log and the `airlineTemplates`
map in `booking_urls.go`. Bump `applied_at` in `.printing-press-patches.json`.

## Classification

- **prefill**: URL params survive page load and pre-fill the carrier's booking form. User clicks once, lands on a search-results-ready page.
- **landing**: URL takes the user to the carrier's booking entry page; dates and route are not pre-filled. User clicks once, lands on the right airline's booking surface, may need to retype dates. Still useful because the user already chose this airline.

## Captures

Dated 2026-05-12. Sources: Lufthansa Developer Center, Delta visible URL params, mobile.southwest.com URL bar, carrier landing pages.

| IATA | Carrier | Kind | URL pattern | Source |
|---|---|---|---|---|
| AS | Alaska | landing | `https://www.alaskaair.com/search/flights?O={origin}&D={destination}&OD={depart}&RD={return}&A={pax}` | Observed booking-flow path; param names may not survive SPA hydration |
| AA | American | landing | `https://www.aa.com/booking/find-flights?from={origin}&to={destination}&departDate={depart}&returnDate={return}&adultPassengerCount={pax}&type={trip_type}` | Param names from existing PR #512 entry; SPA; no public deeplink spec |
| DL | Delta | prefill | `https://www.delta.com/flightsearch/book-a-flight?tripType={trip_type_dl}&originCity={origin}&destinationCity={destination}&departureDate={depart}&returnDate={return}&paxCount={pax}` | Directly observable URL params; `trip_type_dl` = `ROUND_TRIP`/`ONE_WAY` |
| UA | United | landing | `https://www.united.com/en/us/fsr/choose-flights?f={origin}&t={destination}&d={depart}&r={return}&px={pax}&tt={trip_type_int}&sc=7&taxng=1&clm=7` | Existing PR #512 entry; SPA backed by FetchFlights internal API |
| B6 | JetBlue | landing | `https://www.jetblue.com/booking/flights?from={origin}&to={destination}&depart={depart}&return={return}&isMultiCity=false&noOfRoute=1&adults={pax}` | Existing PR #512 entry; SPA |
| WN | Southwest | prefill | `https://www.southwest.com/air/booking/?originationAirportCode={origin}&destinationAirportCode={destination}&departureDate={depart}&returnDate={return}&adultPassengersCount={pax}&tripType={trip_type_wn}` | Param names from mobile.southwest.com URL; `trip_type_wn` = `oneway`/`roundtrip` |
| F9 | Frontier | landing | `https://booking.flyfrontier.com/` | Navitaire common; URL params not stable |
| AC | Air Canada | landing | `https://www.aircanada.com/aco/en_us/aco-booking-flights/flight-search?orgCity1={origin}&destCity1={destination}&date1={depart}&date2={return}&numAdults={pax}` | Existing PR #512 entry; SPA |
| BA | British Airways | landing | `https://www.britishairways.com/travel/fx/public/en_us?eId=120001&depAirport={origin}&arrAirport={destination}&outboundDate={depart}&inboundDate={return}&adults={pax}` | Existing PR #512 entry; SPA |
| AF | Air France | landing | `https://wwws.airfrance.us/` | SPA; AF/KLM developer portal is API-only |
| KL | KLM | landing | `https://www.klm.com/en-us/flights` | SPA; same group as AF |
| LH | Lufthansa | prefill | `https://www.lufthansa.com/deeplink/partner?airlineCode=LH&originCode={origin}&destinationCode={destination}&travelDate={depart}&returnDate={return}&travelers=adult={pax}` | Lufthansa Developer Center, Shopping_Links_Search |
| LX | Swiss | prefill | `https://www.lufthansa.com/deeplink/partner?airlineCode=LX&originCode={origin}&destinationCode={destination}&travelDate={depart}&returnDate={return}&travelers=adult={pax}` | Lufthansa Group spec; resolves to swiss.com after redirect |
| IB | Iberia | landing | `https://www.iberia.com/us/flight-search-engine/` | SPA |
| VS | Virgin Atlantic | landing | `https://www.virginatlantic.com/en-US` | SPA |
| SK | SAS | landing | `https://www.flysas.com/en` | SPA |
| AY | Finnair | landing | `https://www.finnair.com/us/en` | SPA; only managebooking deeplink is documented |
| EI | Aer Lingus | landing | `https://www.aerlingus.com/html/dashboard.html` | SPA |
| DE | Condor | landing | `https://www.condor.com/us/` | SPA; "tcibe" booking engine |
| EK | Emirates | landing | `https://www.emirates.com/english/book/` | SPA |
| QR | Qatar Airways | landing | `https://booking.qatarairways.com/nsp/views/deepLinkLoader.xhtml` | Deeplink endpoint exists; parameter names not public |
| EY | Etihad | landing | `https://www.etihad.com/en-us/book` | SPA |
| SQ | Singapore Airlines | landing | `https://www.singaporeair.com/en_UK/us/home` | SPA; developer.singaporeair.com is API-only |
| BR | EVA Air | landing | `https://booking.evaair.com/flyeva/eva/b2c/booking-online.aspx?lang=en-us` | Legacy ASP.NET; lang param confirmed |
| CX | Cathay Pacific | landing | `https://www.cathaypacific.com/cx/en_US.html` | SPA; flights.cathaypacific.com is a separate marketing-promo subdomain |
| KE | Korean Air | landing | `https://www.koreanair.com/booking/search` | SPA |
| NH | ANA | landing | `https://www.ana.co.jp/en/us/plan-book/` | SPA; splits domestic vs international |
| JL | JAL | landing | `https://www.jal.co.jp/jp/en/inter/booking/` | International booking flow; domestic flow lives separately |
| TG | Thai Airways | landing | `https://www.thaiairways.com/en/book/booking.page` | SPA, AEM .page suffix |
| PG | Bangkok Airways | landing | `https://www.bangkokair.com/flight/booking` | SPA |
| HU | Hainan | landing | `https://www.hainanairlines.com/US/US/Search` | SPA |
| CI | China Airlines | landing | `https://www.china-airlines.com/us/en/booking/book-flights` | SPA |
| OZ | Asiana | landing | `https://flyasiana.com/C/US/EN/index` | Legacy .do servlets at m.flyasiana.com |
| JX | Starlux | landing | `https://www.starlux-airlines.com/en-US/booking/book-flight/search-a-flight` | SPA |
| ET | Ethiopian | landing | `https://www.ethiopianairlines.com/us/book/booking/flight` | SPA |

## Notes on the prefill-classified entries

- **DL**: visited `https://www.delta.com/flightsearch/book-a-flight?tripType=ROUND_TRIP&originCity=SEA&destinationCity=LAX&departureDate=2026-09-15&returnDate=2026-09-22&paxCount=2` in browser 2026-05-12. Page navigates to the booking flow with the expected URL params preserved.
- **LH / LX**: officially documented by [Lufthansa Developer Center / Shopping_Links_Search](https://developer.lufthansa.com/docs/read/api_partner/customer_deeplinks/Shopping_Links_Search). The `airlineCode` param routes the same template to LH or LX.
- **WN**: param names from `mobile.southwest.com` URL bar; the desktop site uses identical query params.

## Notes on landing-classified entries

Each landing URL has been spot-checked to confirm it resolves to a page on the carrier's booking surface. URL params (where present in the table) may or may not pre-fill the form, but the user still lands on the right airline's booking entry — a one-tap improvement over "open Google Flights and search again."

When a future capture upgrades a landing entry to prefill, change `kind` in `booking_urls.go` and re-verify by visiting the templated URL in a browser. Document the new pattern in the table above with the new source / date.
