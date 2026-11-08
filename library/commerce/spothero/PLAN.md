# SpotHero consumer API notes

Reverse-engineering notes for spothero-pp-cli. Updated 2026-11-07 from the logged-in site and its public JS bundles. Names and shapes only. Nothing marked "from the bundle" has been sent to SpotHero.

> Not the partner API. Partner `X-API-Key` on `api.spothero.com/v2` is B2B only.

## Session

`AUTH_TOKEN` cookie on `.spothero.com`, an RS256 JWT from Auth0 (accounts.spothero.com) with a 30 day expiry. Proven 2026-11-07: that cookie alone returns 200 from `/reservations/` and `/users/me/`. `appSession.*`, `sh-sso` and `csrftoken` alone return 401. The same token as `Authorization: Bearer`, `Token` or `JWT` with no cookie returns 401. `/user/` returns 401 even with a good session.

Chrome stores cookie values in `encrypted_value` with `value` empty. Database version 24 and later prepends SHA-256 of the host to the plaintext.

## Checkout (from the bundle, never sent)

`POST /api/v1/checkout/`, `SpotHero-Version: 2026-10-06`.

Body: `items[]{item_type:"rental", price, key, add_ons:[], item_context{facility, search_id, action_id, phone_number, quote_token, rental_source_title, starts, ends, is_event_rate, vehicle_profile_id | license_plate_str + license_plate_state | license_plate_unknown}}`, `payment{use_spothero_credit, cards[{card_external_id}]}`, `total_price`, `currency`, `email`. The site also sends an analytics `mixpanel_id`, which this CLI leaves out. Item level `rate_id`, `quote_token`, `quote_mac` are no longer sent.

## Cancel

`GET /reservations/{id}/refund-eligibility/` (proven live, `SpotHero-Version: 2025-04-28`) returns `original_source_refund.is_eligible`, `credit_only_refund.is_eligible`, `cancellation_reasons[]{id, reason, reason_displayed_to_customer}`.

`POST /reservations/{id}/refund/` (from the bundle, never sent) with `{reason, access_key, refund_as_credit}`. `reason` is the `reason` string, not the id.

## Inventory

`GET api.spothero.com/v2/search/transient` and `/search/transient/{id}` are public (proven live). Quote fields: `rates[].quote.meta.{quote_token, quote_valid_until}`, `quote.order[0].{starts, ends, rate_id}`, `quote.total_price.value` in cents. Requirements are under `facility.common.requirements`. `tracking.action_id` comes back empty; the site mints its own.

## Live field types that bit us

`vehicles[].license_plate` is an object `{id, value, state}`. `reservations[].cancellation_allowed` is a string such as `customer`. `reservations/` is slow: 25 rows take about 6s and 100 can outlast a 30s timeout.

Paging: `meta.{count, next, previous}` with `page` and `page_size`.

## Timezone

Naive `--starts`/`--ends` use local time via `--city-slug` or the search city. RFC3339 with `Z` or an offset overrides. Craig queries use UTC; checkout `item_context` uses offset times from the quote.
