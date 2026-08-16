# WeWork CLI — Headless location listing + book command (agent-native)

## Goal
Let an agent resolve a fuzzy place ("book me a wework off Barton Springs") to a
concrete bookable location + all identifiers, then book — fully headless.

## Reverse-engineered the missing WeWork-owned chain (verified live in browser)
- `get-locations-by-geo(city, bounds)` -> buildings [{uuid, name, address, accountType(=LocationType), timeZone, spaceAvailabilityCount}].
- `get-spaces(locationUUIDs=ALL building uuids joined, type=0, locationType=1, platFormType=1, ...)` ->
  workspaces; join to buildings on `workspace.location.uuid == building.uuid`. workspace.uuid == WeWorkSpaceID,
  workspace.productPrice.price.amount == price, workspace.seat.available == availability.
  (Single-UUID / kube-numeric-id get-spaces calls return 0 — must pass the full set of building UUIDs.)
- `inventory-details(propertyId=LocationID, spaceId=WeWorkSpaceID, propertyType=LocationType, startDate, ...)` -> kubeSpaceId == SpaceID.
- `payments/get-user-cards` -> default card uuid == CardUuid.
- create (POST /common-booking/) self-contained (empty ReservationID).

## Built (durable, markerless, registered via registerNovelCommand)
- `internal/cli/wework_locations.go` — `locations --city <c> [--date --filter --available-only --bookable-only]`;
  shared resolveWeworkBuildings + resolveCityGeo/cityNameOnly/todayLocalDate helpers.
- `internal/cli/wework_book.go` — `book --city <c> (--location <name>|--location-id <id>) --date <d>
  [--start --end] [--confirm]`; resolveSpaceID (inventory-details), resolveDefaultCard (get-user-cards),
  localToUTC (IANA tz -> UTC + offset), buildBookingPayload. Preview by default; --confirm charges.
- Unit tests: localToUTC, pickBuilding, cityNameOnly, orDefault.

## Status
- Build/vet/tests pass; shipcheck all legs PASS except scorecard live_api_verification.
- Every endpoint + join in the chain was verified against live browser data; the create+cancel were
  already live-proven via book-desk/cancel-booking. A live end-to-end run of the NEW `locations`/`book`
  commands still wants a CDP re-seed (dedicated Chrome + one login) — pending.
- Committed: ff84d0f.
