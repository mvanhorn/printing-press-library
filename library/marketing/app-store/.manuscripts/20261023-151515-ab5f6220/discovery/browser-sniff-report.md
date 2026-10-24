# App Store browser-sniff report

Run 20261023-151515-ab5f6220 · 2026-10-23 · target https://apps.apple.com/ (pre-approved, phase 0)

## 1. User Goal Flow
- Goal: see where apps rank in App Store charts, then drill into one app's full listing.
- Steps completed:
  1. Today page (full load) → iPhone landing, no API call on load beyond assets.
  2. Apps tab (SPA click) → `GET /api/apps/v1/editorial/us/groupings?name=apps`.
  3. Top Free Apps (link) → full document load `GET /us/iphone/charts/36?chart=top-free`, plus
     `GET /api/apps/v1/editorial/us/collections/{id}/contents`.
  4. Scroll to bottom (6 scrolls + 3 scroll-into-view) → no pagination request; the UI shows
     25 rows. The SSR page model already carries ranks 26-200 as ids
     (`segments[].nextPage.remainingContent`, 175 ids).
  5. Paid filter → full document load `GET /us/iphone/charts/36?chart=top-paid`.
  6. Category picker → links to `/us/iphone/charts/{genreId}?chart=...` for 30 genres plus Kids
     age bands (`ageBandId=`); no request until a link is followed.
  7. Games tab → `GET /api/apps/v1/editorial/us/groupings?name=games`; game charts at
     `/us/iphone/charts/6014?chart=top-free|top-paid`.
  8. App from Games (MONOPOLY GO!, SPA) → `GET /api/apps/v1/catalog/us/apps/1621328561`
     with the full product parameter set (below).
  9. Ratings & Reviews see-all (SPA) → `GET /api/apps/v1/catalog/us/apps/{id}?include=developer,reviews&sparseLimit[apps:reviews]=20&limit[reviews]=10`;
     20 reviews rendered; scrolling fired no further page.
  10. Developer link (SPA) → `GET /api/apps/v1/catalog/us/developers/305003647`.
  11. Search: typed "spoti" (no suggestion request), then "fy" + Enter → full document load
      `GET /us/iphone/search?term=spotify`.
  12. Platform menu → iPad → full document load `GET /us/ipad/search?term=spotify`.
  13. Replay for response bodies: full load of Candy Crush, then developer (SPA), see-all
      reviews (SPA) and a similar app (SPA): three catalog calls with bodies captured.
- Steps skipped: region switch (picker not opened; storefront is the first path segment,
  confirmed by probes); version-history and privacy modals (their data is inline in the
  product model; no separate request expected); Arcade, stories, in-app events.
- Secondary flows: game charts; search; platform switch.
- Coverage: 13 of 16 planned steps.

## 2. Pages & Interactions
1. https://apps.apple.com/us/iphone/today — open
2. Apps tab — click (SPA)
3. https://apps.apple.com/us/iphone/charts/36?chart=top-free — link click (document)
4. Charts page — scroll x9 (no request)
5. https://apps.apple.com/us/iphone/charts/36?chart=top-paid — "Filter by Paid apps" (document)
6. "All Apps" category picker — click (no request; 30 genre links read from DOM)
7. Games tab — click (SPA)
8. https://apps.apple.com/us/app/monopoly-go/id1621328561 — app link (SPA)
9. https://apps.apple.com/us/app/monopoly-go/id1621328561?see-all=reviews&platform=iphone — see-all (SPA)
10. https://apps.apple.com/us/developer/scopely-inc/id305003647 — developer link (SPA)
11. Search box — typed "spotify", Enter → https://apps.apple.com/us/iphone/search?term=spotify (document)
12. "App Store" platform menu → iPad → https://apps.apple.com/us/ipad/search?term=spotify (document)
13. https://apps.apple.com/us/app/candy-crush-saga/id553834731 — open; developer link,
    see-all reviews and "Candy Crush Soda Saga" similar app (SPA) for bodies

## 3. Browser-Sniff Configuration
- Backend: agent-browser 0.27.0, headless, session `appstore`, HAR via `network har start/stop`.
  browser-use 0.13.6 was installed but its CLI no longer exposes open/eval/scroll (now
  heredoc snippets through a persistent daemon attached to the user's running Chrome), so it
  was treated as incompatible per Cardinal Rule 1.
- Bodies: agent-browser HAR omits bodies; three representative catalog responses were read
  from the browser with `network request <id> --json`, the chart document body came from the
  same URL fetched earlier, and an enriched HAR was built for analysis (kept outside the
  manuscript tree; reviewer names present in raw bodies).
- Pacing: 1.2-3 s between interactions, about 25 interactions over about 8 minutes.
- Proxy-envelope detection: not detected. `/api/apps/v1/...` mirrors Apple Media API paths
  (one path per resource); the SPA sends an empty `Authorization: Bearer` header and Apple's
  edge supplies the token.
- Teardown: session closed with `close --all`; no agent-browser or headless browser process
  remained.

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type |
|---|---|---|---|
| GET | /us/iphone/today | 200 | text/html (SSR) |
| GET | /us/iphone/charts/{genreId}?chart={top-free,top-paid}[&ageBandId=] | 200 | text/html (SSR) |
| GET | /us/{iphone,ipad}/search?term= | 200 | text/html (SSR; robots-disallowed) |
| GET | /api/apps/v1/catalog/{cc}/apps/{id} | 200 | application/json (robots-disallowed) |
| GET | /api/apps/v1/catalog/{cc}/developers/{id} | 200 | application/json (robots-disallowed) |
| GET | /api/apps/v1/editorial/{cc}/groupings?name={apps,games} | 200 | application/json (robots-disallowed) |
| GET | /api/apps/v1/editorial/{cc}/collections/{id}/contents | 200 | application/json (robots-disallowed) |
| GET | /manifest.json | 200 | application/json (web app manifest) |

Product request parameters (step 8): `platform=iphone`,
`additionalPlatforms=appletv,ipad,iphone,mac,realityDevice,watch`, `extend=` (accessibility,
accessibilityDetails, ageRating, description, fileSizeByDevice, minimumOSVersion,
customScreenshotsByType, ...), `extend[apps]=distributionKind,isVerifiedForAppleSiliconMac`,
`include=alternate-apps,app-bundles,customers-also-bought-apps,developer,developer-other-apps,merchandised-in-apps,related-editorial-items,reviews`,
`include[apps]=app-events`, `views=categorizations,top-in-app-purchasables`,
`availableIn[app-events]=future`, `sparseLimit[apps:customers-also-bought-apps|developer-other-apps|related-editorial-items]=40`,
`limit[reviews]=8`, `l=en-US`. Catalog JSON carries `attributes.chartPositions`
(e.g. Candy Crush: top-free, genre 7003 Casual).

Known from research and main-session probes (not browser-observed), all 200 on 2026-10-23:
iTunes Lookup and Search API, legacy RSS charts (100-row cap), customer-reviews RSS (50 x 10
pages), app page and developer page SSR, see-all SSR (10 reviews), search hints plist.

## 5. Traffic Analysis
- Protocols: `ssr_embedded_data` (0.85), `rest_json` (0.75).
- Reachability: `standard_http` (0.65 from capture; `probe-reachability` on the proxy URL
  returned standard_http 0.95 with stdlib and Surf both 200).
- Auth signals: empty `Authorization: Bearer` on /api requests; no cookies required.
- Generation hints: none requiring browser auth, JS rendering or protected clients.
- Candidate commands from the analyzer: get_apps, get_developers, get_contents,
  list_groupings (editorial), list_manifest.json (noise).
- Warnings: `empty_payload` for body-less HAR entries (editorial groupings, collections,
  manifest), `weak_schema_evidence` for binary assets. Typed shapes exist for catalog apps and
  developers from the replayed bodies.

## 6. Coverage Analysis
Exercised: charts (free, paid, apps and games, genre picker), product, see-all reviews,
developer, search, platform switch. Matches the brief's top workflows 1-3 and 5.
Gaps: in-app events, Arcade, stories and Today editorial items (data seen only via the
editorial endpoints, bodies not captured); version history and privacy are inline in the
product model; Mac/TV/Watch/Vision pages not loaded (route pattern confirmed by the
platform menu); keyword-rank depth (search SSR next-page ids) not inspected.

## 7. Response Samples
Redacted per-endpoint samples written by the analyzer to
`research/app-store-browser-sniff-spec-samples/` (5 files; checked: no reviewer names or
review text). Shapes:
- catalog apps: `{data:[{id, type:"apps", href, attributes:{artistName, chartPositions:{appStore:{chart, chartLink, genre, genreName, position}}, ...}, relationships:{developer, reviews, customers-also-bought-apps, ...}}]}`
- catalog developers: `{data:[{id, type:"developers", attributes:{name, ...}, relationships:{...}}]}`
- chart SSR: `data[0].intent.$kind = ChartsPageIntent`; `data[0].data.segments[].shelves[0].items`
  (25 lockups: adamId, bundleId, title, subtitle, developerName, rating, ratingCount,
  ordinal, isEditorsChoice, offerDisplayProperties) and `segments[].nextPage.remainingContent`
  (175 `{id, type, href}`).

## 8. Rate Limiting Events
No 429 or 403 during the browser capture (about 14 /api calls and 8 documents at 1.2-3 s
spacing). Earlier, outside the browser: one 429 "API capacity exceeded" from the /api proxy
at 10:20:49Z on its second plain-curl call; RSS v2 host timed out and returned 504.

## 9. Authentication Context
No authenticated session used. Every observed surface answered anonymously.

## 10. Bundle Extraction
Not run. The storefront research sweep had already mapped client routes from community
sources; recovered Apple client source was used only as a route map, with no code copied.
