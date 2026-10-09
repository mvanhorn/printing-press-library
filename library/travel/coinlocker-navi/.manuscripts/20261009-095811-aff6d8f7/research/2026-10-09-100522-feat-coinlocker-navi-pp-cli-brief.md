# coinlocker-navi CLI Brief

## API Identity
- Domain: Walk-up coin lockers in Japan. Primary source: コインロッカーなび / Coin Locker Navi (https://www.coinlocker-navi.com/, operated by SHIFT PLUS Inc.). Secondary live-vacancy source: マルチエキューブ / Multi Ekicube (https://multiecube.com/, JR East Smart Logistics; public JSON at https://api.multiecube.com).
- Users: trip planners and travellers deciding where to drop a suitcase or day bag without a booking.
- Data profile: server-rendered Japanese HTML (no official API). Locker records: id (/cl/<id>), name, note (location text), service type (コインロッカー / ecbo cloak / 荷物預かり), lat/lon (Google Maps link), sizes with price and box count (小/中/大/特大, e.g. "中 400円/39個"), change machine (両替機), payment methods (現金, Suica, PASMO, ICOCA, PiTaPa, クレジットカード...), hours (利用時間). Many fields are "情報なし" (no information). No per-record last-updated date on the page.

## Verified surfaces (plain HTTP, no auth, 2026-10-09)
- GET /search?q=<keyword> -> HTML list of up to 10 lockers (+"もっと見る" POST /search_more pattern via CSRF). 200.
- GET /cl/<id> -> detail page: 基本情報 (サービス, サイズ・料金, 利用時間, 支払方法, 両替機, 備考) + 周辺のコインロッカー (5 nearest with distance m). 200.
- GET /search/gps/ sets csrfToken cookie + X-CSRF-Token in inline JS; POST /search/gps/nearest_cl form location_lat, location_lon -> HTML fragment with ~100 lockers sorted by distance (m). 200, verified at Shinjuku (35.6896,139.7006).
- GET /<pref>/area/<slug>/ and /<pref>/eki/<line>/<station>/ -> nearest lockers to an area/station centroid with distance. POST ../../area_more {cnt,key} for more.
- Real-time vacancy on coinlocker-navi (FAQ confirms "一部のコインロッカー情報はリアルタイムの空き情報検索に対応"):
  - /areamap/maihamaeki/ (舞浜駅, Tokyo Disney Resort): page shows "YYYY/MM/DD HH:MM現在の情報"; GET ./ajax?gid=0001..0006 -> per block 小/中/大 設置数 and 空き数 + /cl/<id> link. 200.
  - /areamap/hankyu-umeda/{1f,2f,3f} (大阪梅田駅 阪急, PiTaPa lockers): embedded JSON objects with total_{s,m,l}_num, blank_{s,m,l}_num, num_update (ISO JST), area_nm, floor_nm. 200.
  - /areamap/jreast-tokyostation/ and /areamap/tokyoeki-1bangai/ -> 301 to /areamap/servicestop/ (service stopped).
- Multi Ekicube: GET https://api.multiecube.com/v1/location/ph2?lang={ja|en}&service_type=1,3&includes_premium=true&includes_no_empty=true&from_at=YYYY-MM-DD&end_at=YYYY-MM-DD[&q=|&latitude=&longitude=&distance_max=][&ticket_gate=inside|outside][&page_no=N] -> JSON {locations[], total, page_no, page_total, pagination}. Each location: id, lat/lon, inside_ticket_gate, outside_ticket_gate, attributes.display_name, distance (m), business_hours (per weekday + summary), base (station name, English when lang=en), area (gate/floor text), box_availability.{ss,s,m,l,lw,...}.by_service[{service_type, num_empty, price{basic,std,ext,late}}]. Confirmed by alltheplaces spider multi_ecube_jp.py. 381 sites / 1,149 units (JR East SL, Aug 2026).
  - Semantics (from the site's own strings): num_empty is the reservable-empty count; "There may still be lockers available even though there are no available lockers in the search results"; listed rates on the reservation path include a ¥500 reservation fee (price.basic=500). price.std is the usage fee.

## Reachability Risk
- Low. All surfaces return 200 to plain HTTP with a browser UA. CSRF token for the GPS POST is minted by a GET in the same cookie jar (replayable). No Cloudflare/captcha observed. Hankyu page JSON leaks through a broken template (brittle; parse defensively). Two of four areamap pages are already stopped.
- Terms (coinlocker-navi 利用規約): content may not be copied beyond private use. Implication: on-demand lookups only, no bulk mirror, polite rate (1 req/s), no redistribution.

## Top Workflows
1. "Lockers near this station/spot": keyword or lat/lon -> nearest walk-up lockers with distance, sizes, prices, payment (IC card?), hours, inside/outside gate when known.
2. "Is there space right now?": live vacancy at stations that publish it (Multi Ekicube JR East sites by station/coords; Maihama; Hankyu Osaka-Umeda).
3. "Will my large suitcase fit / what does L cost?": filter to lockers with a large/XL size and a known price.
4. "Detail on one locker": /cl/<id> with neighbours.

## Table Stakes
- Keyword search (station/area names), GPS-nearest, detail page, size/price/payment/hours. Google Maps link.

## Data Layer
- Primary entities: locker (coinlocker-navi), live vacancy snapshot (multiecube location, areamap block).
- No sync/full mirror (terms + low value; data is on-demand and live). Optional short-TTL HTTP cache only.
- FTS/search: not needed; the source search is the index.

## User Vision
- Trip planners: "Can I leave a large suitcase / day bag at station X (or near spot Y) without a booking, what size and price, inside or outside the ticket gates, what hours, IC card (Suica/PASMO)?" Complements ecbo-cloak-pp-cli (bookable storage) and yamato-pp-cli (forwarding). Include real-time vacancy from public sources when found; otherwise static facts with source dates and explicit unknowns. Less is more: keep only commands that change a planning decision. Bilingual names where the source has them.

## Source Priority
- Primary: coinlocker-navi — no spec, HTML replay — free.
- Secondary: Multi Ekicube public JSON — free, no key — scoped to live vacancy only.
- Economics: both free; no keys.
- Inversion risk: Multi Ekicube has cleaner JSON, but coinlocker-navi stays the headline (nationwide walk-up coverage); Multi Ekicube only feeds `vacancy`.

## Product Thesis
- Name: coinlocker-navi-pp-cli
- Why it should exist: no tool answers "walk-up locker near X, does my bag fit, does it take Suica, inside the gate, is there space now" in one honest JSON answer. The site is Japanese-only, ad-heavy, and hides most "unknown" fields; live vacancy is split across operator pages nobody finds.

## Build Priorities
1. `near` (keyword or --lat/--lon) with normalized sizes/prices/payment/hours/gate and explicit unknowns.
2. `vacancy` (live) merging Multi Ekicube + coinlocker-navi areamap sources, with as-of time and the "0 reservable != full" caveat.
3. `locker <id>` detail.
