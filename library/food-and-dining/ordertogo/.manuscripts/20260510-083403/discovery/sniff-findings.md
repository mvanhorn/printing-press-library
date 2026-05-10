# OrderToGo browser-sniff findings

Captured 2026-05-10 via chrome-MCP, logged in as Matt Van Horn.

## Architecture

- Server-rendered HTML + jQuery 2.2.1 + jsviews/jsrender templates loaded from `/templates/*.tmpl`
- Bootstrap 3.2.0, font-awesome 4.7, jquery.history, jquery.touchSwipe, fastClick, jsencrypt, w3css
- Firebase Auth (firebase-app + firebase-auth + firebaseui) for phone-OTP login
- Braintree DropIn for payment (Stripe.js loads but is not the payment path)
- Cart state lives entirely client-side in `localStorage` keyed by `order.rest<slug><id>`
- Cookie-based session auth (httpOnly), travels automatically via `cookies.txt`

## API surface (all observed)

| Endpoint | Method | Purpose | Body |
|---|---|---|---|
| `/m/api/restaurants/filter/<location_code>` | GET | restaurant list by location (e.g. `sto` = Seattle area) | - |
| `/m/api/restaurants/<slug>/menus/full` | GET | full restaurant menu | - |
| `/m/api/restaurants/<slug>` | GET | restaurant detail | - |
| `/m/api/getmicmeshorders` | POST | order history list | `{}` (empty for own orders) |
| `/m/api/getmicmeshorders` | POST | order detail by id | `{orderid, restname}` |
| `/m/api/orders` | POST | pre-create order, returns `{token}` | `{restid, items}` |
| `/m/generateBrainTreeClientToken` | GET | Braintree client token (string) | - |
| `/m/api/orders/braintreeCheckout` | POST | submit order with payment nonce | postParams (see below) |
| `/m/api/voidSelfOrder` | POST | cancel own order | TBD |
| `/m/api/party_invite` | POST | group order invite | TBD |
| `/m/api/mapping/geocode` | POST | geocoding (delivery only) | TBD |
| `/api/markPromotionUsed` | POST | promo code redemption | TBD |
| `/m/api/releaseWaiver_lookup`, `/m/api/releaseWaiver_byId` | various | dine-in waivers (irrelevant to togo) | - |
| `/api/ordertogoV2/notification/getNotificationCountUserUnread` | GET | unread notifications badge | - |
| `/trackorder/<orderToken>` | GET (HTML) | order tracking page | - |
| `/history` | GET (HTML) | order history page | - |

Idempotency header on most POSTs: `__requestid: <epoch_ms>_<random_int>`.

## Critical: payment is Braintree DropIn

- The "Place Order!" button does NOT POST a static body. It opens Braintree DropIn,
  asks user to confirm a payment method (saved card or new card), then on
  `onPaymentMethodReceived(payload)` posts `{nonce, ...}` to `/m/api/orders/braintreeCheckout`.
- A single-use nonce is generated client-side by Braintree. **A Go CLI cannot mint
  a fresh Braintree nonce without running Braintree.js.**
- Implication: a fully autonomous "submit a real order" command is infeasible
  from pure Go without re-implementing Braintree's client SDK (or finding a
  hidden vault/saved-card flow on the backend, which is unlikely to be
  accessible to a customer integration).

## Place-order body shape (from order.m.togo.js line 1410)

```json
{
  "nonce": "<braintree single-use nonce>",
  "tip": 0.90,
  "customerphone": "5551234567",
  "customername": "Matt",
  "deliveryfee": 0,
  "restname": "Mix Sushi Bar Lincoln Square",
  "orderdetails": {
    "items": [
      {
        "item_id": 19001,
        "optionsstr": null,
        "optionitemids": null,
        "optionItemIdsAndPrices": [],
        "price": 4.99,
        "taxrate": null
      }
    ],
    "subtotal": 17.98
  },
  "restid": 72,
  "database": "<tenant_db>"
}
```

Headers: `Content-Type: application/json`, `__requestid: <epoch>_<rand>`,
session cookies.

## Cart shape (from localStorage `order.restmixsushibarlin72`)

```json
{
  "items": [
    {
      "id": 19001,
      "item_id": "Nigiri Two Pieces04",
      "name": "Salmon",
      "price": 4.99,
      "optionsstr": "",
      "taxrate": null,
      "rewardsInfo": null,
      "optionitemobjects": [],
      "optionPriceList": {},
      "optionitemids": [],
      "togo": "0",
      "specialIns": ""
    },
    {
      "id": 19031,
      "item_id": "Roll08",
      "name": "Salmon and Avocado Roll 8PC",
      "price": 12.99,
      "optionsstr": "No Ginger",
      ...
    }
  ],
  "orderToken": null,
  "sortedItems": [...]
}
```

## User profile observed

- Account: Matt Van Horn, phone last 4 = 6052, payment MASTERCARD-2126
- Default restaurant: Mix Sushi Bar Lincoln Square (`mixsushibarlin`, restid 72)
- "Usual" order, repeated weekly (4/1, 3/30, 3/27, 3/24, 3/19, 3/18, 3/17, 3/12, 2/27...):
  - 1× Salmon (id 19001, item_id `Nigiri Two Pieces04`) $4.99
  - 1× Salmon and Avocado Roll 8PC (id 19031, item_id `Roll08`, opts: No Ginger) $12.99
  - 1× Reuseable Plastic Bag Option (id 19108, item_id `hide`, opts: No Bag) $0.00
  - subtotal $17.98 + tax $1.85 + tip $0.90 (5%) = $20.73
- Most recent order: #31 / 2258091, 4/1/2026 12:51pm, 17 reward points
- The URL the user pasted was Crossroads Mall (`mixsushibar/mesh`), but every order in
  history is at Lincoln Square (`mixsushibarlin/mesh`). Lincoln Square is the right
  default for "the usual."

## Multi-restaurant evidence

Homepage shows other restaurants (Nuodle Restaurant 2 Locations, Mini Moon at NE 8th St,
etc.). The `/m/api/restaurants/filter/<loc>` endpoint returns the list; `<loc>` is a
location code embedded in the page (`sto` for Seattle area).

## Implications for CLI design

1. `order plan` is fully implementable in Go — pure HTTP + cookie session, including
   the `POST /m/api/orders` cart pre-validate that returns an order token.
2. `order place` cannot be fully autonomous in Go due to Braintree client-side nonce.
   Two viable shapes:
   - **A. Hybrid handoff (recommended).** CLI prepares the cart in localStorage of
     the user's Chrome (via browser cookie + a generated URL with cart preloaded),
     then opens Chrome at the checkout page. User clicks Place Order in browser. CLI
     watches the user's order history (`getmicmeshorders`) for the new order and
     reports back when placed. Budget gate enforced before opening browser.
   - **B. Vault-only saved-card flow.** Search for a customer-vault endpoint that
     mints a nonce server-side from a saved card. Unlikely to exist as a public
     customer endpoint; probably internal-only.
3. Browse/menu/last-order/order-history all work as plain HTTP+cookie calls.
4. The cookie session is shared across the user's Chrome profile, so the CLI's
   `auth login --chrome` can import cookies via the standard chrome-cookie path.
