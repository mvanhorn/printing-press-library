# Shopper CLI Brief — Reprint 2026-08-03

## API Identity
- Domain: shopper.com.br — Brazilian online-only supermarket (SP-state, 130+ municipalities)
- Users: Households managing recurring monthly/biweekly/weekly grocery baskets + one-off and ultra-fast lanes
- Data profile: 12,000+ SKUs; recurring basket template; 6 active storefronts; delivery cadence + slots; charge cycle (charged 7d before delivery for subscription stores)

## Reachability Risk
- Low (confirmed). siteapi.shopper.com.br responds to Bearer-token REST calls. Web frontend (programada.shopper.com.br etc.) redirects unauthenticated users to landing page.
- Confirmed: 152 siteapi HAR entries from prior sniff, all responding 200/204.

## Storefront Map (confirmed live via GET /features/stores)
| CLI name      | store_id | cluster_id | with_recurrence | ultra_fast | Description        |
|---------------|----------|------------|-----------------|------------|--------------------|
| programada    | 1        | 1          | true            | false      | Compra Programada (monthly) |
| fresh         | 2        | 1          | true            | false      | Programada Fresh (weekly/biweekly) |
| unica         | 3        | 3          | false           | false      | Compra Única (one-time) — **prior CLI had wrong cluster_id=1** |
| pet           | 5        | 3          | true            | false      | Pet.Shopper |
| now           | 6        | 11         | false           | true       | Shopper Now (ultra-fast ~20min) — **NEW** |
| now-bebidas   | 8        | 11         | false           | true       | Now Bebidas (beverages, ultra-fast) — **NEW** |

## API Surface (two transports)

### Transport A: siteapi.shopper.com.br (REST JSON, Bearer token)
All confirmed from live HAR + patches:
- GET /address/ — delivery addresses with per-address available_stores
- POST /cart/add — {id: int, quantity: int, engine?: string}
- POST /cart/remove — {id: int, quantity: int}
- GET /cart/summary — cart total, items count, cashback, ranges
- GET /catalog/banners, /catalog/banners/{id}/view
- GET /catalog/departments
- GET /catalog/home (partial capture, status unreliable)
- GET /catalog/products/news
- POST /catalog/search — {brands:[], metadata:[], types:[], page?, query?}
- POST /catalog/search/count, /catalog/search/filters
- GET /catalog/search/suggest
- GET /delivery/summary — {deliveryDate, message, expressDelivery*}
- GET /delivery/v2/calendar — date-picker config (allowed range + disabled days)
- GET /features/stores — full 6-store list with params (minimal_days_credit_card, minimal_days_bankslip, allow_pix)
- POST /features/stores/select — store selection (no-op for reads, confirmed in patch)
- POST /features/timer/start, GET /features/timer/tick
- GET /features/toggle, POST /features/toggle/view
- GET /orders/orders?size=N — purchase history (patched, not in original sniff)
- GET /auth/validation/social

### Transport B: <storefront>.shopper.com.br (Django legacy, session-cookie, form-encoded)
Confirmed from static JS bundle analysis (not siteapi, browser-required):
- GET /shop/checkout — checkout page
- POST /shop/carrinho/add/, /shop/carrinho/add-indication/
- POST /shop/carrinho/diminuir-quantidade/, /shop/carrinho/remove/
- POST /shop/carrinho/pause/, /shop/carrinho/play/ — subscription pause/resume
- GET /shop/minha-conta/verifica-max-cartoes/ — check card slot availability
- POST /shop/minha-conta/adicionar-cartao — form-encoded, computed expiry (no raw card in CLI)
- POST /shop/minha-conta/excluir-cartao, /tornar-principal
- POST /shop/minha-conta/transacionar — retry failed transaction
- GET /shop/minha-conta/consultar-entrega — delivery info lookup
- POST /shop/minha-conta/alterar-data — reschedule delivery (fields: data_entrega, botao)
- POST /shop/minha-conta/pular-entrega/ — skip one delivery
- POST /shop/minha-conta/suspender-entrega — suspend subscription
- GET /shop/minha-conta/calendario-entregas — delivery calendar HTML
- GET /shop/minha-conta/recuperar-boleto/ — retrieve boleto URL
- POST /shop/minha-conta/substituir-compra — replace order (processed-order ID)

CLI design decision: Transport B commands are browser-required (session cookies, CSRF, computed form fields). CLI exposes: (1) deep-link openers (open system browser to the correct storefront page), (2) read-only siteapi equivalents where available, (3) explicit "browser-required" diagnostics for mutations. No raw card numbers. No checkout confirmation via CLI.

## Prior Patch Records (must preserve)
1. **store-scoping**: x-store-id/x-cluster-id headers select storefront. Fixed unica cluster 1→3. Adding now (6/11) and now-bebidas (8/11). Cache key must be store-aware.
2. **orders-history**: GET /orders/orders — all 4→6 storefronts queried for spend.
3. **shopper-catalog-search-required-flags**: brands/metadata/types are optional, only query is required.
4. **shopper-charge-calendar-realshape**: /delivery/summary for deliveryDate; /delivery/v2/calendar for date-picker config; charge=−7d, lock=−5d.
5. **catalog-go-1265-vuln-floor**: go.mod must declare go 1.26.5+.

## Top Workflows
1. **Browse + add to basket** (catalog search, cart add, cart summary) — highest frequency for recurring stores
2. **Manage subscription basket** (pre-cycle edit window, basket diff, deadlines)
3. **Checkout preview** (before confirming: cart summary + delivery summary + payment info + charge date)
4. **Monitor charge schedule** (charge-calendar: next charge date, lock date, delivery date)
5. **Cross-store spend analysis** (orders spend across all 6 stores)
6. **Now/Now-Bebidas quick order** (ultra-fast: browse, add, open checkout in browser)

## Data Layer
- Primary entities: products (catalog), cart items, orders, delivery schedule, addresses
- Sync cursor: catalog by category, orders by date, delivery dates
- FTS: product name/brand/category offline search (12k+ SKUs)
- Local store: basket snapshots, price history, spend history

## Product Thesis
- Name: shopper-pp-cli
- Why: First programmatic interface to Shopper. Covers all 6 storefronts with correct store/cluster scoping, full read surface via siteapi REST, safe browser-deep-link mutations, and local analytics (charge calendar, basket diff, price watch, spend analysis) the web app doesn't surface.

## User Vision (from reprint request)
- Complete customer journey for every available storefront
- Dynamic storefront discovery (not hardcoded 4-store list)
- Cart reads AND mutations; addresses; delivery slots/selection where applicable
- Checkout preview, payment-method listing, order submission (open browser)
- Order status/history; cancellation if supported; receipts/charge schedules
- Safety: preview/dry-run for consequential commands; explicit confirmation barrier for mutations
- Never print/log/cache/commit credentials or sensitive payment data

## Build Priorities
1. Fix store map: add now+now-bebidas, correct unica cluster, dynamic discovery
2. Cart set-quantity command (not just increment/decrement)
3. Checkout preview command (siteapi reads aggregated)
4. Delivery management: deep-link openers for reschedule/skip/suspend + boleto
5. Subscription: pause/resume deep-link openers
6. Payment: card count status (read-only siteapi) + browser-required guidance for card add/remove
7. MCP enrichment: intents for core workflows, transport [stdio, http], cache-ID fix for singleton responses
8. All 6 stores in spend, history, and store-scoped reads
