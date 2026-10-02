# Repark browser-sniff provenance

## User goal flow
Goal: find Repark parking and assess vacancy, declared fit and source price rules. Native isolated Chrome opened REP0022209; detail successfully rendered and reload observed GET 200. The native full CDP/reload invocation took 1,989 seconds. Following user instruction, further contract discovery uses already observed provider HTML, forms and inline assets. No new debug session, cookies, or protected feature required.

## Pages and interactions
https://www.repark.jp/parking_user/time/result/detail/?park=REP0022209: open and reload. Native AX showed 空車 on reload (previous view 混雑), 4 spaces, 24-hour opening, day/night rates 30min/300JPY and 60min/100JPY, caps 1800/600JPY repeating, vehicle height 2m length 5m width 1.9m weight 2t. Calculation link and public freeword form observed.

## Configuration
Native Chrome through cua_repl; public tab and same-origin CDP event capture. HAR includes one actual native observed request and status, with exact-URL anonymous direct HTTP response-body enrichment. Enrichment time is labelled as such; request timings were not measured. Same-provider HTML is replayable with standard HTTP, with no proxy envelope.

## Endpoints and response shapes
GET /parking_user/time/result/detail/?park=REP0022209 -> 200 text/html. GET /parking_user/time/freeword/?st=1&word=東京駅 -> redirects to /parking_user/time/map.html?lat=35.6812996&lon=139.7670658&... -> 200 text/html SSR lot results. GET /ajax/time_markers.json?range=C...N...W...S...E... -> 200 application/json, array of string-field lot objects and charges groups, 68 rows in Osaka sample. These extra endpoints are source-derived direct HTTP verification, not claimed as native interactions.
GET calculator form -> 200 HTML; POST same form func=settime and pkid, bay, start/end date/hour/minute -> 200 HTML estimate 1800JPY for bay1, 2026-10-03 08:00..12:00 JST. Public simulator excludes discounts and does not guarantee final billed charge.

## Analysis and coverage
browser-sniff analyzed the actual detail capture and emitted one HTML endpoint; primary spec is augmented from exact source form names and verified JSON response fields. No auth or challenge evidence, no cookies exported. Search st=1 provider-resolves named locations; bounds are caller-provided/search-resolved, never inferred driver location. JSON source import_date/updated_at fields are retained raw: their time zone and precise measurement semantics are unestablished. Native detail's map XHR did not appear in captured tail, so range grammar is grounded in its public inline getMarkersAjax/getDispRange functions.

## Source samples and rate limiting
Sample files: detail-osaka.html, search-tokyo.html, markers-osaka.json, quote-osaka.html. Public map keys are redacted in retained HTML. No 429s observed. Full source bodies are working evidence; runtime emits only bounded planning facts.

## Authentication context
No authenticated session used. No secrets, cookie values, payments, reservations or account mutations included.

## Community research and instruction precedence
Focused web queries for Repark CLI/MCP/SDK/plugin/automation libraries did not discover a relevant implementation; official plugin directory lacks Repark. No wrapper source or issue tracker to inspect. User explicitly restricts the batch to one builder and one fresh-context MAX reviewer per CLI: novel scope is synthesized by this builder, overriding the skill's extra brainstorming-agent step. Scope and absorb decisions are preauthorized in the batch brief.
