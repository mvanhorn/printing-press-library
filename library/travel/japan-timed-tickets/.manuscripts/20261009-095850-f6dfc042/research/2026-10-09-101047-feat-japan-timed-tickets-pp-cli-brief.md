# japan-timed-tickets CLI Brief

Research date: 2026-10-09 (JST). All facts below were read from the live sources on that date.

## API Identity
- Domain: official timed-entry ticket sale rules and public dated availability for three Japan sight families: Ghibli Museum, Mitaka (三鷹の森ジブリ美術館), SHIBUYA SKY (渋谷スカイ), and teamLab permanent venues in Japan (チームラボ).
- Users: trip planners and their agents who must decide when to be online (JST) and which channel to use, and which visit dates/time slots are still open.
- Data profile: small, fast-changing, per-date and per-slot inventory plus slow-changing sale rules. No official public API. Undocumented public JSON on teamLab ticket hosts; static HTML elsewhere.

## Reachability Risk
- teamLab: Low. `probe-reachability https://www.teamlab.art/e/planets/` says browser_clearance_http (0.6), but that is a false positive: both rungs returned 200 with full content and the only evidence is the body text marker "perimeterx". The ticket hosts are plain JSON over stdlib HTTP:
  - `https://{borderless-azabudai,kyoto,botanicalgarden,dfs-okinawa}.ticket(s).teamlab.art` and `https://ticket.e-zofukuoka.com` (same SPA backend): `GET /api/v1/configurations` (Calendar_Period = last bookable month, e.g. 2026-12), `GET /api/v1/products/stocks-statuses?from&to` (per-date `stock_status` in {"", few, sold_out, closed}; `to` beyond Calendar_Period returns 400 "beyond the calendar range"), `GET /api/v1/products/search2?date&lang` (products on a date, is_few_stock/is_sold_out), `GET /api/v1/products/stocks?date&product_id=<uuid>&lang` (30-min `time_spans` with stock_count, is_few_stock, start/end JST, price per option), `GET /api/v1/texts?lang=en|ja` (`unpublished_calendar_notice`, e.g. "On sale at 12:00 PM on November 2 (tentative)." / "10月30日 12時00分発売予定").
  - teamLab Planets TOKYO uses DMM (`teamlabplanets.dmm.com`): `GET /en/ticket` -> 302 `/en/ticket/redirect` -> 302 `/en/ticket/admission_date/<token>` (HTML calendar for ~3 months; per-day classes is-available / is-few / is-soldOut / close, price "4,800~", note "Tickets for January will be available starting late-October."). `POST /ticket/admission_time_list/<token>` form `entry_date=YYYYMMDD` returns JSON 30-min `entry_times` with `max_stock`, `instock`, prices. The token is an anonymous per-visit page token from the redirect (no login, no cart). The POST is a read of time slots (the site's own calendar does it on date click).
  - Browser Use is NOT needed. The approved Browser Use runtime is not used.
- Lawson Ticket (l-tike.com): Medium. Akamai rejects a Chrome User-Agent from a non-Chrome TLS stack (HTTP/2 INTERNAL_ERROR / timeout), but an honest non-browser UA over Go stdlib (Go 1.26+ defaults) gets 200. `https://l-tike.com/ghibli/` (sale rules and monthly exceptions such as "※10月入場券は9/10(木)発売", "10/1・10/3は市民デーの為、一般販売はございません") and `https://l-tike.com/search/?lcd=300MM` (sale window "2026/9/10(木) 10:00 ～ 2026/10/31(土) 16:00", status 発売中). Per-date Ghibli availability is behind Lawson WEB member login + phone verification: OUT OF SCOPE (unknown).
- SHIBUYA SKY: official site is plain HTTP (Next.js, facts are static strings in the page chunk). Ticket store `webket.jp` 302-redirects every page to `waiting.webket.jp` (Queue-it virtual waiting room). Hard limit: never join a queue. Per-slot availability is therefore UNKNOWN by design; the CLI gives the rule-derived sale opening and a handoff URL.
- Ticket Pia (t.pia.jp): reachable, but searches for チームラボ / ジブリ / SHIBUYA SKY / 渋谷スカイ returned 0 sale pages. Dropped as a source (no sale pages for these sights).

## Source facts (verbatim-derived)
- Ghibli Museum (ghibli-museum.jp/ticket/ and /en/tickets/): advance reservation only; no sale at the museum. Sale: 10:00 JST on the 10th of each month for the next month ("毎月10日の10時から、翌月入場分を発売"). Entry times 10,11,12,13,14,15,16 (enter within 1 hour). Prices 1,000/700/400/100 JPY, under 4 free. Closed every Tuesday plus listed dates; EN page has a 15-month calendar with `close-dates` cells. Channels: Lawson Ticket (domestic, needs Lawson WEB member + Japanese mobile number for e-ticket), Lawson Ticket English site (overseas), Sunrise Tours JTB bus tour. Ticket format change: from Nov 2026 entry, browser e-ticket or paper ticket at Lawson.
- SHIBUYA SKY (FAQ + ticket page chunk): sale opens at 00:00 JST two weeks before the entry date ("入場日の2週間前の日本時間午前0時から販売"); 20-minute entry slots; web adult 2,700 JPY (entry until 14:59) / 3,400 JPY (15:00 and later); counter 3,000 / 3,700; children (elementary) and infants: counter only, same day; if web is sold out the counter is sold out for adults/teens; cancellations can reopen slots. Rooftop can close for weather.
- teamLab: dynamic pricing ("~"), per-venue next-month release announcement, Calendar_Period per venue.

## Top Workflows
1. "When do tickets for <sight> on <date> go on sale?" -> on-sale datetime JST (+ traveler local time), channel, source rule, exceptions (e.g. Ghibli 市民デー dates with no general sale), and whether the window is already open.
2. "Which teamLab dates/time slots are still open for my trip dates?" -> per date status and per 30-min slot stock, sold-out explicit, fetch timestamp.
3. "Sunset at SHIBUYA SKY on <date>": sunset time for Shibuya (computed), the 20-minute slots around sunset, price tier, and when that date goes on sale (00:00 JST, D-14); availability unknown (queue-protected).
4. "Trip plan across sights": one calendar (agenda) of on-sale moments for all sights for a trip date range, sorted by when the user must act.

## Table Stakes
- No competing CLI/library found (GitHub/npm searches return only travel blogs, Fiverr agents and resale listings). Blogs disagree with the source (one says SHIBUYA SKY sells 4 weeks ahead; the official FAQ says 2 weeks) -> source-backed facts are the value.
- Table stakes from user CLIs in this repo (eplus, tablecheck, omakase): `--agent` JSON envelope with fetched_at, source URLs, explicit unknowns, `doctor`, `--dry-run`, bounded requests, bilingual names.

## Data Layer
- Primary entities: sight (static registry, bilingual), sale_window (computed per visit date), day_status, slot.
- Sync cursor: none. Live reads with a short cache; no offline mirror needed (data is volatile; less is more).
- FTS/search: not needed (closed set of ~8 sights).

## User Vision
- Decide: "When do tickets for <sight> on <date> go on sale, which dates/time slots are still open, and what must I do (which channel, which day, what JST time)?"
- Core outputs: sale-window calendar (on-sale date/time JST per sight per visit date) and dated slot availability with fetch timestamps. Sold-out and unknown explicit.
- Less is more: only commands that change a planning decision. Read-only: never buy, never add to cart, never join a queue, never log in. Bilingual names. Source dates.

## Source Priority
- Primary: Ghibli Museum — official HTML (no spec) + Lawson Ticket sale page for windows — free.
- Secondary: SHIBUYA SKY — official HTML/static chunk — free; availability unknown (Queue-it).
- Tertiary: teamLab — undocumented public JSON (ticket.teamlab.art hosts) + DMM HTML/JSON for Planets — free; richest availability data.
- Lawson Ticket — only the Ghibli sale page/search. Ticket Pia — dropped (no sale pages).
- Economics: all free, no keys.
- Inversion risk: teamLab has the richest machine-readable data; do not let it push Ghibli out of the headline. The headline command is the sale-window calendar where Ghibli is first.

## Product Thesis
- Name: japan-timed-tickets (`japan-timed-tickets-pp-cli`)
- Why it should exist: the three most-asked "how do I get tickets" Tokyo sights each use a different rule (monthly 10th 10:00 JST; rolling D-14 00:00 JST; per-venue monthly release with dynamic pricing). Travelers miss windows because of time zones and blog misinformation. One read-only CLI answers "when, where, and what is still open" with source evidence.

## Build Priorities
1. `onsale` — sale-window calendar for sights x visit dates (computed from source rules, confirmed against live source text; JST + optional --tz).
2. `availability` — dated day status + time slots (teamLab live; SHIBUYA SKY and Ghibli explicit unknown with reason and handoff URL).
3. `sunset` (SHIBUYA SKY) — sunset-covering slots and price tier for a date; folds into availability if cut.
4. `sights` — registry with bilingual names, channels, rules, source URLs.
5. `doctor` — reachability of each source without buying anything.

## Reachability Gate (Phase 1.9, 2026-10-09)
- Decision: PASS. 200 from teamLab ticket JSON (/api/v1/configurations), ghibli-museum.jp/en/tickets/, teamlabplanets.dmm.com/en, shibuya-scramble-square.com/sky/ticket/, l-tike.com/ghibli/.
- Lawson Ticket note: the Akamai edge resets HTTP/2 streams for some header combinations. Honest UA `japan-timed-tickets-pp-cli/<ver>` + `Accept: text/html,application/xhtml+xml` returned 200 on 6/6 spaced requests. Chrome UA spoofing is not used (user rule).
