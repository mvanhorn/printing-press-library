# amazon-orders Browser-Sniff Report

## Goal

Walk a logged-in Amazon buyer's order history and pull delivery updates, status, pricing, dates, totals, and tracking — across orders, items, shipments, and transactions. No public buyer API exists, so the CLI must parse Amazon's authenticated HTML pages.

## Capture method

Drove the user's existing logged-in Chrome via `mcp__claude-in-chrome__*` tools. No new login required; the user was already signed in. Capture artifacts: this report + a traffic-analysis JSON. No HAR was archived (chrome-MCP redacted query strings and headers; the structure was captured via DOM/JS inspection instead).

## Reachability

`probe-reachability` results:

| URL | Mode | Status | Notes |
|---|---|---|---|
| `https://www.amazon.com` | `standard_http` | 200 | Plain HTTP works |
| `https://www.amazon.com/gp/your-account/order-history` | `standard_http` | 401 | "Not authenticated" — expected. Cookie auth flips this to 200. |

**Verdict:** standard HTTP transport with cookie auth. **No browser-fingerprint Surf, no clearance cookies, no resident browser at runtime.** The printed CLI ships `auth login --chrome` to import cookies once, then makes ordinary HTTPS GETs.

## Endpoints discovered

| Endpoint | Method | Purpose | Params | Response |
|---|---|---|---|---|
| `/your-orders/orders` | GET | Paginated order list (HTML) | `timeFilter` (e.g. `year-2026`, `last30days`, `months-3`), `startIndex` (0, 10, 20, …), `ref_` | HTML containing `.order-card.js-order-card` divs, ~10 per page |
| `/gp/your-account/order-history` | GET | Alias / entry into order list | (no required params) | HTML, same shape as `/your-orders/orders` |
| `/your-orders/order-details` | GET | Full order detail (items, shipping, payment, totals) | `orderID` (e.g. `XXX-XXXXXXX-XXXXXXX`), `ref` | HTML with item rows, ASIN links, "Arriving <date>" header, money breakdown |
| `/gp/your-account/ship-track` | GET | Per-shipment tracking | `orderId`, `shipmentId`, `itemId`, `packageIndex` | HTML with carrier name, tracking number, status, ETA |
| `/gp/css/summary/print.html` | GET | Printable invoice | `orderID`, `ref` | HTML invoice |
| `/cpe/yourpayments/transactions` | GET | Transactions list | (no required params; show-more is Ajax) | HTML grouped by date; each row has payment method, last-4, amount, Order # |
| `/gc/balance` | GET | Gift card balance + activity | (no required params) | HTML balance + transaction list |

## Authentication

Cookie-based. The full set on the captured session:

```
session-id, session-token, session-id-time, ubid-main, lc-main,
x-main, i18n-prefs, csm-hit, csd-key, rxc, regStatus,
session-id-apay, aws-target-data, aws-target-visitor-id, aws-userInfo,
AMCV_*, kndctr_*
```

Minimum subset needed for authenticated requests (empirically): `session-id`, `session-token`, `ubid-main`, `x-main`, `lc-main`, `i18n-prefs`. The CLI's auth model: import these from Chrome's cookie jar via `auth login --chrome` and persist them in the local config.

No CSRF/anti-CSRFtoken on the GET surfaces above. CSRF would only matter for state-changing endpoints (cancel, return, gift-card redeem) which are out of scope for v1.

## Order-card HTML shape (listing page)

Selector: `.order-card.js-order-card` (each order is one card).

Inner text shape (whitespace-normalized):
```
ORDER PLACED
<Month Day, Year>
TOTAL
$<Amount>
SHIP TO
<Recipient Name>
ORDER #
<XXX-XXXXXXX-XXXXXXX>
View order details  View invoice
<one or more product titles>
<status text — e.g. "Delivered May 5", "Arriving May 20", "Out for delivery">
```

Selectors for parsing:
- Order placed date: `[class*="order-info"] .a-fixed-right-grid-col:has(.label:contains("ORDER PLACED")) .value`
- Total: similar pattern with `TOTAL` label
- Ship-to recipient name: `TOTAL` label
- Order ID: extracted from "ORDER # XXX-..." line via regex `\d{3}-\d{7}-\d{7}`
- Detail link: `a[href*="order-details"]` → params `orderID`, `ref`
- Track link: `a[href*="ship-track"]` → params `orderId`, `shipmentId`, `itemId`, `packageIndex`
- Product links: `a[href*="/dp/"]` → ASIN extraction via regex `/dp/([A-Z0-9]{10})`
- Status: text containing `Delivered`, `Arriving`, `Out for delivery`, `Shipped`, `Preparing`, `Cancelled`, `Returned`, etc.

## Order-detail HTML shape

Title: "Order Details"

Sections found on the live page:
- "Order placed <date> Order # <ID> View invoice"
- "Ship to <recipient> <street> <city, state, ZIP> Country"
- "Payment method <Card name> ending in <last-4>"
- "Arriving <date>" / "Delivered <date>" header per shipment
- Product item rows with quantity, condition, seller
- "Order Summary": item subtotal, shipping & handling, total before tax, sales tax, grand total
- "Track package" buttons on each shipment

## Pagination

`/your-orders/orders` returns 10 cards per page by default with `?timeFilter=...&startIndex=N&ref_=...`. Last page detection: the "Next →" link is missing or has class `.a-disabled`.

Empirically, the `/gp/your-account/order-history` entry (no filter) shows 20 cards in some viewports and 10 in others — varies by A/B test. The CLI should treat 10 as the safe page size and rely on "Next →" presence to drive pagination.

## Transactions page

Server-rendered HTML, list grouped by date headers. Each transaction row contains:
- Payment method + last-4 ("Prime Visa ****1234")
- Signed amount (negative for charges)
- Linked order number (or non-linked description for non-order charges like Prime/Subscribe & Save)

The "View More" / pagination on the transactions page is a stateful Ajax POST that we intentionally do NOT support in v1 — first page only is enough for the headline use case (recent reconciliation). The user can use date filtering on the orders page for older windows.

## Rate limiting

No documented limits. Empirically, ~10 order-detail fetches in fast succession can trigger throttling or a CAPTCHA. The generated CLI MUST:
- Serialize order-detail fetches with at least 1s spacing
- Use an `AdaptiveLimiter` per-host with target ~30 req/min, burst 5
- Surface `*cliutil.RateLimitError` (not silent empty) when 429s exhaust

## What we are NOT building

- Cancel/return/refund endpoints (state-changing, CSRF, out of scope for v1)
- Cart/checkout (different surface entirely)
- Subscribe & Save management UI (separate page tree)
- Wish lists
- Reviews
- Account settings
- The transactions "Show More" Ajax pagination (v1 first-page only)

## Replayability verdict

**PASS.** All discovered endpoints are GET requests against HTML pages, parseable offline, replayable through standard HTTP with cookies. No live page-context execution required. No JavaScript-rendered content (the order data is in the initial HTML, not rendered by client-side JS). The printed CLI will ship as a standard cookie-authenticated HTTP CLI; no resident browser, no JS engine, no Playwright at runtime.
