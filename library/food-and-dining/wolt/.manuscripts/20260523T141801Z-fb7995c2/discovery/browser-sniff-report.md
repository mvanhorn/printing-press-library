# Wolt Browser-Sniff Report

## Goal
Identify the live request shape for: (1) menu items endpoint, (2) order tracking by share link.

## Capture method
browser-harness CDP Network domain, navigation to wolt.com/en/fin/helsinki/restaurant/noodle-story-kamppi.

## What we found

### Confirmed via live capture
- **Venue details / open status / delivery configs**: `GET https://consumer-api.wolt.com/order-xp/web/v1/venue/slug/{slug}/dynamic/?selected_delivery_method=homedelivery`
  - Required headers minimum: `Referer: https://wolt.com/`, `Platform: Web`
  - Response: `{venue, venue_raw, is_venue_favourite, order_status, order_minimum, ...}`
  - Verified with curl outside browser (14.9KB response).
- Geo-IP city resolution: `GET https://wolt.com/v1/geo_ip_approximate`
- City config: `GET https://restaurant-api.wolt.com/v2/config?lat=&lon=`
- Address fields per locale: `GET https://restaurant-api.wolt.com/v1/consumer-api/address-fields?language=en&lat=&lon=`

### Documented but live-broken
- **Menu items endpoint**: `GET https://restaurant-api.wolt.com/v4/venues/slug/{slug}/menu/data` returns HTTP 200 with `content-length: 0` via CloudFront even with browser-like headers. The SPA renders menu items as `horizontal-item-card` DOM nodes, sourced from an XHR our 500-event buffer truncated past. Suspect: additional session/fingerprint header required, or the endpoint has moved to a path I did not enumerate.
- **Order tracking by share link**: `wolt.com/en/track/<id>` is the user-facing URL. The JSON endpoint behind it is undocumented. No live order available to browser-sniff during this run.

### Endpoint variants tried and ruled out
- `/v3/venues/slug/{slug}` → 410 Gone
- `/v3/menus/slug/{slug}` → 404
- `/v3/venues/slug/{slug}/menu` → 410 Gone
- `/v2/venues/slug/{slug}` → 404
- `/consumer-assortment/consumer-assortment/items/menu/slug/{slug}` → 404
- `/order-xp/web/v1/venue/slug/{slug}/menu/` → 404
- `/order-tracking/v1/orders/<id>` → 404

## Recommendation for printed CLI

Ship four browse commands fully implemented (cities list, restaurants near, search, venue details). For menu items and order tracking, ship working stubs that:
- attempt the documented endpoint with the browser-like header set,
- on empty/404 response, return a clear actionable error pointing the user to file an issue with the live request URL captured from DevTools,
- include `--help` examples showing the known-good usage.

This is consistent with the user's "browse-only good enough" scope.
