# Pagliacci CLI Brief (v3 — reprint on printing-press v3.7.0)

## Reprint context
- Prior CLI: pagliacci-pizza-pp-cli, generated 2026-04-26 with printing-press v2.3.6.
- Major version delta (2.3.6 → 3.7.0). User explicitly asked to reuse prior research where it doesn't hold us back from v3.7.0 benefits.
- Re-validation outcome: brief, spec, and traffic analysis still hold; the v3.7.0 deltas show up in MCP surface enrichment, scorecard rubrics, and home-user persona pivot — not in API or auth shape.

## API Identity
- Domain: Local pizza delivery & carryout (Seattle area, 8+ stores)
- Users: Seattle-area home users ordering for family or small parties (2–6 people); also agents ordering on behalf of users
- Data profile: Stores (locations, hours, GPS, delivery zones, available slices), Menu (top items, slices, full cache, prices, product detail), Time windows (delivery/pickup days + slots), Orders (build, price, send, history), Account (rewards, stored coupons/credit/gifts, addresses), CMS (site-wide messages)
- API type: **Undocumented** — Angular SPA with two backends:
  - `pag-api.azurewebsites.net/api` — Custom Azure-hosted REST API (ordering, stores, menu)
  - `cdn.contentful.com` — Headless CMS (homepage, blog, loyalty info)
- Bundle source (prior run): `main.BPDM6VAU.js` revealed 33 endpoints

## Auth (composed, cookie-derived)
- Custom `Authorization: PagliacciAuth {customerId}|{authToken}` header
- Angular reads `customerId` and `authToken` from cookies/storage, constructs the header client-side
- Cookie replay alone returns 401 — must capture cookies AND construct the header
- Login flow: POST `/Login` with username/password → response sets cookies
- `auth login --chrome` reads cookies from active Chrome profile and writes config; `auth set-token` accepts a paste of the constructed header

## Reachability Risk
- **None** — verified 2026-05-03: `pag-api.azurewebsites.net/api/Version` and `/api/Store` both 200, JSON, no challenge headers. `traffic-analysis.json` reports `reachability.mode: standard_http` with confidence 0.95. Surf/clearance transport not needed.

## Top Workflows (User-First — home users for family/small party)
1. **Family pizza for delivery** — Find nearest store → browse menu → build a multi-pizza order (often half-and-half for picky kids) → check delivery zone → price → place. The bread-and-butter goal.
2. **Small-party planning (4–8 people)** — Pick combos that feed N people, layer on slices for variety, review delivery time vs. pickup, apply rewards/credit before checkout.
3. **"What's available right now?"** — Today's slices across all 8+ stores (perishable; rotates daily). Unique to Pagliacci, no other tool surfaces this efficiently.
4. **Reward & coupon awareness** — "Do we have a free pizza waiting?" — scan stored coupons, rewards balance, account credit, gift balances before ordering.
5. **Reorder a family favorite** — Pull "the kids' usual" from history, re-price (prices change), re-send.

## Personas (User-First Discovery — home users)
1. **The Family Cook** — Orders 1–2x/month for family of 3–5. Half-and-half pies are normal because of differing tastes. Wants delivery time to match dinner. Probably has rewards built up.
2. **The Small-Party Host** — Hosts 4–8 people occasionally (game night, birthdays). Needs to right-size the order: how many pizzas, what mix of slices, when to schedule for the crowd. Cares about rewards stacking because totals are larger.
3. **The Agent (AI)** — Ordering on behalf of a household. Needs structured output, idempotent operations, dry-run, precise error messages, ability to verify cart before sending. Needs to handle "the family always gets X" patterns.

(Office Coordinator and Late-Night User personas are out of scope for this reprint — the user explicitly anchored on home/family.)

## Table Stakes (must match)
- Menu browsing (categories, prices, descriptions, images)
- Pizza customization (size, toppings, crust, half-and-half)
- Store locator with delivery zone validation
- Address autocomplete + validation
- Cart build → price → submit
- Order history
- Rewards/loyalty balance
- Login/logout

## Data Layer (sync targets for transcendence)
- Primary entities: Stores, MenuItems, MenuSlices (perishable, daily), TimeWindows, Orders, RewardCard, StoredCoupons, StoredCredit, StoredGift, AddressInfo
- Sync cursor: Menu changes weekly (cache OK); slices change daily (re-sync each session); orders append-only on auth.
- FTS/search: Menu item search by name/description, store search by address/zip, order search by item

## Codebase Intelligence (carried over from prior bundle extraction)
- 33 API endpoints discovered from main.BPDM6VAU.js
- Auth: Composed `PagliacciAuth {customerId}|{authToken}` header constructed client-side
- Data model: Stores → MenuTop/MenuCache/MenuSlices per store; Cart in-flight via QuoteStore; Orders assoc to customerId
- Architecture: Angular SPA + Azure REST + Contentful CMS

## User Vision (this reprint)
> "I want unauthenticated AND authenticated scenarios. The primary user is home users ordering for family / small party. Reuse old research where it doesn't hold the v3.7.0 machine back. For novel features, look at the old set and rationalize why we'd do something different."

This run prioritizes:
- **Both auth and unauth surfaces, equal first-class support** — no auth-as-afterthought.
- **Browser session login** — `auth login --chrome` is the headline auth path.
- **Home-user persona** — features rationalized against a family of 3–5 (not an office coordinator or solo late-night user).
- **v3.7.0 MCP enrichment** — opt into Cloudflare-pattern surface (transport [stdio,http], code orchestration, hidden raw endpoint tools) because the tool surface is >50.

## Source Priority
- Single source: pagliacci.com → pag-api.azurewebsites.net. The Multi-Source Priority Gate does not apply.

## v3.7.0 Machine Deltas (re-validation summary)
- **Transport / reachability:** unchanged. `standard_http` mode confirmed 2026-05-03. Surf/clearance not needed.
- **Scoring rubrics:** tier-1 + tier-2 composite still applies; novel features need to score against the home-user personas above.
- **Auth modes:** `composed` is fully supported. No new mode unlocks more endpoints here.
- **MCP surface:** prior CLI shipped 57 endpoint-mirror tools (mcp_ready: cli-only per old manifest). v3.7.0 supports `mcp.transport: [stdio, http]` + `mcp.orchestration: code` + `mcp.endpoint_tools: hidden` for surfaces >50 tools. **Apply the Cloudflare pattern** in spec enrichment before generation.
- **Discovery:** browser-sniff/crowd-sniff workflows unchanged for this API; reusing prior captures.

## Product Thesis
- Name: **pagliacci-pp-cli** (was pagliacci-pizza-pp-cli; reprint shortens to match catalog convention)
- Why it should exist: The first and only CLI/MCP for Pagliacci's ordering API, focused on the family-ordering use case. Ordering Seattle artisan pizza from the terminal was a stunt; ordering it with **today's slices snapshot, half-and-half builder, family-size combo planner, rewards-aware deal stacking, and a local order history** is a useful agent-native tool for households. The Pagliacci API is undocumented, has no SDK, has no MCP — this CLI is the only programmatic interface to it.

## Build Priorities
1. **Foundation** — Sync stores, menu, slices, time windows, customer profile, orders, rewards into local SQLite
2. **Public surface** — Menu browsing, store finder, slice availability, time windows, address validation
3. **Auth surface** — Login (cookie + custom header capture), order history, rewards, stored coupons/credit/gifts, address book
4. **Order workflow** — Cart build, price, send, verify; reorder from history; half-and-half pies
5. **Transcendence (home-user)** — Today's slices snapshot across stores, party-size planner, rewards-aware ordering, store-of-the-night picker, reorder-the-favorite
