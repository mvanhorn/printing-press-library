# Discovery report (direct HTTP; no browser capture)

Browser capture was not run: the user's rule allows it only when plain HTTP is not enough, and plain HTTP replayed every needed surface.

## User Goal Flow
- Goal: find walk-up coin lockers near a station/spot, with size, price, IC card, hours, gate side, and live vacancy where published.
- Steps: homepage -> prefecture/area/station pages -> keyword search -> locker detail -> GPS-nearest (CSRF POST) -> areamap live vacancy (Maihama, Hankyu Umeda) -> Multi Ekicube JSON search.
- Coverage: all planned steps completed.

## Endpoints Discovered (all public)
| Method | URL | Status | Content-Type |
|---|---|---|---|
| GET | https://www.coinlocker-navi.com/search?q=<kw> | 200 | text/html |
| GET | https://www.coinlocker-navi.com/cl/<id> | 200 | text/html |
| GET | https://www.coinlocker-navi.com/search/gps/ (sets csrfToken cookie; token in inline JS) | 200 | text/html |
| POST | https://www.coinlocker-navi.com/search/gps/nearest_cl (form: location_lat, location_lon; header X-CSRF-Token) | 200 | text/html fragment |
| GET | https://www.coinlocker-navi.com/<pref>/area/<slug>/ , /<pref>/eki/<line>/<station>/ | 200 | text/html |
| GET | https://www.coinlocker-navi.com/areamap/maihamaeki/ (as-of time "YYYY/MM/DD HH:MM現在") | 200 | text/html |
| GET | https://www.coinlocker-navi.com/areamap/maihamaeki/ajax?gid=0001..0006 | 200 | text/html fragment |
| GET | https://www.coinlocker-navi.com/areamap/hankyu-umeda/{1f,2f,3f} (embedded JSON: total_/blank_{s,m,l}_num, num_update) | 200 | text/html |
| GET | https://www.coinlocker-navi.com/areamap/jreast-tokyostation/ | 301 -> /areamap/servicestop/ | - |
| GET | https://api.multiecube.com/v1/location/ph2?lang=&service_type=1,3&includes_premium=true&includes_no_empty=true&from_at=&end_at=&[q|latitude,longitude,distance_max]&ticket_gate=&page_no= | 200 | application/json |

## Traffic Analysis
- Protocols: server-rendered HTML (coinlocker-navi, Shift-JIS bytes in some comments; body UTF-8), REST JSON (multiecube).
- Auth: none. CSRF: cookie `csrfToken` + header `X-CSRF-Token` (CakePHP style) for the GPS POST only.
- Protection: none observed (no Cloudflare/captcha). Browser UA used.
- Parameter evidence: multiecube param names from site bundle (`includes_premium`, `includes_no_empty`, `freely_boxes_only`, `q`, `latitude`, `longitude`, `distance_max`, `ticket_gate`=inside|outside, `box`=<size>:1, `from_at`, `end_at`, `lang`, `service_type`, `page_no`, `limit`) and alltheplaces spider `multi_ecube_jp.py`.
- Warnings: Hankyu page JSON leaks through a broken template (repeated objects; parse first-seen per id). Two areamap pages are already retired.

## Rate Limiting Events
- None. ~1 req/s manual pacing.

## Authentication Context
- No authenticated session used.

## Bundle Extraction
- https://multiecube.com/_next/static/chunks/*.js: axios baseURL https://api.multiecube.com; `/v1/location/ph2` GET; service_type enum {GENERAL:1, PREMIUM:2, FREELY_GENERAL:3, FREELY_PREMIUM:4}; strings "There may still be lockers available even though there are no available lockers in the search results" and "The rates listed include a reservation fee of ¥500".

## Outbound Links Observed In Source Pages
- https://maps.google.com/maps?q=<lat>,<lon> — every coinlocker-navi list row and detail page carries this Google Maps link. The CLI copies it into `map_url` and reads `q=lat,lon` as coordinates. The CLI never sends a request to maps.google.com.
