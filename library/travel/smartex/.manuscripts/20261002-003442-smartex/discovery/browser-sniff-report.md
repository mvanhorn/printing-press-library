# Browser discovery report

## User goal flow
Resolve a dated Shinkansen route fare and authoritative planning constraints. Public route input → Shinkansen section selection → class selection → dated fare comparison all completed. Reservation login inspected, maintenance observed; authenticated seat flow excluded by user scope. Secondary flow JR official basic timetable publication and luggage rules completed.

## Pages and interactions
See project evidence/native-browser.md and MAX-HANDOFF.md for chronological actual native Chrome actions. Chrome extension was the working native backend after unavailable IAB. Only fresh isolated tabs. Station Tokyo/Shin-Osaka and date2026-10-02; Green and reserved results read. JR Travel Information→Timetable followed from rendered links.

## Configuration
Native Browser Use, no proxy, session transfer, cookies, credential headers or JS interception. HAR artifact is explicitly a reconstruction combining observed public page URLs with verified cookie-free HTTP response captures, not a raw browser-export HAR. Timings unknown. Runtime plain HTTP HTML extraction. The legacy EUC-JP fare form is custom code, separate from generated GET reference endpoints.

## Endpoints
GET smart-ex.jp public service, booking-window, baggage, change/refund, boarding, five product pages: HTTP200 text/html, public. GET global.jr-central.co.jp basic timetable/baggage/Nozomi pages: HTTP200 text/html, public. POST unchin-navi.jp/cgi-bin/plusex/tokai_exic.cgi: HTTP200 EUC-JP HTML, public, fare calculation only.

## Traffic analysis
Generated GET HTML extraction is based on full response captures. No JSON API is claimed. Fare field spelling and class encoding are preserved in native-browser.md and fare-step1.html. Exact inventory login is a separate maintenance/login surface. No challenge,429 or credential replay occurred. Code must validate form/result route,date,class and active one-way rows.

## Coverage
Public planning sources covered. Exact departures/seats, accounts, payment, bookings and reservation changes are outside user-approved scope. Timetable PDFs are8-page outlined graphics: parsed schedules require additional validation. Discount route matrices and exclusions remain unknown unless fetched explicitly.

## Response samples
Full public HTML captures in discovery; representative fare HTML fare-step1.html4352bytes and fare-reserved.html24543bytes. Base sources and status/size in proofs/source-fetches.json and extra-fetches.json. Binary PDFs retained as research only.

## Rate limiting and authentication
No429 observed, bounded three concurrent source fetches and serial fare workflow. No authenticated session used.
