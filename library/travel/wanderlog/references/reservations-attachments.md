# Reservations And Attachments Reference

Wanderlog Reservations and attachments: Flight, Lodging, Rental car, Restaurant, Train, Bus, Ferry, Cruise, Attachment.

## Command Surface

```bash
wanderlog-pp-cli plan reservation list --target-key TARGET --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 1 --kind KIND ... --dry-run --agent
wanderlog-pp-cli plan reservation edit --target-key TARGET --day 1 --block-id BLOCK_ID --field FIELD --value VAL --dry-run --agent
wanderlog-pp-cli plan reservation remove --target-key TARGET --day 1 --block-id BLOCK_ID --kind KIND --dry-run --agent
```

`--kind`: `flight`, `lodging`, `rental-car`, `restaurant`, `train`, `bus`, `ferry`, `cruise`, `attachment`.

Hotel search: `lodging search --geo-id … --bounds … --start-date … --end-date … --agent`. Compact candidate list by default; `--raw-response` only for the full payload.

## Block Model

- `flight`, `rentalCar`, `train`, `bus`, `ferry`, `cruise`, `attachment`: native block types.
- `lodging`: `place` block with nested `hotel`.
- `restaurant`: `place` block plus reservation fields (`date`, `startTime`, `partySize`, `nameForReservation`, `confirmationNumber`).
- `plan block attachment *` attaches files/links to an existing block. Standalone Attachment item: `plan reservation add --kind attachment`.

## Lodging

```bash
wanderlog-pp-cli plan reservation add --target-key TARGET --day 1 --kind lodging --place-id PLACE_ID --start-date YYYY-MM-DD --end-date YYYY-MM-DD --traveler-name "Ada Lovelace" --dry-run --agent
```

Resolve with `--place-id`, or `--query NAME --lat LAT --lng LNG`, or `--lodging-offer-json` from `lodging search --raw-response` (Airbnb/Kayak/internal offers without Google ids). After geocode the report includes `resolved_name`, `resolved_address`, `distance_m`. `--expect-name-substring` fails closed if nothing matches. `--display-name` sets `place.name` after resolve.

**Multi-night stays.** `--span-nights` defaults true when `end-date` > `start-date`. That copies the lodging place+hotel onto each dated day in `[checkIn, checkOut)` with new ids in one ops array. Do not JSON0-`li` extra night copies. Dry-run/apply report `{day,date,block_id,name}` per night.

**Rename.** `plan block rename --name "Property"` — never a raw op.

Link costs with `plan budget expense add --category lodging --block-id BLOCK_ID`.

## Other Kinds

```bash
wanderlog-pp-cli plan reservation add --target-key TARGET --day 1 --kind flight --airline XX --flight-number 123 --departure-airport AAA --arrival-airport BBB --start-date YYYY-MM-DD --start-time 08:30 --end-date YYYY-MM-DD --end-time 11:15 --dry-run --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 2 --kind rental-car --pickup-place-id PICKUP --dropoff-place-id DROPOFF --start-date YYYY-MM-DD --start-time 09:00 --end-date YYYY-MM-DD --end-time 18:00 --dry-run --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 2 --kind restaurant --place-id PLACE_ID --start-date YYYY-MM-DD --start-time 19:00 --party-size 4 --name-for-reservation Ada --dry-run --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 3 --kind train --departure-place-id FROM --arrival-place-id TO --carrier CARRIER --start-date YYYY-MM-DD --start-time 10:00 --end-date YYYY-MM-DD --end-time 11:15 --dry-run --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 4 --kind cruise --departure-place-id FROM --arrival-place-id TO --cruise-line "Example" --ship-name Blue --voyage-number V1 --start-date YYYY-MM-DD --end-date YYYY-MM-DD --dry-run --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 1 --kind attachment --title "Tickets" --url https://example.com/tickets.pdf --filename tickets.pdf --mime-type application/pdf --dry-run --agent
```

Bus/ferry: same shape as train with `--kind bus` or `--kind ferry`.

## Edit Fields

`plan reservation edit --field hotel.checkIn` (or `partySize` with `--json-value`, or `attachments.0.title`). `--json-value` for numbers, booleans, arrays, objects, null. `--remove` only after list/outline confirms the field exists.

## Safety

- Outline and `plan reservation list` before apply.
- Disposable clone for live tests.
- Do not book travel; these commands only record plan data.
- After apply, list again; `plan undo --apply` if the block is wrong.
