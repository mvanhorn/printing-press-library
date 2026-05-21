# amazon-orders CLI Brief

## API Identity
- **Domain:** Amazon's authenticated buyer-side order surfaces (`amazon.com/gp/your-account/order-history`, `/gp/your-account/order-details`, `/cpe/yourpayments/transactions`, `/gc/balance`, `/gp/your-account/ship-track`).
- **No public API exists.** Amazon SP-API is for Sellers, not buyers; buyer order history is browser-only. There is no documented endpoint, no API key, no OAuth.
- **Users:** Power consumers, accountants doing VAT/expense reconciliation, indie devs running price/return analysis on their own purchase history, AI agents that need "what did I buy and when did it arrive" context for downstream automations.
- **Data profile:** Orders, items (ASINs), shipments + tracking, transactions (payments), gift card balance/transactions, returns, refunds, subscriptions. Heavily relational — orders → items → shipments → trackings → carrier URLs.

## Reachability Risk
- **High.** Amazon aggressively blocks scrapers. Confirmed evidence:
  - alexdlaird/amazon-orders README explicitly says "this package may break at any time"
  - alexdlaird notes Captcha challenges require manual entry on Python 3.13+
  - Amazon uses fetch-metadata validation, session-bound CSRF tokens, SameSite cookies, and bot fingerprinting
  - Plain `curl` against authenticated paths returns sign-in redirect HTML even with cookies if user-agent/headers don't match
- **Mitigation:** the entire competitive set (alexdlaird, marcusquinn, MaX-Lo, philipmulcahy/azad) uses one of three approaches: (1) Selenium/Playwright headed/headless browser with the user's actual login flow, (2) a Chrome extension that piggybacks on the live session, (3) cookie/session capture with browser-fingerprint-matching HTTP. Our generated CLI must use option (3) — Chrome cookie import + browser-fingerprint Surf transport — because options (1) and (2) violate the Printing Press rule against keeping a resident browser as the runtime.

## Top Workflows
1. **Sync recent order history into a local store and ask questions about it.** "What did I buy in the last 30 days?" "Which items are still in transit?" "Show me orders over $100 last quarter."
2. **Track in-flight deliveries.** "Which of my open orders are out for delivery today?" "Where is order 113-XXXX-YYYY?"
3. **Reconcile spending.** "How much did I spend at Amazon this year, broken down by month?" "Export everything as CSV for my accountant."
4. **Audit a specific order.** "Pull full details for order 113-XXXX-YYYY: items, prices, shipping address, payment method, shipment status, tracking number."
5. **Find the order for a thing I bought.** "When did I order that USB-C cable?" → search across all synced orders + items for the text.

## Table Stakes (every competitor has these)
- List orders with date filter (year, last-30-days, last-3-months) — alexdlaird, MaX-Lo, drewdaemon
- Order details with items + ASINs + prices — alexdlaird (`--full-details`), marcusquinn, azad
- Shipment / tracking export — marcusquinn (`export_amazon_shipments_csv`), azad (paywalled)
- Transactions / payments page — marcusquinn (`get_amazon_transactions`)
- Gift card balance + history — marcusquinn (`get_amazon_gift_cards_csv`)
- CSV export — marcusquinn, drewdaemon, azad
- Multi-region (16 Amazon TLDs) — marcusquinn
- 2FA / OTP support — alexdlaird (TOTP via env var)
- Browser session reuse (no re-login per command) — marcusquinn (Playwright), azad (extension)

## Codebase Intelligence
- Source: README + repo metadata for top 5 competitors (alexdlaird/amazon-orders, marcusquinn/amazon-order-history-csv-download-mcp, MaX-Lo/Amazon-Order-History, philipmulcahy/azad, drewdaemon/amazon_order_history_scraper).
- **Auth:** Cookie-based session. Critical cookies are `session-id`, `session-token`, `ubid-main`, `at-main`, `x-main`, `lc-main`. Many requests need a CSRF-style hidden form token (`anti-csrftoken-a2z` or `csrf` parsed from the page). Browser-fingerprint headers required (`User-Agent`, `Accept-Language`, `sec-ch-ua-*`).
- **Data model:**
  - `Order` (id `XXX-XXXXXXX-XXXXXXX`, placed_date, total, status, ship_to, items, shipments)
  - `Item` (asin, title, qty, unit_price, seller, condition, product_url)
  - `Shipment` (status, eta_date, delivered_date, tracking_id, carrier, tracking_url)
  - `Transaction` (date, payment_method, last4, amount, order_ids[])
  - `GiftCardTx` (date, kind, amount, balance, order_id?, claim_code?)
- **Rate limiting:** No documented limits. Empirically, ~10 order-detail fetches in quick succession trigger throttling or CAPTCHA. Concurrent requests must be limited; existing tools serialize at 0.5–4s per order.
- **Architecture insight:** Order history page renders ALL orders for a year/window in one HTML response with `?orderFilter=year-2025&startIndex=N&...` pagination. Order detail and ship-track pages are per-order. Transactions page (`/cpe/yourpayments/transactions`) uses an AJAX "Show More" infinite-scroll backed by a POST to a stateful endpoint — significantly harder to scrape than the listing pages.

## User Vision
- (from briefing) Walks and explores Amazon order history in detail. Get delivery updates, status, pricing, order placed date, delivery date, total, etc. Browser-sniffing required since there is no public buyer API.

## Source Priority
- Single source: `amazon.com` (and its TLD variants). No combo CLI, no spec inversion risk.
- Reachability: HIGH. Phase 1.7 will MUST run browser-sniff because no spec exists; Phase 1.9 reachability gate expects a 200 from `/gp/your-account/order-history` only with a logged-in cookie jar.

## Product Thesis
- **Name:** `amazon-orders-pp-cli`
- **Why it should exist:** Every existing tool gives you orders. None give you orders **plus a local SQLite store** that AI agents can query offline, search with FTS5, join across orders + items + shipments + transactions, and detect interesting patterns ("which items have I returned >2x", "which delivery dates slipped >3 days from estimate", "what's my recurring monthly spend at Amazon"). Existing tools dump CSVs and stop; we sync once and answer cross-cutting questions forever — without re-hitting the live site, without burning agent context on full HTML pages, without waiting for Playwright to spin up.

## Build Priorities
1. **Foundation: Chrome cookie import + browser-fingerprint HTTP transport.** Without a working authenticated session, nothing else functions.
2. **Sync surfaces:** orders (list), order-detail (items, shipments), transactions, gift-cards. Each goes into its own SQLite table.
3. **Match-and-beat the absorb manifest:** every feature competitors have, but agent-native (`--json`, `--select`, `--csv`, `--dry-run`, typed exit codes), offline once synced, and FTS5-searchable.
4. **Transcend:** the local-store novel features only we can do (delivery-slip detector, recurring-spend rollup, shopping-pattern queries, "where's my stuff" radar across orders).

## Reachability Plan
- Direct HTTP probe before generation MUST run (Phase 1.9). Expect 200 with a logged-in cookie jar; expect a sign-in redirect or 4xx without one. Either is acceptable evidence the surface is reachable; only an outright 5xx or DataDome/CAPTCHA wall is a HARD STOP.
- Browser-sniff in Phase 1.7 will use the user's real Chrome session via the chrome-MCP / agent-browser path, capture HARs of: order history listing, one order detail, ship-track, transactions, gift cards. This produces the synthetic spec we feed to `printing-press generate`.
- Generated CLI ships **Surf-backed** browser-fingerprint HTTP + Chrome cookie import (`auth login --chrome`). No persistent browser at runtime.
