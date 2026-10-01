# eplus CLI brief

## API identity and user vision
Read-only discovery and sale planning for Japanese concert, theatre, sports and cultural events on eplus.jp and ib.eplus.jp. The user plans Japan activities, needs exact performance and sale-round identity, JST time, lottery deadlines, availability distinctions and overseas eligibility. Booking is a browser handoff; no purchase, reservation or lottery submission.

## Top workflows
1. Search performances by keyword/artist, date, region, category and venue.
2. Inspect one event or individual performance with all visible sale rounds.
3. Plan lottery/general-sale windows without conflating entry with seats.
4. Discover international offerings separately and read product-specific restrictions and prices.
5. Produce compact projected JSON with source timestamps and explicit missing fields.

## Table stakes and competitors
The domestic website is the primary incumbent, offering search/filters and a separate performance/detail view. The international CS-Cart tour/product catalog is the second public surface; offerings differ. Searches for eplus SDK/CLI/MCP did not identify an authoritative integration to reuse; unrelated ticket automations are not relevant contracts. Public website HTML and embedded JSON are the contract, not an official supported API.

## Data layer
Event -> performance -> sale round. No SQLite mirror: ticket inventory needs fresh source checks. Short read-through cache records retrieval time; --fresh bypasses it. Preserve source IDs/URLs/Japanese names. Search/default one bounded page; follow only observed pagination links/parameters.

## Reachability risk
Public GETs returned 200, useful SSR HTML/embedded JSON, without authentication or browser resident runtime. Network sandbox DNS fails; authorized network escalations succeed. No published rate limit found; conservative concurrency <=3, retry <=2, timeout <=20s, body <=4MiB. Parser drift is a material risk and fail-closed extraction is required.

## Contracts observed 2026-09-30 UTC
Domestic GET /sf/search?keyword=Radiohead&block=true returns script#json data.so_kensu/record_list. Record contains kogyo_code/sub, koen_code, koen_detail_url_pc, kanren_venue, koenbi_term, kaien_time, kaijo_time and kanren_uketsuke_koen_list. /sf/detail/{id} SSR article.block-ticket-article contains session, sections.block-ticket with exact sale kind/status/windows and booking links.
International GET /concert and / exposes .col-product tour cards; /tokinosora-live-st contains group-schedule-row links to products.view product_id/date. Products have ticket options, displayed prices, descriptions, eligibility and conditions. Catalog currently includes one streaming tour; empty sports/culture categories must remain real empty results, not synthesized from stale archived products.

## Authentication and access
Discovery uses anonymous GET only. Domestic membership/order/entry requires authentication, excluded. Domestic listings do not prove overseas bookability. International FAQ https://ib.eplus.jp/faq describes overseas customer eligibility, VISA/MasterCard/Alipay, handling charge included; event-specific conditions override generic guidance. Official domestic support notes overseas delivery is unsupported unless explicitly offered. Do not infer residency/phone/payment conditions from nationality; only return explicit source text.

## Product thesis and build priorities
Name: eplus-cli. Useful because it keeps event, session and sale round distinct, provides source-backed overseas planning and compact agent output. Implement search, detail, catalog/policies, field projection, bounded HTTP/cache/error metadata, deterministic consequential tests, live read-only matrix and efficiency proof.
