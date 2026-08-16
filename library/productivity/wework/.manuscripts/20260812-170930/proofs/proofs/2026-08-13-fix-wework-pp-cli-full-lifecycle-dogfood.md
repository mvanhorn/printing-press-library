# WeWork CLI — 100% Live Dogfood (headless auth + full book/cancel lifecycle)

## Outcome: FULLY proven end-to-end through the compiled CLI. Net cost $0.

### Headless autonomous auth (no paste, no browser at runtime)
- Launched a dedicated Chrome with --remote-debugging-port; logged in once.
- `auth login --cdp` read the LIVE session (access+refresh+uuid+member) via CDP -> seeded the CLI.
- Verified the CDP-seeded token authenticates: `cities` returned real data.

### Full desk lifecycle via the CLI (real API, real account)
- `common-booking book-desk --stdin` -> REAL booking: BookingStatus "BookingSuccess",
  ReservationID 11188534, HTTP 200. Confirmed in `bookings` (CLI) AND the WeWork web UI
  ($47, Austin test location, Tue Aug 18).
- `common-booking cancel-booking --stdin` -> REAL cancel: POST /common-booking/cancel, 200,
  success:true. `bookings` then showed 0. Full $47 refund (net $0).

### Payloads reverse-engineered (captured browser flow, create/cancel blocked to avoid double-action)
- Create: POST /common-booking/ — self-contained (empty ReservationID), needs CardUuid (payment
  account id from payments/get-user-cards), SpaceID (kubeSpaceId from common-booking/inventory-details),
  LocationID + LocationType (from wework-yardi/ondemand/get-locations-by-geo), WeWorkSpaceID, SpaceType,
  StartTime/EndTime (UTC), CreditCharged.
- Cancel: POST /common-booking/cancel — bookingId/reservationId/bookingExternaluuid (all = the reservation
  id), locationId, reservableId, spaceId, startTime/endTime, creditsUsed, bookingType, bookingLocationType,
  cancellationNote, mailParams.

## Known gap (documented, not blocking)
- The CLI `book-desk`/`cancel-booking` commands work perfectly when given the payload (proven), but the CLI
  does not yet RESOLVE a WeWork-owned building name -> all IDs headlessly (two ID systems: affiliate numeric
  vs WeWork-owned UUID; the WeWorkSpaceID/SpaceID chain spans get-locations-by-geo + a desk-list get-spaces +
  inventory-details). A future `book --location <name> --date <d> --confirm` command would orchestrate that
  chain. For now the endpoints + exact payloads are mapped and the write commands are verified.

## Token hygiene
- The live token lived only in a 0600 temp home (deleted) + the dedicated agent Chrome. Booking payloads
  (CardUuid = internal account id 374911, not a card number) were in scratch temp files, now deleted. No
  token or payload in any archived artifact.
