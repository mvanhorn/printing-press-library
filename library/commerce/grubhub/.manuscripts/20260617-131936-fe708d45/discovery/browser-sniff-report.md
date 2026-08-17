# Grubhub Discovery Report

**Method:** Direct HTTP probing (curl) during Phase 1 research. The Phase-0 "website itself"
choice pre-approved browser capture, but direct probing mapped the entire replayable contract
first, so a Chrome capture would have added nothing.

**Reachability:** `standard_http`. Every endpoint returned clean JSON 200 from a datacenter host
on 2026-06-17 with no PerimeterX/HUMAN challenge.

## Auth handshake (confirmed)
1. `GET https://www.grubhub.com/eat/static-content-unauth?contentOnly=1` → scrape `beta_[A-Za-z0-9]+` client_id (live: `beta_UmWlpstzQSFmocLy3h1UieYcVST`, rotates).
2. `POST https://api-gtm.grubhub.com/auth` header `Authorization: Bearer` (empty seed), body `{"brand":"GRUBHUB","client_id":"<scraped>","device_id":<rand10>,"scope":"anonymous"}` → `{"session_handle":{"access_token","refresh_token"}}`.
3. All calls thereafter: `Authorization: Bearer <access_token>`.

## Endpoints (all confirmed 200)
| Purpose | Method & path | Key params |
|---|---|---|
| Geocode address | `GET /geocode?address=<urlenc>` | returns array; `[0].latitude/.longitude` |
| Restaurant search | `GET /restaurants/search` | `location=POINT(lng lat)` (urlenc), `orderMethod=delivery`, `facetSet=umamiV6`, `sortSetId=umamiv3`, `pageSize`, `includeOffers=true`, `sponsoredSize`, `hideHateoasLinks=true` |
| Restaurant details + menu | `GET /restaurants/{id}` | `version=4`, `orderType=standard`, `location=POINT(lng lat)`, `showMenuItemCoupons=true`, `includePromos=true`, `hideUnavailableMenuItems=true` |
| Menu item (modifiers) | `GET /restaurants/{id}/menu_items/{itemId}` | `time=<epoch_ms>`, `orderType=standard`, `version=4` |
| Availability | `GET /restaurants/availability_summaries` | `location`, `time`, `orderType`, `pageNum`, `pageSize` (200; needs valid time) |
| Order history (credentialed) | `GET /diners/{ud_id}/search_listing` | requires login; out of v1 scope |

## Response shapes
- Search: `{search_id, search_result:{stats:{total_results,total_hits,page_size}, pager:{total_pages,current_page}, results:[card...]}}`. Card fields: `restaurant_id, name, cuisines[], ratings, price_rating, delivery_fee.price, delivery_minimum.price, delivery_time_estimate, distance_from_location, open, coupons_available, available_offers[], available_promo_codes[], address, logo`.
- Details: `restaurant:{id, name, latitude, longitude, address, cuisines, menu_category_list[]:{menu_item_list[]}, available_offers, available_promo_codes, has_coupons}`.
- Menu item: top-level `{id, name, description, price:{amount,currency,styled_text}, choice_category_list[], item_coupon, popular, menu_category_name}`.

**Conclusion:** Ship plain-HTTP transport. No resident browser, no clearance cookie, no user API key.
