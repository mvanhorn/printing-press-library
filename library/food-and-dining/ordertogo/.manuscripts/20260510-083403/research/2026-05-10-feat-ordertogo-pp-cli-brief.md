# OrderToGo CLI Brief

## API Identity

- Domain: pickup ordering at OrderToGo.com (multi-tenant restaurant ordering platform; "$0 additional fee" pitch)
- Users: customers placing pickup orders, primarily small chains in the Seattle area on this platform
- Data profile: restaurants (multi-tenant, one DB per restaurant), menus (categories + items with options), orders (items + tip + payment), cart (per-restaurant client state in localStorage), customer (one Firebase phone-OTP identity)
- Primary user (this run): Matt Van Horn, default restaurant Mix Sushi Bar Lincoln Square (slug `mixsushibarlin`, restid `72`), saved payment MASTERCARD-2126

## Reachability Risk

None. Direct HTTP returns 200 OK with no Cloudflare, no challenge, no bot detection.
`probe-reachability` mode `standard_http`. Cookie-based session works for all
read endpoints. Place endpoint requires Braintree client-side nonce.

## Top Workflows

1. **Reorder my usual** (the user's #1 ask) — find last order at default restaurant, recompose cart from same items, validate against budget, place via headless Braintree handoff.
2. **Browse menu and order something specific** — list restaurants, view menu, add items (with options like "No Ginger"), set tip, place order.
3. **Spending analytics** — track total spent, frequency, favorite items across full local order history (we sync `getmicmeshorders` once and the rest is local SQL).
4. **Order tracking** — fetch `/trackorder/<orderToken>` to get pickup status (received → preparing → ready → picked up).
5. **Multi-location chain handling** — Mix Sushi Bar has multiple locations (Lincoln Square, Crossroads Mall); CLI defaults to last-ordered location but supports `--restaurant <slug>` to pivot.

## Table Stakes

- Authentication via cookie import from Chrome (Firebase phone-OTP login is the auth flow on the website; we shortcut by reading cookies)
- Restaurant search by location code (e.g. `sto`)
- Full menu fetch with options/modifiers
- Cart compose / show / clear (mirrors localStorage shape)
- Order pre-validate (`POST /m/api/orders` returns `{token}`, includes tax)
- Order place (Braintree DropIn → `POST /m/api/orders/braintreeCheckout`)
- Order history list and detail
- Order tracking (live status)

## Data Layer

- **Primary entities**: restaurant, menu_category, menu_item, item_option, order, order_item, cart_state, customer_profile
- **Sync cursor**: `getmicmeshorders` returns full history, page-cursored. Sync replaces local copy on each `sync --orders`.
- **FTS/search**: full-text on menu_item.name, menu_item.optionsstr, restaurant.name, order_item.name, special_instructions
- **Local-first analytics**: total spent, average order, days since last order, top items, top restaurants — all SQL over local store

## Codebase Intelligence

- Source: direct read of `/javascripts/miyu_c/order.m.togo.js`, `/javascripts/menu.m.common.js`, `/javascripts/miyu_c/order.base.mobile.js`
- Auth: cookie-based session (httpOnly), no Authorization header. The `__requestid` header on POSTs is an idempotency key (epoch_ms + random int).
- Data model: cart state mirrors localStorage `order.rest<slug><id>`, with items[], orderToken, sortedItems[]. Order POST body contains `restid, items` for pre-validate; `nonce, tip, customerphone, customername, deliveryfee, restname, orderdetails, restid, database` for place.
- Rate limiting: not observed; one Google Analytics POST returned 503 (likely unrelated rate limit at GA, not OrderToGo).
- Architecture: server-rendered HTML + jQuery + jsviews templates (`/templates/*.tmpl`), Bootstrap 3, Firebase Auth on the login surface only, Braintree DropIn for payment, jsencrypt for any pre-encryption of card-adjacent fields.

## User Vision

Personal sushi-ordering agent flow:

> "I want to be able to order my regular sushi order from Mix Sushi Bar. Ideally I want to tell my agent 'order sushi' and it'll find my previous order, or re-create it from scratch because it knows it, and it efficiently places the order with a budget limit — don't go above $25 without confirmation."

Expansion (added mid-run):

> "Also know how to shop / add to cart / other restaurants too."

So the CLI is general-purpose for any OrderToGo restaurant, with the "usual" reorder pattern as the headline command, layered safety (`--max <dollars>`, `--confirm`, `--confirm-over-budget`), and a fully autonomous headless place via chromedp + Braintree DropIn drive.

## Source Priority

Single-source CLI. Primary is OrderToGo's mobile API (`/m/api/*`). No combo
CLI gate.

## Product Thesis

- Name: `ordertogo-pp-cli`
- Why it should exist: The web app has "Order Again" but it requires a browser session, manual taps, and no budget gate. There is zero CLI / MCP / SDK presence for OrderToGo today. An agent-native CLI lets you say "order my usual under $25" once and have it happen, with a single Place Order tap if headless fails. Local order history makes "what's my usual" answerable offline, and SQL analytics turn pickup ordering into a tracked habit.

## Build Priorities

1. **Auth + read path**: cookie import via `auth login --chrome`, `restaurants find/show`, `menu`, `orders list/show`, `last-order`, `order track`. All pure HTTP+cookie. **Non-negotiable.**
2. **Local store + sync**: SQLite store for restaurants, menus, items, orders. `sync --orders` calls `getmicmeshorders` and rebuilds order history; `sync --menu <slug>` caches a menu locally. FTS5 on item names. **Non-negotiable.**
3. **Cart compose + validate (`order plan`)**: build a cart from items or from `--reuse-last`, call `POST /m/api/orders` to get token + validate, show subtotal/tax/tip/total. Hard cap at `--max`. Verify-env safe (no submit). **Non-negotiable.**
4. **Place order (`order place`)**: chromedp-driven headless Chrome that imports cookie session, navigates to pre-filled checkout, clicks Place Order, picks saved card in Braintree DropIn, posts via Braintree handler, waits for `/trackorder/<orderToken>` redirect. Multi-layer safety: `--confirm` required, `--max` required, `--confirm-over-budget` for over-cap, `__requestid` idempotency key, `PRINTING_PRESS_VERIFY=1` short-circuit. Fall back to opening visible Chrome with cart pre-populated if headless fails.
5. **Analytics + transcendence**: spend total, days since last order, top items, average tip, weekly cadence, alert when default restaurant is closed at request time, etc.
