# EDEKA CLI Brief

## API Identity
- Domain: EDEKA online shops of independent EDEKA retailers on `<market>.edeka.shop` (for example `doerflinger-ulm.edeka.shop`). Each market is its own OXID eShop instance run by maexware ("olivia3" theme, maexware-kundencloud image CDN). The portal `edeka.shop` only hosts the market finder.
- Users: households ordering groceries for pickup (Selbstabholung) or delivery (Lieferservice) from their local EDEKA market; agents (OneDone WhatsApp shopping assistant, ChatGPT/Claude via MCP) acting for one logged-in customer.
- Data profile: per-market catalogue (OXID article id such as `5915407005MXST`, title, price, unit price, image, category path), weekly offers (category `/Angebote/`, about 1,200 articles per market), basket (saved per user on the server), pickup and delivery slots (embedded JSON in the calendar widget), checkout (user -> payment -> order), order history.
- Homepage: https://edeka.shop/ (market finder) and https://<market>.edeka.shop/ (observed 2026-11-17).

## Reachability Risk
- Low. Apache, no Cloudflare or bot protection; plain curl and Go net/http get 200 on every page and JSON endpoint (measured 2026-11-17). HTML pages are large (2-3 MB each, mostly the category navigation).
- Mutations need the OXID CSRF token `stoken` from any HTML page of the same session; without it the shop answers with an HTML page instead of JSON.
- Login has a captcha after repeated failed attempts (observed while creating the account); the CLI never logs in with a password.

## Auth
- OXID session cookies per market host: `sid` (session), `sid_key`, `language`; remember-me cookie `oxid_1` (one year) plus `oxid_1_autologin`. Measured 2026-11-17: a request that carries only `oxid_1` gets a new logged-in `sid` from the server (silent renewal, no password). The session `sid` from the browser alone does not replay from another client (bound to the browser), so the CLI always sends `oxid_1` and keeps the `sid` the server hands it.
- Slot choice lives in cookies too (`mxpickupdate`, `mxreservationid`, set by `mx_pickup_controller/setCookie`), so the CLI must persist response cookies (recording jar, locked atomic save).
- When `oxid_1` is gone or rejected the account page shows the login form: typed "re-login needed" error, exit code 4.

## Top Workflows
1. Pickup order: search products -> add to basket (or add by free text, best match) -> earliest pickup slot -> pay at pickup (cash or card on handover) -> place order only after explicit confirmation.
2. Offers: what is on offer in my market this week, by category or search term.
3. Market finder: markets by postal code or town, with pickup/delivery flags and delivery postal codes.
4. Basket hygiene: show, set quantity, remove, clear.
5. Order history: list and show past orders.

## Table Stakes
- Market search, product search with unit price, product detail, categories, offers, basket add/set/remove/clear, slot list and earliest slot, slot reserve, payment options, order preview and place, order history, JSON output for agents, dry-run for every mutation.

## Data Layer
- Primary entities: products (article id, title, price cents, unit price, market), offers (article id, market, seen date), basket snapshots, slots, orders placed by the CLI.
- Sync cursor: offers per market and day; prices per article and day.
- FTS/search: cached offers and products (title) for offline "is X on offer" and price history.

## User Vision
- Same as the REWE tools: order by WhatsApp voice note, the agent knows the offers, proposes the earliest pickup slot, and never orders without the customer's yes. The CLI/MCP is the tool layer and must work standalone. Public layer (markets, offers, search) must work for every edeka.shop market without login.

## Product Thesis
- Name: edeka-pp-cli ("EDEKA online shop from the terminal and from agents").
- Why it should exist: EDEKA has no public shop API and the independent markets each run their own shop; there is no community client for the edeka.shop (maexware) shops. This CLI replays the web shop with a self-renewing cookie session and works for any market by subdomain.

## Build Priorities
1. markets search/select, offers, product search/show, categories (public, any market).
2. basket show/add/add-by-text/set/remove/clear.
3. slots list/earliest/reserve (pickup and delivery).
4. order preview/place (--confirm), order list/show.
5. auth import (CDP loopback or file), auth session, silent renewal through `oxid_1`.
6. Price history in local SQLite (cheap, from search and offers).
