# Shopper CLI — Absorb Manifest

## Landscape
No existing Shopper CLI, MCP server, or SDK wrapper exists (greenfield — confirmed via GitHub/npm/PyPI search). "Absorb" therefore = expose every discovered API surface as agent-native commands and beat the web UI with offline SQLite, FTS search, --json/--select, --dry-run, and typed exit codes. Transcendence is where differentiation lives.

## Absorbed (discovered endpoints → agent-native commands)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search catalog by text | siteapi POST /catalog/search | (generated endpoint) catalog search | Offline FTS over synced SKUs, regex, --json/--select |
| 2 | Search suggestions | siteapi GET /catalog/search/suggest | (generated endpoint) catalog suggest | Scriptable autocomplete |
| 3 | Search result count | siteapi POST /catalog/search/count | (generated endpoint) catalog count | Quick availability check |
| 4 | Search filters/facets | siteapi POST /catalog/search/filters | (generated endpoint) catalog filters | Facet discovery for scripted filtering |
| 5 | List departments | siteapi GET /catalog/departments | (generated endpoint) catalog departments | Offline cached category tree |
| 6 | New products feed | siteapi GET /catalog/products/news | (generated endpoint) catalog news | Diff against last sync |
| 7 | Promo banners | siteapi GET /catalog/banners | (generated endpoint) catalog banners | JSON output for scripting |
| 8 | Cart summary | siteapi GET /cart/summary | (generated endpoint) cart summary | Local snapshot for diffing |
| 9 | Delivery summary | siteapi GET /delivery/summary | (generated endpoint) delivery summary | Surfaced with charge-date math |
| 10 | Delivery calendar | siteapi GET /delivery/v2/calendar | (generated endpoint) delivery calendar | Joined with charge dates |
| 11 | List addresses | siteapi GET /address/ | (generated endpoint) address list | --json output |
| 12 | List/select stores | siteapi GET /features/stores, POST /features/stores/select | (generated endpoint) stores list / stores select | Persist store context (x-store-id) in config |
| 13 | Feature toggles | siteapi GET /features/toggle | (generated endpoint) features toggle | Inspect account feature flags |
| 14 | Session/auth validation | siteapi GET /auth/session, /auth/validation/social | (behavior in shopper-pp-cli doctor) | doctor verifies token validity |

## Table stakes (grocery-app awareness — built via above + offline store)
- Catalog search + category filters (rows 1-5).
- Cart visibility (row 8) and delivery scheduling visibility (rows 9-10).
- Account/address context (rows 11-12).

## Auth & transport
- Bearer JWT (`Authorization: Bearer <SHOPPER_TOKEN>`), token sourced from logged-in browser session. Required headers: `app-os-x-version: web:1002`, `x-store-id`, `x-cluster-id` (store context from stores/select; default 1/1). Standard HTTP (probe-confirmed). `doctor` validates token via GET /auth/session.

## Transcendence (see research.json novel_features; scored by subagent)
(Populated by Step 1.5c.5 novel-features subagent.)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Charge & Edit Calendar | charge-calendar | hand-code | Joins snapshotted delivery calendar + plan cadence; derives charge=-7d, lock=-5d(-3d Fresh) across cycles | none |
| 2 | Basket Diff | basket diff | hand-code | Needs two basket states over time; API only returns current /cart/summary | none |
| 3 | Price Watch | price-watch | hand-code | Per-SKU price baseline from local snapshots; API returns only today's price | none |
| 4 | Restock Predictor | restock predict | hand-code | Per-SKU purchase cadence time series from order-history snapshots | none |
| 5 | Catalog Drift Detector | catalog drift | hand-code | Compares successive catalog snapshots for shrinkflation/discontinuation; normalizes unit price | none |
| 6 | Cashback Threshold Optimizer | cashback optimize | hand-code | Knapsack over cart + cashback rule + predicted demand | none |

**Load-bearing primitive:** all 6 depend on periodic local SQLite snapshots of cart, catalog prices, order history, and delivery calendar (the moat). Offline FTS catalog search is infrastructure that powers price-watch/catalog-drift.
