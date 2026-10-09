# Crowd-sniff report: japan-theme-parks (2026-10-09)
Approved by pre-answered user gate ("approved if useful").

1. npm Packages Analyzed: `cli-printing-press crowd-sniff --api queue-times.com` returned "downloads API returned status 400" and no endpoints (rc=4). No npm SDK yielded endpoints.
2. GitHub Repos Searched (gh, authenticated): "ticketPopup tokyodisneyresort" (0 hits), "is-notSales" (0 hits), "mobile-service.usj.co.jp" (hits: romualdoag/parksapi-mcp upstream/src/parks/usj/universalstudiosjapan.ts uses `https://mobile-service.usj.co.jp/api/Venues/${VENUE_ID}/Hours?endDate=...`; xanke/themeparks-map proxies the same host).
3. Endpoints Discovered: GET /api/Venues/{venueId}/Hours (code-search, 2 sources) — matches the browser capture.
4. Base URL Resolution: https://mobile-service.usj.co.jp (code-search + browser capture).
5. Auth Patterns: none needed for Hours (parksapi passes no key for this call in the matched line; our curl replay without headers returned 200).
6. Parameter Name Evidence: `endDate` in MM/DD/YYYY form (browser capture `endDate=10/09/2027`; community `endDate=10/01/2026`).
7. Coverage Summary: crowd-sniff adds confidence, no new endpoints.
