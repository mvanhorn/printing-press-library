# DoorDash CLI — Printing Press Prototype Status

## Current status

Installed local command:

```bash
doordash-pp-cli
```

Important install note: the active PATH commands are intentionally Node/CycleTLS wrappers. Stale generated Go binaries in active PATH locations caused 403s against DoorDash in browser-facing calls. They were backed up under `/home/hermes/.hermes/backups/doordash-pathfix-*`, `/home/hermes/.hermes/backups/doordash-profile-pathfix-*`, or `/home/hermes/backups/doordash-pp-cli-install/`, then replaced with wrappers to the canonical clean repo build:

```text
/home/hermes/projects/doordash-pp-cli/active-wrapper/dist/cli.js
/home/hermes/projects/doordash-pp-cli/active-wrapper/dist/index.js
```

The wrappers export `HOME=/home/hermes` before launching Node so Paperclip/Herman profile-home isolation still uses the protected shared session at `/home/hermes/.doordash-mcp/session.json`.

Source/prototype implementation:

```text
/home/hermes/projects/doordash-pp-cli/active-wrapper
```

Printing Press-generated skeleton and curated GraphQL spec artifacts:

```text
/home/hermes/printing-press/library/doordash-pp-cli
/home/hermes/printing-press/library/doordash-pp-cli/spec.yaml
/home/hermes/printing-press/library/doordash-pp-cli/references/doordash-graphql-spec.yaml
/home/hermes/printing-press/library/doordash-pp-cli/references/doordash-graphql-spec-manifest.json
```

Phase 2 / PER-33 validation:

```text
Curated GraphQL operations: 19
Generated graphql create-* CLI commands: 19
Build toolchain: /home/hermes/.local/bin/go (Go 1.26.3)
Verification: go test/build with isolated Go cache, graphql command count check
```

Phase 3 / PER-34 transport-session validation:

```text
Transport: Go net/http with persistent cookie jar and HTTP/2 enabled
Session file: ~/.config/doordash-pp-cli/session_cookies.json (0600, values never printed)
Browser-context headers: Accept, Origin, Referer, User-Agent, X-Requested-With, optional X-CSRFToken
Verification: doctor reaches https://www.doordash.com; auth status reports safe metadata only
```

Phase 4 / PER-35 read-only command validation:

```text
Curated commands: search, menu, item-options, convenience-search, recent-orders, addresses, payment-methods
Verification: go test -vet=off ./internal/cli -run 'TestReadOnlyCommandsRegistered|TestParse' passed; full smoke passed
Safety: commands call read-only GraphQL endpoints and expose --variables overrides; no cart/order mutation paths
```

Phase 6 / PER-37 checkout preview validation:

```text
Active wrapper command: doordash-pp-cli checkout preview --cart-id <id>
Generated skeleton command: doordash-pp-cli cart preview --variables '{...}'
Behavior: calls checkout/fee-tally style preview APIs only; never calls createOrderFromCart
Output: summarizes restaurant, items, address, payment method, tip, fees, and total when present
Latest live verification: 2026-05-14T11:52Z — created one guarded Pizza Hut Garlic Dip test cart, ran checkout preview with tip_cents=0, confirmed safety.placesOrder=false, then removed the item and verified store-scoped cart count returned to 0. Raw preview contained address/payment metadata and was deleted after writing a sanitized summary.
```

Phase 8 / PER-39 install/test validation:

```text
Smoke script: ./scripts/smoke-doordash-cli.sh
Coverage: go test, go vet, CLI/MCP builds, doctor/help, 19 generated GraphQL commands, live-order gate rejection, gateway PATH visibility
Latest verification: 2026-05-14T11:56Z — absolute installed wrapper doctor/help passed; doctor reports authenticated=true, csrf_present=true, consumer_id_present=true; go test -vet=off ./... passed with `/home/hermes/.local/bin/go`; Go CLI/MCP builds passed; live read-only smoke passed; active wrapper cart add `--dry-run` passed; cart add without `--yes` rejected before mutation; order placement rejected without `ALLOW_DOORDASH_ORDERING=1`. The earlier 2026-05-14T11:52Z controlled live Pizza Hut Garlic Dip add/remove and checkout preview also passed with final store-scoped cart count returned to 0, safety.placesOrder=false, and raw address/payment preview deleted after sanitizing. Earlier 2026-05-13T23:34Z checks also had go vet ./... passed and make build-all/install-mcp succeeded.
Verifier caveat: `printing-press verify --dir .` currently fails before testing because verifier assumes Go files directly under `cmd/`; this repo uses nested `cmd/doordash-pp-cli` and `cmd/doordash-pp-mcp`. Manual Go/smoke checks above are the Phase 8 shipcheck substitute until verifier supports nested cmd layout.
Install: go install refreshed /home/hermes/go/bin/doordash-pp-cli and /home/hermes/go/bin/doordash-pp-mcp.
Gateway visibility: live hermes-gateway.service PATH includes /home/hermes/.local/bin and /home/hermes/go/bin.
Node tests: skipped because this Go CLI repo has no package.json.
```


Phase 10 / PER-61 read-only polish validation:

```text
Latest verification: 2026-05-14T15:53Z — active installed wrapper doctor/help passed; orders recent --limit 2 now returns itemDetails with item names/quantities and a human items summary; stores rank Thai --limit 5 returns ratingValue, normalized rating display, delivery estimate, and price range; items search "chicken parmesan" --store-query "italian" separates cheapest sub/sandwich from cheapest dinner/pasta-style meal. No cart, checkout, or order placement commands were run.
Changed active Node/CycleTLS wrapper files: src/api/orders.ts, src/api/menu.ts, src/cli.ts and rebuilt dist/* with npm run build. Generated Go skeleton verification also passed with /home/hermes/.local/bin/go: go test -vet=off ./..., go build ./cmd/doordash-pp-cli, and go build ./cmd/doordash-pp-mcp.
Sanitized samples: recent orders returned 2 orders with itemDetails_count 1 and 4; Thai ranking returned 5 stores with top visible ratings 4.6/5 (200+)/(500+) and delivery windows 38-58 min; chicken parmesan search scanned 5 stores, returned 10 matches, and identified cheapest dinner/pasta as Carrabba's Chicken Parmesan ($16.49) plus cheapest sub/sandwich as Pizzitalia's Chicken Parmigiana Sub ($13.99).
```

## What works now without printing DoorDash secrets

- `doordash-pp-cli --help`
- `doordash-pp-cli doctor --json`
- `doordash-pp-cli --help` as runtime command truth
- `doordash-pp-cli login --json` / `verify --code ... --json` for approved interactive login flow; do not paste secrets in chat
- generated Go skeleton preserves 19 curated low-level GraphQL commands, but the active Hermes wrapper exposes the safer curated command surface
- read-only curated commands: `search`, `stores rank`, `menu`, `items search`, `item-options`, `convenience-search`, `recent-orders`, `addresses`, `payment-methods`
- checkout/fee preview under `checkout preview`; it never calls `createOrderFromCart`
- guarded order placement help/safety gates under `order place`; no live order runs without explicit bricenice17 approval and `ALLOW_DOORDASH_ORDERING=1`

## Commands implemented

Auth/session:

```bash
doordash-pp-cli doctor --json
doordash-pp-cli login --json
doordash-pp-cli verify --code 123456 --json
```

Low-level GraphQL skeleton:

```bash
doordash-pp-cli graphql --help
doordash-pp-cli graphql create-add-cart-item --help
doordash-pp-cli graphql create-autocomplete-facet-feed --help
doordash-pp-cli graphql create-checkout --help
doordash-pp-cli graphql create-consumer-order-cart --help
doordash-pp-cli graphql create-convenience-search-query --help
doordash-pp-cli graphql create-create-order-from-cart --help
doordash-pp-cli graphql create-delete-cart --help
doordash-pp-cli graphql create-detailed-cart-items --help
doordash-pp-cli graphql create-get-has-new-notifications --help
doordash-pp-cli graphql create-get-open-carts-count --help
doordash-pp-cli graphql create-item-page --help
doordash-pp-cli graphql create-list-carts --help
doordash-pp-cli graphql create-poll-order-payment-status --help
doordash-pp-cli graphql create-promo-sticky-footer --help
doordash-pp-cli graphql create-remove-cart-item-v2 --help
doordash-pp-cli graphql create-storepage-feed --help
doordash-pp-cli graphql create-total-fee-tally --help
doordash-pp-cli graphql create-update-cart-item-v2 --help
doordash-pp-cli graphql create-validate-consumer-address-with-address-link-id --help
```

Cart/order safety gate:

```bash
doordash-pp-cli order place --cart-id <id> --confirm "PLACE ORDER" --max-total-cents <cents> --max-delivery-fee-cents <cents> --max-tip-cents <cents>   # still blocked without ALLOW_DOORDASH_ORDERING=1
ALLOW_DOORDASH_ORDERING=1 doordash-pp-cli order place \
  --cart-id <id> \
  --confirm "PLACE ORDER" \
  --tip-cents <cents> \
  --max-total-cents <cents> \
  --max-delivery-fee-cents <cents> \
  --max-tip-cents <cents> \
  --payment-card-id <saved-id>
```

Do not run live ordering without bricenice17 explicitly approving the exact store/items/address/payment/tip/total.

## Key fixes discovered

- CycleTLS prototype defaulted to local port `9119`. On this Hermes LXC that port was already occupied, causing `Failed to initialize CycleTLS: undefined`. The prototype patch in `/home/hermes/printing-press/sources/doordash-mcp-ashah360/src/client/http.ts` picks a free localhost port before initializing CycleTLS.
- The generated Go transport had HTTP/2 effectively disabled. DoorDash negotiated h2 and returned HTTP/2 frames, which were parsed as HTTP/1.x and caused `malformed HTTP response`. `internal/client/client.go` now keeps HTTP/2 enabled with `ForceAttemptHTTP2 = true`.

## Verification run

Passed locally at 2026-05-13T23:02Z with `/home/hermes/.local/bin/go`:

```bash
cd /home/hermes/printing-press/library/doordash-pp-cli
export PATH="/home/hermes/.local/bin:$PATH"
export GOCACHE=/tmp/per33-go-build-cache
go test -vet=off ./...
go build -o /tmp/doordash-pp-cli-current ./cmd/doordash-pp-cli
go build -o /tmp/doordash-pp-mcp-current ./cmd/doordash-pp-mcp
/tmp/doordash-pp-cli-current doctor --json
/tmp/doordash-pp-cli-current auth status --json   # safe session metadata only; no cookie values
```

## PinchTab cookie import status

bricenice17 suggested using the logged-in DoorDash session from PinchTab. This worked for read-only search after importing sanitized DoorDash-domain cookies from the PinchTab `default` profile into:

```text
/home/hermes/.doordash-mcp/session.json
```

Security handling:

- Cookie values were not printed in chat/log summaries.
- Session dir is `0700`; session file is `0600`.
- Existing session files are backed up before overwrite.

DoorDash cookie shape observed from PinchTab in May 2026 differs from the original MCP assumptions:

- Present: `dd_session_id`, `ddweb_session_id`, `XSRF-TOKEN`, `cf_clearance`
- Absent: `ddweb_token`, `csrf_token`

Local patch in `/home/hermes/printing-press/sources/doordash-mcp-ashah360/src/client/session.ts`:

- `isAuthenticated()` accepts `ddweb_token || dd_session_id || ddweb_session_id`.
- `getCsrfToken()` accepts `csrf_token || XSRF-TOKEN`.

Verified live read-only command:

```bash
doordash-pp-cli search pizza --json
```

Returned real stores including Pizza Hut, Pizza World, Pizza Famiglia, etc.

## Live validation status — 2026-05-13

Phase 4 read-only commands now live-verify through the active installed command:

```text
doctor: authenticated=true, csrf_present=true
search pizza: 5 stores; first Pizza Hut
menu --store-id 2418408: Pizza Hut, 11 categories, 63 items
item-options --store-id 2418408 --item-id 35292395509: Personal Pan Pizza®, 4 option groups
convenience-search --store-id 23321373 milk: 15 items
orders recent --limit 3: command succeeds; current session returned 0 orders
addresses list: command succeeds; current session returned 0 addresses
payment-methods list: command succeeds; current session returned 0 payment methods
cart show: command succeeds; current cart count 0
```

Phase 4 fixes:

- Initial `menu` timeout was not a DoorDash menu failure. Root cause was a stale Go binary earlier in PATH plus one bad shell test harness that piped CLI output into a Python heredoc. The Node/CycleTLS wrapper is now installed in `/home/hermes/go/bin/doordash-pp-cli` so `command -v` resolves to the working implementation.
- `orders recent` had GraphQL schema drift. Current DoorDash requires `getConsumerOrdersWithDetails(offset: Int!, limit: Int!)` and returns an array. The query now uses validated conservative fields: `id`, `orderUuid`, `createdAt`, `submittedAt`, `store { name }`, `grandTotal { displayString }`, `orders { id }`.

Phase 5 safe layer verified:

```text
cart add --dry-run: succeeds, no DoorDash mutation
cart remove --dry-run: succeeds, no DoorDash mutation
cart add without --yes: blocked before mutation
cart remove without --yes: blocked before mutation
```

Current blocker / pause state:

- Previous blocker resolved 2026-05-14T11:52Z: current DoorDash session now reports `consumer_id_present: true`, guarded live cart add/remove works, and checkout preview works without order placement.
- Guarded live cart test: Pizza Hut `Garlic Dip` add with `--yes` returned cart/item IDs, store-scoped `cart show --store-id 2418408` showed one visible item, `cart remove --yes` succeeded, and final store-scoped cart count returned to 0.
- Checkout preview test: a temporary Pizza Hut `Garlic Dip` cart was created, `checkout preview --tip-cents 0 --json` returned one item, address/payment present, three fee lines, totals present, and `safety.placesOrder=false`; raw preview with address/payment metadata was deleted after writing a sanitized summary.
- Do not place orders. `order place` remains blocked unless bricenice17 explicitly approves the exact store/items/address/payment/tip/total and `ALLOW_DOORDASH_ORDERING=1` is deliberately set.
- Do not ask bricenice17 to paste DoorDash passwords, MFA, cookies, or tokens in chat.

Pause handoff:

```text
/home/hermes/printing-press/library/doordash-pp-cli/PHASE5_PAUSE_HANDOFF.md
```

Never paste DoorDash password, MFA, or cookie values into Telegram.
