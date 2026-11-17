# REWE CLI Brief

## API Identity
- Domain: REWE online grocery shop (www.rewe.de/shop; shop.rewe.de redirects there), pickup ("Abholservice") and delivery, Germany.
- Users: households ordering groceries for pickup; agents (WhatsApp shopping assistant OneDone) acting for one logged-in customer.
- Data profile: per-market catalogue (listingId = article + market store id, price in cents), weekly market offers, basket (versioned), pickup timeslots, payment options, favourites, purchased products, recipes (schema.org Recipe) and an ingredient-to-product mapper.
- Homepage: https://www.rewe.de/shop/ (observed 2026-11-17).

## Reachability Risk
- Medium. Cloudflare in front of every host. JSON APIs under /shop/api and /api answered 200 to plain curl with the session cookies (measured 2026-11-17: configuration, timeslots, favourites, myproducts, products). HTML pages (offers, product list) return 403 `cf-mitigated: challenge` to Go stdlib and 200 to the Surf Chrome fingerprint (`probe-reachability` = browser_http).
- Community evidence: korb (app API) reported Cloudflare TLS-fingerprint blocking in April 2026; rewerse (app API, no-login routes) still works Sept 2026. Neither uses the web shop surface we use.

## Auth
- Keycloak realm `sso` at account.rewe.de, web client `ecom`, PKCE, server-side Spring session. Shop session cookies: `rstp`, `mtc`, `cso_jsid` (browser-session lifetime) plus Cloudflare `cf_clearance`, `__cf_bm`, `_cfuvid`, market cookie `wksMarketsCookie`.
- First login needs a human (Turnstile + mandatory 2FA since 2025-08-27). After that `KEYCLOAK_IDENTITY`/`KEYCLOAK_SESSION` (30 days, rolling) and `KEYCLOAK_REMEMBER_ME` (1 year) allow a silent re-login over plain HTTP: GET /mydata/login -> Keycloak "continue with this account" form -> POST login= -> GET callback -> new shop session. Verified with curl 2026-11-17 (isLoggedIn:true, timeslots 200).
- So the CLI stores a cookie jar (imported once from a logged-in browser), renews the shop session itself, and fails with a typed "re-login needed" error when Keycloak cookies expire.

## Top Workflows
1. Weekly pickup order: search products -> add to basket -> pick earliest pickup slot -> pay in market -> place order (explicit confirmation).
2. Offers: what is on offer in my market this week / next week, filtered by category or my usual products.
3. Recipe to basket: recipe -> servings -> REWE ingredient mapper -> recommended products -> add chosen ones.
4. Reorder: "Bereits gekauft"/favourites -> add again.
5. Market and slot planning: find pickup markets by zip, see free slots and fees.

## Table Stakes
- Market search and selection, product search with price per unit, product detail, basket add/set/remove/clear, timeslots list/reserve, payment options, order placement, favourites add/remove, already bought, offers, recipes search/detail, recipe mapping, JSON output for agents, dry-run for every mutation.

## Data Layer
- Primary entities: products (listingId, productId, name, price, grammage, market), offers (nan, title, price, week), basket snapshots, timeslots, recipes, orders placed by the CLI.
- Sync cursor: offers per market and ISO week; products cached per market with fetched_at.
- FTS/search: offers + cached products (title, brand, category) for offline "is X on offer" queries and price history.

## User Vision
- Jim (2026-11-17): order by WhatsApp voice note, agent knows offers, remembers preferences, proactively suggests offers, recipes as a HelloFresh-like basket, always proposes the earliest pickup slot ("frühestens morgen 8 Uhr - passt das?"). The CLI/MCP is the tool layer under that agent and must also work standalone.

## Product Thesis
- Name: rewe-pp-cli ("REWE Abholservice from the terminal and from agents").
- Why it should exist: no official API; existing community clients target the mobile app API, need a client certificate extracted from the APK and are partly blocked. This CLI uses the web shop surface with a self-renewing session, so it needs no secret certificate and can ship publicly.

## Build Priorities
1. Session: `auth import` (cookie jar from browser), silent renew, `auth status`, typed re-login error.
2. Market, products, offers (HTML via Surf), basket, timeslots, checkout + order with confirm gate.
3. Recipes + ingredient mapping to basket; favourites; already bought.
4. Local store: offers per week, price history, "deals on my usual products", basket total/fee preview.
