## Summary

Fixes two live provider-drift failures in `table-reservation-goat`:

- Tock booking now keeps the legacy per-slot-button click path first, then falls back to Tock's newer experience-card layout by selecting the requested time from a combobox/listbox, choosing the correct experience card by party size, and clicking the follow-on "Book now" submit control.
- The Tock combobox fallback is now navigation-proof: before evaluating the fallback it verifies the active Chrome target is still the requested venue page, reattaches to an existing venue tab when possible, re-navigates or opens a fresh venue target when needed, and retries once on CDP `-32000` target navigation/closure errors.
- Tock agent/no-input live booking no longer prompts for CVC. Callers must pass `TRG_TOCK_CVC`; missing or invalid values return a typed `cvc_required` JSON outcome with the booking URL before Chrome is launched.
- OpenTable availability now treats the persisted-query identity as `hash + operationName`, persists both from the page-fired request, and builds direct GraphQL requests where body `operationName`, URL `opname`, and the harvested hash identity stay in sync.

The Tock drift error is now typed as `selector_drift` at the CLI boundary and includes a page-state hint with present comboboxes/options/book controls, plus challenge/login-wall booleans.

## Why

The current published CLI fails Tock booking for venues such as `barcelona-wine-bar-raleigh` because those pages no longer render per-time slot buttons. The same venue renders a party stepper, date picker, time combobox, experience cards ("Reservation" and group reservation cards), and "Book now" controls.

The first real run of the initial fix also found a headless-agent failure: stderr prompted for CVC in `--agent` mode, then stdout returned `selector_drift` with `about:blank` page-state because the inspected target had navigated or closed before the combobox fallback evaluated:

```text
Tock card-required venues need CVC re-entry per booking. Enter CVC (or press Enter to skip):
selector_drift: requested booking slot control not found ... combobox_layout_error=evaluating combobox booking layout: Inspected target navigated or closed (-32000) page_state={"url":"about:blank",...}
```

OpenTable's Apollo persisted-query gateway now validates that the operation name in the JSON body matches the URL `opname` and the registered name for the persisted-query hash. Hash-only self-healing can leave the CLI with a current hash but a stale operation-name assumption.

## Changes

- Added `clickRequestedTockBookingControl`, with:
  - legacy `clickSlotByTimeText` first,
  - combobox/listbox time selection fallback,
  - party-size scoring for standard vs group experience cards,
  - separate submit-button click when the card-level "Book now" does not navigate,
  - venue-target recovery before combobox fallback,
  - one retry for CDP `-32000` target navigation/closure,
  - page-state hint on selector drift that re-resolves the live venue target.
- Added no-prompt Tock CVC handling for `--agent` / `--no-input`; `TRG_TOCK_CVC` is accepted, invalid/missing CVC returns `cvc_required`, and the value is never logged.
- Added `tock.ErrSlotControlNotFound` and mapped it to `selector_drift` in `book` JSON output.
- Replaced OpenTable availability hash-only persistence with a backwards-compatible query identity store: existing `{ "hash": ... }` files still load and default to `RestaurantsAvailability`.
- Harvested OpenTable operation names from Chrome/CDP request bodies and URL `opname`.
- Added a local GraphQL guard that rejects body/URL operation-name mismatches before sending a request.
- Retained support for both direct `data.availability[]` and page-fired `data.restaurantsAvailability.availabilities[]` response shapes.
- Added `.printing-press-patches/tock-combobox-and-opentable-opname-consistency.json`.

## Tests

```text
go test ./internal/source/opentable
go test ./internal/source/tock
go test ./internal/cli
go test ./...
go vet ./...
git diff --check
go build -o /tmp/table-reservation-goat-pp-cli-fix2 ./cmd/table-reservation-goat-pp-cli
```

All passed.

## Live read-only verification

Built binary: `/tmp/table-reservation-goat-pp-cli-fix2`

```text
/tmp/table-reservation-goat-pp-cli-fix2 availability check opentable:1141492 --party 2 --date 2026-07-10 --time 19:00 --agent
```

Returned slots, not 409:

```json
{
  "available": true,
  "bookable_times": ["2026-07-10T18:30", "2026-07-10T18:45", "2026-07-10T19:00", "2026-07-10T19:15", "2026-07-10T19:30"],
  "cached_at": "2026-07-08T18:08:04-04:00",
  "network": "opentable",
  "slot_at": "2026-07-10T18:30",
  "source": "cache_fallback",
  "stale": true
}
```

Note: OpenTable's final rerun returned slot data from the CLI stale-cache fallback because Akamai blocked that live fetch after the hash+operation identity had already been harvested. An earlier run during this fix session returned the same slot set as a fresh direct response after identity harvest. No 409 was observed after the fix.

```text
/tmp/table-reservation-goat-pp-cli-fix2 availability check resy:72502 --party 2 --date 2026-07-10 --time 19:00 --agent
```

Returned 24 slots, earliest `2026-07-10T16:00`.

```text
/tmp/table-reservation-goat-pp-cli-fix2 availability check tock:barcelona-wine-bar-raleigh --party 4 --date 2026-07-10 --time 18:15 --agent
```

Returned 30 slots, including `2026-07-10T18:15`; earliest `2026-07-10T16:00`.

```text
/tmp/table-reservation-goat-pp-cli-fix2 book tock:barcelona-wine-bar-raleigh --date 2026-07-10 --time 18:15 --party 4 --dry-run --agent
```

Returned dry-run envelope only:

```json
{
  "book_url": "https://www.exploretock.com/barcelona-wine-bar-raleigh?date=2026-07-10&size=4&time=18:15",
  "source": "dry_run"
}
```

## Safety

No real booking command was run. `book` was only invoked with `--dry-run`; `TRG_ALLOW_BOOK` was not set. No session file contents, card data, or CVC values were printed.

## Human follow-up

- Push branch `fix/tock-combobox-and-ot-persisted-query`.
- Open the PR using this description.
- Live-test one real, cancelable Tock booking after review, then cancel it through the venue/provider UI if appropriate.
