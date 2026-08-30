# Best Food Trucks (BFT) CLI Brief

## API Identity
- Domain: Food-truck booking, location-scheduling, and online-ordering SaaS platform. Self-description: "Best Food Trucks (BFT) is the nation's largest food truck booking & ordering platform. From location management & food truck catering to our exclusive order ahead technology to setting up food trucks at your office or event, Best Food Trucks will handle all the logistics."
- Users: three personas —
  1. **Consumers / office workers** who want to know what truck is at their office campus ("lot") today, browse trucks near them, check menus/ratings, and order ahead.
  2. **Facility/property managers** who host a rotating weekly food-truck schedule at a "lot" (example target: Playa District, an office campus in Playa Vista/Los Angeles).
  3. **Food-truck operators** who get booked into dated "shifts" at lots (operator-side portal not explored this run — likely requires separate operator credentials).
- Data profile: Nationwide directory. Confirmed cities in the "Find trucks by state" directory: Los Angeles, San Francisco, Chicago, New York, Oklahoma City, Kansas City, Jersey City, Panama City, Traverse City (and more, paginated). URL pattern `/food-trucks/<city-slug>` for city listings and `/lots/<lot-slug>` for a specific hosted location's schedule.

## Reachability Risk
- Medium, but resolved by transport choice — [evidence: `probe-reachability` against `https://www.bestfoodtrucks.com/lots/playa-district` returned `mode: browser_http`, confidence 0.85]. Plain stdlib HTTP gets HTTP 429 with a Vercel "Security Checkpoint" bot-mitigation page (`x-vercel-mitigated: challenge`, `x-vercel-challenge-token` present). Surf with a Chrome TLS fingerprint clears the same URL cleanly (200 OK, real content, 97ms). **Runtime decision is settled: ship Surf/Chrome-TLS transport, no clearance-cookie or `auth login --chrome` needed for the public marketing/lots surface.**
- No official developer API exists. Confirmed via direct probing: `/api`, `/developers`, `/developer`, `/api-docs`, `/docs/api`, `/partners`, `/for-developers`, `/integrations`, `/.well-known/openapi.json`, `/openapi.json`, `/swagger.json` all 404 or don't resolve on `www.bestfoodtrucks.com`. `developers.bestfoodtrucks.com` / `developer.bestfoodtrucks.com` don't resolve (DNS/connection failure).
- Zero community ecosystem — [evidence: GitHub repo search for "bestfoodtrucks" returned `total_count: 0`; npm registry search for "bestfoodtrucks" returned `total: 0`]. No SDKs, wrappers, MCP servers, or competing CLIs exist anywhere. This is a from-scratch build with no absorb-manifest donors from the community — the absorb manifest for this CLI is effectively "match BFT's own web UI," not "beat an existing wrapper."
- **Multiple backend surfaces discovered, not yet confirmed which serves JSON:**
  - `www.bestfoodtrucks.com` — Astro SPA on Vercel (marketing pages, `/lots/<slug>`, `/food-trucks-near-me`, city directory). Guarded by Vercel Security Checkpoint; cleared by Surf.
  - `api.bestfoodtrucks.com` — resolves, Cloudflare-fronted (not Vercel), returns `access-control-allow-methods: GET, POST, OPTIONS, PUT, PATCH, DELETE` and `access-control-allow-credentials: true` on a bare 404 at `/`. This CORS shape is a strong signal this is the actual JSON backend the SPA(s) call client-side — undocumented for third parties, discoverable via browser-sniff network capture.
  - A Rails-style server (CSRF `authenticity_token` meta tags, custom 404 HTML) also answers on `www.bestfoodtrucks.com/api` and `api.bestfoodtrucks.com/v1`, with assets served from a THIRD host, `cdn-api.bestfoodtrucks.com`. This may be a legacy/admin surface behind the same edge routing rather than the primary consumer API — needs traffic capture to disambiguate from `api.bestfoodtrucks.com`'s CORS-enabled JSON surface.
  - `cdn.bestfoodtrucks.com` — serves `_next/image` optimized assets, meaning at least one page (the `/shifts/<id>` truck-detail/ordering page) is a **separate Next.js app**, not Astro. Shift pages and lot pages may be genuinely different frontends calling the backend differently. Browser-sniff should capture both a `/lots/<slug>` visit and a `/shifts/<id>` visit.
- No GitHub issues to review (no wrapper repos exist), so no independent "is this API flaky/blocked" signal beyond the bot-protection evidence above.

## Top Workflows
1. **Find food trucks near me / by city** — browse the nationwide directory (`/food-trucks/<city>`), see which trucks operate in a region.
2. **Check a specific lot's schedule** — "what truck is at Playa District today / tomorrow / this week / full schedule". Confirmed UI buckets: Today, Tomorrow, named upcoming dates, Full Schedule. This is the single highest-value workflow — it's the literal reason a URL like `/lots/playa-district` exists and the reason the user pointed the CLI at it.
3. **View a truck's menu, hours, and ratings for a scheduled shift** — confirmed data: time window (`11:00 am - 2:00 pm, <date>`), full itemized menu with prices ($5.50–$16.75 range observed), review count (8 reviews), social links (Instagram).
4. **Reverse lookup: where is Truck X scheduled** — across lots and dates (not yet confirmed as a UI-exposed search, but implied by the shift/truck/lot data model; likely a novel/transcendence command rather than an absorbed one).
5. **Subscribe to a lot's schedule** (`/customers/login?subscribe_to_lot_id=<id>`) — requires a customer account; the retention workflow for office workers who want to be notified.
6. **Order ahead from a scheduled truck** — the site's own headline differentiator ("exclusive order ahead technology"). Requires customer login + likely payment. Read-only discovery (menu, price, hours) is in scope; completing a real paid order is not something this CLI should automate.
7. Hire-a-truck / corporate catering lead-gen (`/hire-food-truck`, `[REDACTED EMAIL]`) — a sales funnel, not a CLI automation target.

## Table Stakes
Since no competing tool or wrapper exists, "table stakes" = matching BFT's own web UI surface, which the CLI must absorb in full:
- Directory browse: trucks by city/state, lots by name/slug.
- Lot schedule view with the same date-bucket shape (today/tomorrow/named-dates/full).
- Truck detail: menu + prices, hours for a given shift, rating/review count, social links.
- Customer auth (login/subscribe) as a distinct, optional command path — not required for the read-heavy majority of workflows.
- FAQ/help content exists at `/faq/orderahead/*` — worth mirroring key facts (e.g., "how does order ahead work") into `doctor`/help text rather than a dedicated command.

## Data Layer
- Primary entities (names provisional pending real endpoint discovery in Phase 1.7):
  - `lots` — id (e.g., `4702` for Playa District), slug, name, address/geo, description, image.
  - `trucks` — id, name (e.g., "The Chick Truck"), rating, review_count, social links, cuisine (unconfirmed).
  - `shifts` — id (e.g., `179609`), lot_id, truck_id, date, start_time, end_time. The join entity that answers "what truck, where, when."
  - `menu_items` — truck_id or shift_id scoped, name, price. Observed price points: 5.50, 13, 14, 14.50, 14.75, 15 (x2), 16.50, 16.75.
  - `reviews` — truck_id, rating, count (text content unconfirmed).
  - `cities` / `states` — directory taxonomy for the `/food-trucks/<city>` browse structure.
- Sync cursor: shifts are inherently date-windowed (past shifts are history, future shifts are the schedule) — sync should default to a rolling window (e.g., -7d/+30d) per lot or per truck rather than full-table sync.
- FTS/search: truck name, lot name/city/state, menu item name — all good SQLite FTS5 candidates once synced locally.

## Product Thesis
- Name: Best Food Trucks (brand display name, exact BFT capitalization/spacing). Binary/slug: `bestfoodtrucks`.
- Why it should exist: BFT has zero API, CLI, or automation tooling today — the only interface is a JS-heavy, bot-protected website. Office managers and food-truck regulars currently re-visit the site by hand to answer "what's at my lot today" or "when does [truck] come back." A CLI/agent-native tool converts that into instant, scriptable, cacheable answers, and unlocks cross-lot/cross-truck analytics (truck frequency tracking, schedule-change diffing, "notify me when my favorite truck returns") that the web UI cannot do because it has no persistence across visits — only this CLI's local SQLite layer can compound that history over time.

## Build Priorities
1. Data layer for lots, trucks, shifts, menu items, reviews — populated via browser-sniffed JSON endpoints once discovered (Phase 1.7), transported through Surf/Chrome-TLS-fingerprint HTTP per the settled `browser_http` runtime decision above.
2. Absorbed commands: lot lookup + schedule view (today/tomorrow/full), truck lookup + menu/hours/ratings, city/state directory browse, customer login/subscribe as an optional auth path.
3. Transcendence candidates (to be scored properly in Phase 1.5c/1.5c.5, not finalized here): reverse truck-schedule lookup across lots, local schedule-change diffing ("did my lot's Tuesday truck change since last sync"), truck-return alerting, menu/price-drift tracking over time, cross-lot "cuisine near me this week" query — all only possible because the local store accumulates history the live site never shows.
