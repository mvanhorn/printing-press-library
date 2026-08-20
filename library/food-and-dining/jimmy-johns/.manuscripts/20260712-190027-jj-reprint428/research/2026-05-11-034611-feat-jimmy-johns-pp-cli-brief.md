# Jimmy John's CLI Brief

## API Identity
- **Domain:** Fast-casual sandwich ordering. Jimmy John's operates ~2,700 US locations and is famous for its "Freaky Fast Delivery" speed promise (typical SLA: 2–5 minutes off-premise).
- **Users:** Households re-ordering favorites, office lunch orderers, late-night students, hungry commuters who want a sub on the way home.
- **Data profile:** Stores (locations, hours, delivery zones), menu (subs, sides, drinks, modifiers like bread/toppings/unwich), carts, orders, "Freaky Fast Rewards" loyalty points, saved addresses, payment methods.

## Reachability Risk
- **Medium.** Jimmy John's serves the SPA from `www.jimmyjohns.com` behind Cloudflare. Direct `curl` against the page returns 200 but the API surface (`www.jimmyjohns.com/api/*`) is dynamically constructed in `/assets/index-*.js`. JS bundles do not statically inline the API paths — they're composed at runtime. Bot-protection (Akamai/Cloudflare combined fingerprint) is plausible based on the `__cf_bm` cookie on the order subdomain. Reverse engineering requires a logged-in browser session HAR.
- No reported abuse/lawsuit posture against scrapers as of this writing; JJ has no published API terms.

## Top Workflows
1. **"Order my usual"** — Pull the household's most recent order, revalidate prices, send to nearest store.
2. **"Find a store that can deliver to me right now"** — Address → store + delivery zone check, with current ETA.
3. **"Build an office order"** — Sized cart for N people, mixed sandwiches, sides, drinks, split-payment friendly.
4. **"Track my Freaky Fast order"** — Live order status from "Making it" through "Out for delivery" to "Delivered" with ETA.
5. **"Check what's free with my rewards"** — Points balance, available redemptions, expiry awareness.

## Table Stakes (from competitors and incumbent web/iOS UI)
- Store finder (ZIP/address → list with distance, hours, pickup/delivery flag)
- Menu browse (categories: Originals, Favorites, Plain Slims, Wraps, Sides, Drinks, Cookies, Catering)
- Sandwich customization (bread type: 8" original, 16" giant, slim, club, wrap, unwich; toppings; modifiers)
- Cart manipulation (add/remove/modify items, apply promo code, choose pickup vs delivery)
- Order placement with payment (saved card, Apple Pay, guest checkout)
- Order history + receipt access
- Freaky Fast Rewards: points balance, available rewards, history
- Saved addresses, saved payment methods
- Order tracking (status + ETA)
- Re-order from past order

## Data Layer
- **Primary entities:** `store`, `menu_category`, `menu_item`, `modifier`, `cart`, `order`, `order_status`, `rewards_account`, `reward`, `saved_address`, `saved_payment`.
- **Sync cursor:** stores rotate hours, menu is largely static but seasonal items change; orders are append-only per user. Sync `orders` by `updated_at DESC` since `last_sync_at`. Menu by store + last_seen.
- **FTS/search:** `menu_item` (name + description + ingredients), `store` (name + address + city), `order` (item names for "have I ordered the Vito before?").

## Codebase Intelligence
- *No public DeepWiki entry; no published SDK on npm or PyPI.*
- The web SPA's main bundle imports React, MobX, axios. Auth is cookie-based; the request defaults set `withCredentials: true` after login. No Bearer header observed in static analysis.
- Payment tokenization uses Vantiv/Worldpay's eProtect (script loaded from `request.eprotect.vantivcnp.com`).
- Apple Pay supported via Apple's SDK script.
- Content (marketing/menu copy) served from Contentful (`environmentBaseUrl` / `spaceBaseUrl`). The ordering surface is JJ-owned, not Olo.
- Likely API base: `https://www.jimmyjohns.com/api/v1` or similar — confirmed via HAR capture only.

## Source Priority
*Single source.* `www.jimmyjohns.com` is the primary and only source. No combo CLI.

## Product Thesis
- **Name:** `jj-pp-cli` (binary), `jimmy-johns` (slug).
- **Why it should exist:** No CLI or MCP wraps the Jimmy John's ordering API today. Households and office orderers want to re-order their usual quickly. Power users want a "freaky fast" ETA predictor that goes beyond what the app shows. Agents want a structured way to place lunch orders for a group with dietary filters. This CLI ships all three with a local SQLite store that compounds knowledge of menu changes, favorite items, and store reliability over time.

## Build Priorities
1. **`store list` / `store nearest`** — public store finder, no auth.
2. **`menu` / `menu cache`** — full menu for a store, cached locally.
3. **`auth login --chrome`** — read JJ cookies from active Chrome session to authenticate the CLI.
4. **`orders list` / `orders get` / `orders last`** — order history.
5. **`cart build` / `cart price` / `cart send`** — cart construction and submission.
6. **`rewards check`** — points balance + available redemptions.
7. **Transcendence:** `freaky-fast` (per-store ETA predictor), `closest-on-route`, `unwich-mode` (auto-convert any cart to lettuce wraps), `office-lunch --people N` (one-shot office order with dietary filters).

## Open Questions for User
- Confirm preferred CLI/binary name: `jj-pp-cli` (compact) vs `jimmy-johns-pp-cli` (consistent with the press convention).
- Do you have a Jimmy John's account, and are you willing to provide a HAR capture for endpoint discovery?
- Catering vs delivery vs pickup priority: which workflow matters most?
