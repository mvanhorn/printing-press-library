# Booking / Cancel Endpoint Discovery (WeWork desks)

**Capture method:** app JS-bundle string analysis + the "Confirm your desk" modal field inspection, with a hard-block interceptor active (any POST/PUT/PATCH/DELETE to `/workplaceone/api/` was captured-and-blocked, never forwarded). **No booking POST was ever triggered and no charge occurred** — the Book button stayed disabled (invoice address required) and the address-form flow was interrupted by an unrelated browser extension overlay before completion.

## Confirmed facts (from the live confirm modal)
- Booking a desk = a **real card charge** (e.g. $47.00 for the Austin test location, Tue Aug 18 2026, 8:30 AM–5:00 PM) to the member's saved payment account (the member's saved card). NOT membership credits for this account/space.
- Required inputs at confirm time: **invoice address** (street, city, state, zip, country — required for invoicing), **payment account** (required, from saved), optional company name + tax ID, optional promo code.
- Cancellation policy: full refund if canceled ≥24h before start; day-of bookings cancelable within 5 min.

## Endpoint fragments (from JS bundle; base + service prefix assembled at runtime, so full path is inferred)
- Book action: **`reserveworkspace`** (desk = "shared workspace") and/or **`add-reservation`**
- Cancel: **`user-booking-cancellation`**
- Related: `user-booking-details`, `user-booking-modification`, `reservation-list`, `reservations`
- Service host: almost certainly the **`common-booking`** service (confirmed live for `common-booking/upcoming-bookings`).

## Inferred endpoints (NOT verified against a live booking — ship behind --dry-run + verify-on-first-use)
| Intent | Method | Inferred path | Confidence |
|---|---|---|---|
| Book a desk | POST | `/workplaceone/api/common-booking/reserveworkspace` (fallback `/common-booking/add-reservation`) | medium |
| Cancel a booking | POST | `/workplaceone/api/common-booking/user-booking-cancellation` | medium |
| Booking details | GET | `/workplaceone/api/common-booking/user-booking-details` | medium |

## Inferred book payload (from get-spaces fields + confirm modal)
Constructed from: desk `uuid` / `inventoryUuid`, `location.uuid`, `productPrice.productUuid`, `productPrice.price.{amount,currency}`, the selected `date`, `startTime`/`endTime` (default 08:30–17:00 local), a `paymentAccountUuid`, and an invoice `address` object (street/city/state/zip/country) + optional `companyName`. Exact field names unverified.

## CLI implication
- `book` and `cancel` ship as **explicit best-effort commands**: default `--dry-run` prints the exact request that would be sent; `--confirm` required to actually send; help + README clearly state the request shape was inferred from discovery (not a verified live booking) and should be validated on first real use. This avoids shipping a silently-wrong mutation and avoids having placed a real test reservation.
