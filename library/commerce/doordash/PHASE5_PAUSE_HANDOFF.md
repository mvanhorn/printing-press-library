# DoorDash PP CLI — Pause Handoff

Paused: 2026-05-13 evening ET
Owner: Herman

## Current phase

- Phase 1: complete — persistent workspace/spec/skeleton exists.
- Phase 2: complete — sniffed/spec/generated skeleton path works.
- Phase 3: complete for read-only — PinchTab cookie import + CycleTLS transport works.
- Phase 4: complete — live read-only commands verified.
- Phase 5: complete — dry-run and guards work; live guarded add/remove now works with the refreshed/stronger DoorDash session; final cart cleanup verified on 2026-05-14T11:52Z.
- Phase 6: complete for live preview — checkout preview returns fees/totals with `safety.placesOrder=false`; raw preview containing address/payment metadata was deleted after sanitizing.
- Phase 7: implemented safety gate only — do not live-place an order without bricenice17 explicitly approving exact store/items/address/payment/tip/total.

## Important paths

Active CLI command:

```bash
doordash-pp-cli
```

Active implementation:

```text
/home/hermes/printing-press/sources/doordash-mcp-ashah360
```

Project/status workspace:

```text
/home/hermes/printing-press/library/doordash-pp-cli
```

DoorDash session file:

```text
/home/hermes/.doordash-mcp/session.json
```

Security:

- Do not print cookie values.
- Session dir/file should remain `0700`/`0600`.

## Verified complete

Phase 4 live read-only smoke passed via:

```bash
/home/hermes/printing-press/library/doordash-pp-cli/scripts/smoke-doordash-phase4-live-readonly.sh
```

Verified read-only summary:

- `doctor`: authenticated=true, csrf_present=true
- `search pizza`: 5 stores, first Pizza Hut
- `menu --store-id 2418408`: Pizza Hut, 11 categories, 63 items
- `item-options --store-id 2418408 --item-id 35292395509`: Personal Pan Pizza®, 4 option groups
- `convenience-search --store-id 23321373 milk`: 15 items
- `orders recent --limit 3`: succeeds, current session returns 0
- `addresses list`: succeeds, current session returns 0
- `payment-methods list`: succeeds, current session returns 0
- `cart show`: succeeds, current session returns 0

Phase 5 safe layer verified:

- `cart show --json` works and returns empty list after cleanup.
- `cart add --dry-run --json` works and does not mutate.
- `cart remove --dry-run --json` works and does not mutate.
- `cart add` without `--yes` refuses before mutation.
- `cart remove` without `--yes` refuses before mutation.

Phase 5 live guarded cart lifecycle verified 2026-05-14T11:52Z:

- Pre-test store-scoped cart count for Pizza Hut store `2418408`: 0.
- `cart add --store-id 2418408 --item-name 'Garlic Dip' --quantity 1 --yes --json` returned cart ID, cart item ID, menu item ID, item quantity 1, subtotal 104 cents.
- Store-scoped `cart show --store-id 2418408 --json` showed one visible `Garlic Dip` item.
- `cart remove --cart-id <test-cart> --item-id <test-item> --yes --json` returned `ok: true`.
- Final store-scoped cart count returned to 0.

Phase 6 live checkout preview verified 2026-05-14T11:52Z:

- Created a temporary Pizza Hut `Garlic Dip` cart, ran `checkout preview --cart-id <test-cart> --tip-cents 0 --json`, then removed the item.
- Sanitized result: restaurant Pizza Hut, one item, address present, payment method present, three fee lines, totals present, `safety.placesOrder=false`.
- Raw preview included address/payment metadata, so it was kept mode `0600` only long enough to sanitize and then deleted.

## What happened during controlled live Phase 5 test

bricenice17 approved finishing Phase 5. A controlled add/remove was attempted with no checkout/order/payment/address operations.

Initial guard:

- `doctor` was green.
- `cart show` returned empty list.

Attempt 1:

- Item: Pizza Hut `Personal Pan Pizza®`
- Result: add failed safely with GraphQL error: `Please select at least 1 options for Pizza Crust`.
- Cart remained empty.

Attempt 2:

- Item: Pizza Hut `Garlic Dip`
- Reason: no required options, low-cost test item.
- `cart add --yes` returned success with a cart ID and subtotal 104.
- But `cart show` immediately afterward still returned `[]`.
- Store-scoped `listCarts('2418408')` also returned `[]`.
- `detailedCartItems` and `deleteCart` against the returned cart ID failed with DoorDash error: `unexpected cart operation from lite guest`.

Initial interpretation, now superseded by the refreshed-session verification above:

- The earlier PinchTab-imported DoorDash session was sufficient for read-only GraphQL but not for reliable cart lifecycle operations.
- At that time doctor reported `consumer_id_present: false`, which likely caused the lite-guest cart behavior.
- This blocker cleared after the active session began reporting `consumer_id_present: true`; guarded add/remove and checkout preview now pass without order placement.

## Current blocker

No current Phase 5/6 blocker as of 2026-05-14T11:52Z. The previous lite-guest/cart-visibility blocker cleared after the active session began reporting `consumer_id_present: true`; guarded live add/remove and checkout preview now pass with cleanup verified.

Remaining safety gate:

1. Live order placement is intentionally not tested and must stay disabled unless bricenice17 explicitly approves the exact store/items/address/payment/tip/total.
2. Do not set `ALLOW_DOORDASH_ORDERING=1` except for that explicitly approved exact order.
3. Do not ask bricenice17 to paste DoorDash passwords, MFA, cookies, or tokens in chat.

## What not to do tomorrow

- Do not place orders.
- Do not set `ALLOW_DOORDASH_ORDERING=1` without exact bricenice17 approval.
- Do not ask bricenice17 to paste DoorDash passwords, MFA, cookies, or tokens in chat.

## Tracker state

Todoist / Paperclip as of 2026-05-14T11:56Z:

- Phase 4 is complete: live read-only smoke passes.
- Phase 5 is complete: dry-run/no-yes guards pass, and the controlled add/remove lifecycle passed with cleanup verified at 2026-05-14T11:52Z.
- Phase 6 is complete for preview: checkout preview returns totals/fees and reports `safety.placesOrder=false`.
- Phase 7 is implemented as a disabled-by-default safety gate only. Live order placement remains intentionally untested and must not be run without bricenice17 explicitly approving the exact store/items/address/payment/tip/total and deliberately setting `ALLOW_DOORDASH_ORDERING=1`.
- Phase 8 install/test verification passed at 2026-05-14T11:56Z with absolute installed wrapper checks, Go tests/builds, read-only live smoke, cart dry-run/no-yes guard, and order-placement gate rejection.
- PER-40 remains blocked until bricenice17 explicitly approves an exact live smoke/order scenario.
