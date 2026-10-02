# Browser discovery and replay evidence

## User goal flow and pages
Native Chrome (cua_repl), isolated task tab: Osaka–Beppu fee page -> source-linked Reserve0000/IndexEnglish -> Check fares without logging in -> Reserve1030. Selected October 15 2026, Osaka1→Beppu, Use by foot, one adult -> Proceed to Check Availability -> Reserve1020. Cabin availability links were not clicked. Three planned read-only interactions completed. Inspected form field names/options and HTML fare table. The source exposes room prices, status symbols, season/discount and ship/departure/arrival before the cabin-select reservation boundary.

## Configuration and capture provenance
User explicitly requested native-browser-first. Native Chrome was used first. CDP diagnostics later delayed and the anonymous session expired; no resident browser runtime is shipped. browser-sniff-capture.json contains scrubbed direct HTTP enrichment/replay of the native-observed request contract, not a native HAR. Native visible-response evidence is native-browser.json. The HTTP replay reproduced the same dated fare result. No proxy-envelope/GraphQL observed. Public static HTML pages are separately indexed in source-index.json.

## Endpoints and fields
GET /web/yoyaku/Reserve0000/IndexEnglish 200 HTML, POST /web/yoyaku/Reserve0000/Reserve 200 HTML final Reserve1030, POST /web/yoyaku/Reserve1030/MoveNext 200 HTML final Reserve1020. Session handshake tokens __RequestVerificationToken and req_t are carried in ephemeral memory only. Exact line codes and form category fields recorded in scrubbed samples. No account session or user credentials used. Never call Reserve1020/MoveNext.

## Traffic analysis and spec
The real Press browser-sniff analyzer accepted the capture and classified standard_http (0.65). Its endpoint_clusters are empty because this is a form-driven HTML surface rather than JSON API traffic. HTML body samples are populated. The authored internal YAML spec preserves observed public GET HTML sources using response_format: html and html_extract: page; typed planning commands will parse those pages and replay the observed anonymous form safely. No fabricated JSON response types.

## Coverage and response samples
Three Kansai–Kyushu routes/public timetable, fare, cabin, terminal pages fetched 200. portal-sample-2.html holds sanitized fare table: October15, season A, source Web DC(Partcal 5%), Kurenai, 19:05→next06:55, room fares including Private bed14620 JPY. Sample figures are test evidence only. No Hokkaido separate portal, roundtrip/Dangan, phone-only discounts, room selection, standby registration, personal data, payments or holds.

## Limits, rates and auth
No 429/protection gate observed. Each native interaction exceeded a second; HTTP enrichment issued only the three observed read-only flow calls. Native long delay expired a session; runtime starts fresh each call and enforces a total deadline. No authenticated session used; hidden values and cookie/header credentials are scrubbed before capture is written. Availability is a non-guaranteed source snapshot and quote does not reserve inventory.
