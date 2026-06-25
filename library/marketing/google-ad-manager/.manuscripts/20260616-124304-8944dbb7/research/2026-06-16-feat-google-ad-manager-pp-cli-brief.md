# Google Ad Manager CLI Brief

## API Identity
- **Domain:** Publisher ad serving / SSP — **Google Ad Manager** (GAM / GAM360, formerly DoubleClick for Publishers/DFP). NOT Google Ads (advertiser/PPC side — different product, different API).
- **API:** Ad Manager REST API **v1 (Beta)**, `admanager.googleapis.com`. Source = official Google discovery doc, converted discovery→OpenAPI 3 and normalized (`{+parent}`→`{parent}`). 97 paths, 305 schemas, 183 methods, 49 resource groups.
- **Users:** Publisher ad-ops, programmatic/yield managers, revenue analysts on GAM360 networks.
- **Data profile:** Inventory (ad units [hierarchical], placements, sites, ad spots), targeting (custom targeting keys/values, geo/audience/device/browser/OS dimension tables), orders & line items (read-only in REST), reports (async run→fetch), PMP (private auctions, private auction deals, programmatic buyers), reference (companies, contacts, labels, teams, roles, users).

## Reachability Risk
- **Low.** Discovery doc returns 200. Real resource endpoints require OAuth2 Bearer + a network code; a 401 without credentials is expected, not a block. User has a GAM360 account for live smoke testing (read-only scope is safe).
- Auth wire format: `Authorization: Bearer <access_token>`. Scopes: `…/auth/admanager` (full) + `…/auth/admanager.readonly` (read). Network code required in every resource path (`networks/{networkCode}/…`). Service account reusable from an existing SOAP setup.

## Scope boundary (decided with user)
- **In (REST → buildable):** reporting & revenue analytics, inventory + targeting management (incl. REST writes), orders & line-item visibility (read), PMP visibility.
- **Out (SOAP-only → not press-buildable):** forecasting/availability, line-item/order trafficking *writes*, creative/ad creation + LineItemCreativeAssociation. User acknowledged and chose the REST scope.

## Top Workflows
1. Run a revenue/delivery report and pull rows without the UI wizard (async create→run→poll→`:fetchRows`).
2. Browse/search inventory (ad-unit hierarchy, placements, sites) — fast, offline.
3. Look up custom targeting keys → values instantly.
4. Check order / line-item delivery status (read).
5. Inspect PMP deals / private auctions / programmatic buyers.

## Table Stakes (match the official client libs + UI on the REST surface)
- list/get across every REST resource; pagination (`pageToken`), AIP `filter`, `orderBy`.
- REST-supported writes: ad units, placements, sites, ad spots, custom targeting keys/values, labels, teams, contacts, custom fields (create/patch/batch*).
- Report definition CRUD + `:run` + `:fetchRows`; async operation polling.

## Data Layer
- **Primary entities:** ad_units, placements, sites, ad_spots, custom_targeting_keys, custom_targeting_values, orders, line_items, reports, report_rows, private_auctions, private_auction_deals, programmatic_buyers, companies, contacts, labels, teams.
- **Sync cursor:** `pageToken` per list; `updateTime` for drift snapshots.
- **FTS/search:** names, codes, IDs across inventory + targeting + orders/line-items.

## Competitive Gap (why this should exist)
- **No modern REST-based Google Ad Manager CLI exists**, and **no GAM-specific MCP server exists** — the ecosystem tooling found is almost entirely *Google Ads* (advertiser side): adsctl, google-ads-open-cli, googleads/google-ads-mcp, etc.
- Closest GAM-specific tool: `publica-project/dfp-api` — an old DFP-era SOAP CLI (pre-rename, stale). Plus Google's official SOAP client libraries (googleads-python-lib / Java / PHP / .NET / Ruby) — code, not CLIs.
- The GAM web UI is slow for cross-entity lookup and report iteration.
- **This CLI = first-class REST GAM CLI + local SQLite mirror + offline FTS + async report orchestration + agent-native output.** First in class on the publisher side.

## User Vision
- "Control GAM360 without going into it." Reporting + revenue analytics + inventory/targeting management + orders/line-item visibility + PMP visibility. (Trafficking/forecasting/creatives intentionally deferred — SOAP-only.)

## Product Thesis
- **Name:** Google Ad Manager CLI (`google-ad-manager-pp-cli`)
- **Thesis:** The only REST-native GAM CLI — observe, report, and manage publisher inventory and targeting from the terminal, backed by a local mirror the UI can't match for search and change-history, with agent-native JSON output.

## Build Priorities
1. Auth: Bearer access-token (env) + network-code config; `doctor` (token/scope/network-code/reachability).
2. Generated endpoint surface: all REST resources (list/get + REST writes), with network-code-aware ergonomics so users never hand-build `networks/{code}/…`.
3. Local store + `sync` + offline FTS `search`.
4. Async report orchestration: `report run --wait` → cache rows → offline `report query`.
5. Transcendence: inventory tree, targeting explorer, report presets, drift/`since`, cross-entity search.
