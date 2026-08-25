# Discovery Report — SEA Airport Parking (reservesea.portseattle.org)

**Method:** Direct HTTP capture (curl + cookie jar, Chrome UA). Runtime mode: `standard_http`.
**Platform:** AeroParker (Metropolis Technologies) white-label Java servlet engine.

## Session bootstrap
`GET /book/SEA/Parking?parkingCmd=collectParkingDetails` sets `AWSALB`, `AWSALBCORS`, `JSESSIONID; Path=/book`, and `parkingReservationGuid` (cart UUID). These must be carried into the subsequent POST. No user credential required.

## Availability / quote endpoint (headline, replayable)
`POST /book/SEA/Parking`
Form body: `parkingCmd=collectParkingDetails`, `progressToNextStep=1`, `entryDate=MM/DD/YYYY`, `entryTime=HH:MM`, `exitDate=MM/DD/YYYY`, `exitTime=HH:MM`, optional `promocodes`.

- **303 → `?parkingCmd=selectProduct`** on valid+available. Response HTML embeds:
  - `quotes:[{"id":335,"price":188.0,"originalPrice":"$188.00"}]` — total price for the range.
  - Per-date price array: `[{"date":"2026-08-12T00:00:00Z","price":47.0,"rollupPrice":0.0}, ...]`.
  - Product identity: `data-product-id="335" data-product-name="Reserved Parking" data-product-code="RP" data-position="1"`, brand color `#044b1b`.
  - `item__price__val` / `item__price__pence` / `item__price__del` (crossed-out list price = prepaid discount).
  - `productIdLegacyCodeJson:[{"id":335,"carParkId":105}]`.
  - Travel extras: `shopProductAvailabilityDetails:[{"id":4,"adultPrice":50.0,...}]` (e.g. lounge upsell).
  - Reviews via third-party `api.reviews.io`; cart status via `BasketStatusAjax`.
- **Sold out:** selectProduct page, greyed SOLD OUT, "Sorry, this product is unavailable for the dates you requested."
- **Invalid:** re-renders collectParkingDetails with inline error text (min 2-day stay, min 6h advance, etc.).

## Downstream (booking) steps — mutation, guest checkout
`selectProduct` → `collectExtras` → `collectContactDetails` (name, email, phone, **license plate + make/model required**) → payment → confirmation number. Guest checkout supported (no login). Recommend v1 quotes and hands off before payment; never auto-submit a card.

## Replayable surface verdict
Availability, quote, per-date pricing, product metadata, and promo pricing are all obtainable via anonymous HTTP + embedded-JSON extraction. The printed CLI ships a plain HTTP client (no resident browser). Booking is quote+prefill handoff.

## Multi-airport
Same AeroParker grammar at IAH/HOU, LAS, ORD, ORF. An `--airport`/host override is a low-cost reuse bonus; SEA is the default.
