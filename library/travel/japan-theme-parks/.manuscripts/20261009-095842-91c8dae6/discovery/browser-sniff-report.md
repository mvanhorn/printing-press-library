# Browser-sniff discovery report: japan-theme-parks (2026-10-09)

## 1. User Goal Flow
- Goal: "Pick a park date: tickets still on sale, price for that date, park hours" (read-only).
- TDR: ticket calendar month pages (JA + EN), daily calendar. Completed with Surf Chrome-TLS HTTP; the data is server-rendered, no XHR needed.
- USJ: park-hour calendar (completed), ticket lineup (completed), WEB ticket store dated price (STOPPED: store.usj.co.jp shows a waiting-room queue page with "私はロボットではありません" check; not bypassed per hard limit).
- queue-times: browser-sniff declined (plain JSON API, documented).
- Coverage: 4 of 5 planned steps.

## 2. Pages & Interactions
1. https://www.tokyodisneyresort.jp/ticket/index.html (Surf) — embedded `var ticketPopup = {...}` JSON + calendar table.
2. https://www.tokyodisneyresort.jp/ticket/index/202611/ and /202612/ and /202703/ (Surf) — month pages; 202703 not yet published (0 dates).
3. https://www.tokyodisneyresort.jp/en/ticket/index.html (Surf) — same structure, English names, same ticket ids (T1, T2, T8).
4. https://www.tokyodisneyresort.jp/tdl/daily/calendar.html (Surf) — HTML hours; not needed (ticketPopup carries openTime).
5. browser-use (headless, fresh profile, session `jtp`): https://www.usj.co.jp/web/ja/jp/park-guide/schedule/park-hour — XHR GET https://mobile-service.usj.co.jp/api/Venues/10251/Hours?endDate=MM/DD/YYYY.
6. browser-use: https://store.usj.co.jp/ja/jp/c/ticket — queue/robot-check interstitial; stopped.
7. curl: https://www.usj.co.jp/contentdata/usj/ja/jp/tickets/lineup/index.html — CMS JSON; only "from" prices (e.g. 1デイ・スタジオ・パス 大人 ¥8,400~), text says dated price varies by stock and is shown only in the WEB ticket store.

## 3. Browser-Sniff Configuration
- Backends: Surf (Chrome TLS) for TDR; browser-use 0.13.1 CLI mode, headless, own temp profile (no user Chrome profile, no cookies, no login) for USJ.
- Pacing: >= 1 s between requests; no 429s.
- Proxy-envelope: not detected.

## 4. Endpoints Discovered
| Method | URL | Status | Content-Type | Auth |
|---|---|---|---|---|
| GET | https://queue-times.com/parks.json | 200 | application/json | public |
| GET | https://queue-times.com/parks/{id}/queue_times.json | 200 | application/json | public |
| GET | https://www.tokyodisneyresort.jp/ticket/index/{YYYYMM}/ | 200 | text/html (embedded JSON `ticketPopup`) | public, Chrome TLS required |
| GET | https://www.tokyodisneyresort.jp/en/ticket/index/{YYYYMM}/ | 200 | text/html (embedded JSON) | public, Chrome TLS required |
| GET | https://mobile-service.usj.co.jp/api/Venues/10251/Hours?endDate=MM/DD/YYYY | 200 | application/json | public, plain HTTP, no headers needed |
| GET | https://store.usj.co.jp/ja/jp/c/ticket | 200 queue page | text/html | blocked (queue + robot check) |

## 5. Traffic Analysis
- Protocols: rest_json (queue-times, USJ hours), ssr_embedded_data (TDR `var ticketPopup = {...};` single line).
- Auth signals: none required. USJ bundle embeds web client config for an OIDC app; not used and not needed.
- Protection: TDR Akamai (stdlib timeout; Surf OK). USJ store: waiting room + robot check (not replayable, out of scope).
- TDR status classes: `conversion` = 販売中 on sale; `conversion is-few` = 残りわずか few left; `conversion is-none` = 売り切れ sold out (legend: ○ / △ / X); `conversion is-notSales` = not on sale yet (url empty); `passFlg: true` or empty url = no online sale (legend "ー"). TDR sells each date 2 months ahead at 14:00 JST daily.
- TDR ageText: "大人：￥12,400　中人：￥10,200　小人：￥5,900" (EN: "Adult：11,900 yen　Junior：9,800 yen　Child：5,900 yen").
- USJ hours fields: Date, OpenTimeString, CloseTimeString (ISO +09:00), EarlyEntryUnix, VenueStatus, Holiday, IsShowScheduled. 84 dates (2026-10-09..2026-12-31).

## 6. Coverage Analysis
- Covered: TDR dated sale status + price + hours for both parks (all published months); USJ dated hours; live waits for 5 Japan parks.
- Not covered (explicit unknowns): USJ dated Studio Pass price and Express Pass availability (store behind queue/robot check); Fuji-Q Highland and Legoland Japan hours/tickets (not in the user's source list); TDR dates beyond the published months.

## 7. Response Samples
See direct-response-*.json in this directory.

## 8. Rate Limiting Events
None. ~0.5 req/s.

## 9. Authentication Context
No authenticated session used. No cookies stored.
