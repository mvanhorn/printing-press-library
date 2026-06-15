# Shopper CLI Brief

## API Identity
- Domain: shopper.com.br — Brazilian online-only supermarket (founded 2014, São Paulo). Recurring "smart grocery" model (Amazon Subscribe & Save analog). No physical stores; direct procurement; demand forecasting.
- Users: SP-state households (130+ municipalities) running a recurring monthly/biweekly/weekly grocery basket; one-off and ultra-fast (Shopper Now, ~20min dark-store) lanes added 2024-2025.
- Data profile: 12,000+ SKUs across food/hygiene/cleaning/pet/fresh; recurring basket template; delivery cadence + slots; orders; charge cycle (charged 7 days before delivery); account/addresses/payment.

## Reachability Risk
- Low (inferred). Standard Brazilian e-commerce SPA. Main site 302-redirects to landing.shopper.com.br (separate landing vs logged-in app). No public bot-protection evidence yet; confirm during browser-sniff.
- Probe-safe endpoint used: none yet (website target; endpoints discovered via browser-sniff).

## Top Workflows
1. Build & activate the recurring basket (Compra Programada): browse/search catalog, add items, choose cadence + first date, pay.
2. Pre-cycle basket editing (highest-frequency): add/remove/swap items, change qty, suspend a cycle, shift delivery date — until 5 days before (3 for Fresh).
3. Browse & search catalog (12k+ SKUs by category).
4. Manage delivery schedule: change day/week, skip ("suspender"), cancel (free, no lock-in), pick time slot.
5. Check order / delivery status + upcoming charge dates (charge hits 7 days pre-delivery).
6. (Emerging) Switch delivery mode — Compra Única / Shopper Now fill-in between cycles.

## Table Stakes (competitor awareness)
- Catalog search + category filters (all grocery apps).
- Cart management, order history, delivery-slot availability (iFood Mercado, Daki, Rappi, Carrefour/GPA).
- Subscription mechanics (suspend/reschedule/cancel) — Shopper's differentiator; no direct rapid-delivery analog. Clube Wine/Zé are nearest subscription comparables.

## Data Layer
- Primary entities: products (catalog), basket/cart items, orders, delivery slots, subscription schedule, account/addresses.
- Sync cursor: catalog (by category/updated), orders (by date), upcoming-delivery + charge dates.
- FTS/search: product name/brand/category offline search is high-value (12k SKUs).

## Auth
- Email/password login (confirmed pattern). No public OAuth/SSO.
- Inferred: token (JWT/opaque) in localStorage or HttpOnly cookie, sent as Authorization header; long-lived/refresh given 7-day charge cycle. CONFIRM via browser-sniff.
- AUTH_SESSION_AVAILABLE=true — user will log in before authenticated capture; CLI should support cookie/browser auth import.

## Likely API Surface (inference — confirm via sniff)
- REST + JSON likely (no GraphQL signal). Candidate subdomain api./app.shopper.com.br.
- auth/login(+refresh), users/me, products?q&category, cart + cart/items/{id}, orders(+/{id}), delivery-slots?date, subscriptions/{id} (suspend/reschedule/cancel).

## Community Tooling
- NONE found for shopper.com.br (GitHub/npm/PyPI). Only MayconCoutinho/Api-Shopper = a coding-test mock, not a client. Greenfield — this CLI would be pioneering.

## Product Thesis
- Name: shopper-pp-cli ("shopper")
- Why it should exist: First-ever programmatic interface to Shopper. Turns the recurring-basket + pre-cycle-edit workflow (the product's core, untouched by any rapid-delivery competitor) into scriptable, agent-native commands with an offline SQLite catalog/order store, FTS search, and savings/charge-date awareness no web UI surfaces well.

## Build Priorities
1. Catalog: search/list/get with offline FTS over synced SKUs.
2. Cart/basket: view, add/remove/update items (with --dry-run), per-mode awareness.
3. Orders + delivery: list/get orders, upcoming deliveries, charge-date calendar, delivery-slot availability.
4. Subscription: view schedule, suspend/skip, reschedule, cancel (mutations behind --dry-run/confirm).
5. Transcendence: charge-date calendar, basket-vs-history diff, savings tracker, "what changed since last cycle", deadline alerts (edit-by dates).
