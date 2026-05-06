# Domino's Pizza CLI Brief

## API Identity
- Domain: Food ordering (pizza delivery/carryout)
- Data profile: Stores, menus (products/variants/toppings), orders, coupons, customers, order tracking status
- Auth: OAuth 2.0 password grant via `authproxy.dominos.com`, or unauthenticated for read-only operations

## Users (concrete personas)
- **The Friday-night reorderer** — same household, same order most weeks ("large pep with extra cheese, garlic knots, sprite, deliver to 421 N 63rd St"). Today opens the dominos.com tab, fishes for the last order in account history, manually re-clicks every item. Frustration: "I order the same thing every week, why is this six clicks instead of one command?"
- **The deal hunter** — checks dominos for active coupons before every order, mentally cross-references against the cart. Today loads the deals page, scrolls through 12+ promos, eyeballs which apply to the planned cart, sometimes misses a stackable rewards deal. Frustration: "There's no view that says 'best price for this exact cart'."
- **The party planner / office orderer** — needs to feed 8–30 people; juggles store wait times, group preferences, splitting orders across stores when one is slammed. Today calls the store, uses a spreadsheet to track who wants what, manually composes the order in the web UI. Frustration: "Every order goes through one store; no tool comparing wait times or splitting orders across nearby stores."
- **The agent / automation power user** — Claude / a homelab script wants to place an order programmatically as a side-effect of some other workflow ("after standup, order team lunch"). Today writes brittle puppeteer scripts against the website. Frustration: "There's no agent-shaped tool that 'orders pizza for delivery to address X with budget Y' and exits cleanly."
- **The tracker-watcher** — placed an order, refreshes the tracker page every 30 seconds during the bake/quality-check stage. Today: 5 page refreshes per minute. Frustration: "Why isn't there a `track --watch` that streams status updates to my terminal?"

## Reachability Risk
- **Medium** — The UK Python wrapper (tomasbasham/dominos) has a 403 issue, but the US-focused node wrapper (RIAEvangelist) and Go wrapper (harrybrwn/dawg) remain functional. Rate limiting exists but is manageable. Geographic blocking for non-US requests reported.

## API Endpoints (Reverse-Engineered, Stable)

### Base: `https://order.dominos.com`
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/power/store-locator?s={line1}&c={line2}&type={type}` | Find nearby stores |
| GET | `/power/store/{storeID}/profile` | Store details, hours, capabilities |
| GET | `/power/store/{storeID}/menu?lang={lang}&structured=true` | Full menu with categories |
| POST | `/power/validate-order` | Validate order contents |
| POST | `/power/price-order` | Price an order |
| POST | `/power/place-order` | Submit order |
| POST | `/power/login` | Account login |

### Auth: `https://authproxy.dominos.com`
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/auth-proxy-service/login` | OAuth token (password grant) |

### Tracking: `https://tracker.dominos.com` (or `trkweb.dominos.com`)
| Method | Path | Purpose |
|--------|------|---------|
| GET | `/orderstorage/GetTrackerData?Phone={phone}` | Track order by phone |

## Top Workflows
1. **Order a pizza for delivery** — find store, browse menu, build order, validate, price, pay, track
2. **Find nearest store & check hours** — locate by address/zip, check if open, delivery vs carryout availability
3. **Browse menu & search items** — explore categories, search by name, view toppings/options
4. **Track an active order** — monitor prep/bake/box/delivery stages in real-time
5. **Reorder a favorite** — save order templates locally, replay with one command

## Table Stakes (from competitors)
- Store locator with distance (apizza, node-dominos, mcpizza)
- Full menu browsing by category (apizza `menu`, mcpizza `get_menu`)
- Item customization with toppings (apizza topping syntax `P:full:2`)
- Cart management with named orders (apizza `cart new/add/remove`)
- Order validation and pricing (all wrappers)
- Order placement with payment (apizza, pizzamcp)
- Order tracking by phone (node-dominos)
- Config management for saved addresses/payment (apizza `config`)

## Data Layer
- Primary entities: Stores, MenuItems (Products + Variants), Toppings, Orders, OrderItems, Coupons
- Sync cursor: Menu data per store (changes infrequently), order history per user
- FTS/search: Menu item search across name, description, category. Topping search.

## Product Thesis
- Name: **dominos-pp-cli**
- Why it should exist: Every existing tool is either abandoned (apizza last commit years ago), language-specific (node/python only), or limited to MCP (mcpizza). No maintained CLI offers offline menu search, order templates, real-time tracking with polling, or agent-native output. The compound features (price comparison across stores, order history analytics, smart reorder from templates) are impossible without a local data layer.

## Build Priorities
1. Store locator + menu browsing with offline FTS search
2. Full order workflow: build, validate, price, place
3. Order tracking with real-time polling
4. Saved order templates and reorder
5. Coupon discovery and application
6. Account auth with order history sync

## User Vision (reprint — 2026-05-05)
The user requested this reprint specifically to take advantage of ~169 commits and PR #639's auth env-var widening since the prior generation. Goals:
- Better MCP surface (Cloudflare-pattern enrichment if tool count justifies it; the prior CLI had no `mcp:` block)
- Better auth modes (PR #639 introduces typed `AuthEnvVar` with kind/required/sensitive metadata; bearer detection is now richer per PR #634)
- Better scoring rubrics (post-3.0 scoring saw multiple fixes for MCP token counting, freshness coupling, opt-out dimensions)
- Anything else the current machine does better than 2.3.8

## Revalidation Findings (2026-05-05, against printing-press 3.9.1 + branch + PR #639)

**Reachability** — `printing-press probe-reachability https://order.dominos.com/` returns `mode: standard_http, confidence: 0.95`. Direct `curl /power/store-locator` returns 200 + valid JSON with no auth. Prior brief's "reachability risk: medium" was overstated — it referenced a UK Python wrapper's geographic block, not US programmatic access. Downgrade to **Low**.

**Auth model under PR #639** — Prior spec encodes `auth.type: api_key` with `format: "Bearer {token}"` and env var `DOMINOS_TOKEN`. Under the new typed `AuthEnvVar` model + PR #634's "infer bearer auth from inline params", the canonical type is `bearer_token`, not `api_key`-with-bearer-format. Migration needed in Phase 2 pre-generation enrichment. The fact that many read endpoints (`store-locator`, `store profile`, `menu`, `track_order`) work without any auth means the spec also under-tags `no_auth: true` — those should be flagged so MCP surface and SKILL prose are honest about which tools work credential-free.

**MCP surface under current scoring** — Prior spec has no `mcp:` block. Surface count: 19 typed endpoints + ~13 framework cobratree tools + 10 planned novel features = ~42 tools. This is in the 30–50 band where `mcp.transport: [stdio, http]` is recommended for remote reach, and `mcp.intents` is a strong fit because dominos has clear multi-step workflows (find store → menu → build cart → validate → price → place → track). Add `mcp:` enrichment in Phase 2.

**Scoring drift** — Post-3.0 scoring fixes (#486, #488, #489, #519, #552) reclassify what counts. The prior CLI's scorecard was generated under v2.3.8 rubrics; the new scorer will judge it differently. Re-scoring is unavoidable — current rubric is the only meaningful target.

**PR #639 regression watchlist (this branch)** — Generation will exercise `auth.go.tmpl`, `auth_browser.go.tmpl`, `config.go.tmpl`, `doctor.go.tmpl`, `mcp_tools.go.tmpl`, `agent_context.go.tmpl`, `helpers.go.tmpl`, `readme.md.tmpl`, `skill.md.tmpl`, `climanifest.go`, `mcpb_manifest.go`. Watch for: env-var rendering shape (Name/Kind/Required/Sensitive/Description), doctor's auth check correctness, MCP `agent-context auth` output, README auth narrative, manifest `env_vars` field shape, scorer's new auth dimensions.

## Auth Decision (user-confirmed 2026-05-05)

**Domino's has no formal API key.** The prior spec's `auth.type: api_key` + `DOMINOS_TOKEN` was a half-truth — the bearer token is real, but it is *harvested from an OAuth password-grant login flow* against `authproxy.dominos.com`, not something a developer can obtain from a portal. Selected model:

- `auth.type: bearer_token`
- Three env vars under PR #639's typed `AuthEnvVar` model (`x-auth-vars` extension):
  - `DOMINOS_USERNAME` — `kind: auth_flow_input`, `required: false`, `sensitive: false`
  - `DOMINOS_PASSWORD` — `kind: auth_flow_input`, `required: false`, `sensitive: true`
  - `DOMINOS_TOKEN` — `kind: harvested`, `required: false`, `sensitive: true`
- Read-only endpoints (store-locator, store profile, menu, tracking-by-phone) tagged `no_auth: true` — they work without any credential
- Logged-in endpoints (account, loyalty, order history, customer-account checkout) read the harvested token

This stresses all three `kind` values + the harvested-from-login path that PR #639 widened. Exercise of `auth login --chrome`-style flow and `doctor` reachability check is part of the regression watchlist.
