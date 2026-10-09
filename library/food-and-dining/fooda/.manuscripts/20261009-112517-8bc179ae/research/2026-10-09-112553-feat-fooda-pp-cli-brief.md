# Fooda CLI Brief

## API Identity
- Domain: Workplace lunch marketplace (fooda.com marketing site on Webflow; ordering app at app.fooda.com). Office "pop-up" restaurant events by building, plus ordering/catering.
- Users: (1) A subsidized office employee (e.g. NYC Magnite, $20/day subsidy valid Tue/Wed/Thu on Popup+Delivery events) who picks lunch each morning before ordering windows close and wants to know what's on and what's left of their subsidy. (2) The same person auditing months of order history/spend and avoiding vendor fatigue (repeat ordering of the same dish). (3) A team lead coordinating who orders what on shared delivery days (catering/delivery events).
- Evidence: captured account has subsidy 'NYC Magnite Daily' (100% up to $20/day), events across Popup/Cafe/Delivery/Catering, past-order list with vendor, total, delivery time, item.
- Data profile: Buildings, daily events (restaurant pop-ups), menus/items, orders, order history, allowances/credits. No public API or spec.

## Reachability Risk
- High: app.fooda.com/login returns 403 with `cf-mitigated: challenge` (Cloudflare). probe-reachability -> `browser_clearance_http` (confidence 0.6).
- Wrappers: only an abandoned 2016 npm CLI `fooda` (v0.0.8, menu scraper). No SDK, no OpenAPI.

## Top Workflows
1. What's for lunch today/this week at my building (events, restaurants, menus).
2. Order history and spend (what I've eaten, how much, favorite vendors).
3. Place / cancel an order (dry-run by default, --confirm to apply).
4. Week-ahead digest across upcoming events.

## Table Stakes
- Menus per event, building select, order history. (2016 npm tool only scraped menus.)

## Data Layer
- Entities: buildings, events, restaurants, menu items, orders. FTS over items/restaurants.

## User Vision
- "Should be like forkable feature wise": history/spend/venue-rotation/preference analytics, week-ahead, mutations dry-run by default with --confirm.

## Product Thesis
- Name: fooda-pp-cli
- Why: Only terminal/agent surface for Fooda, with longitudinal history, venue rotation and spend analytics the app lacks.

## Build Priorities
1. Auth via logged-in Chrome session cookies (clearance + session).
2. Read: buildings, events, menus, orders.
3. Analytics: order-history, spend, venue-rotation, week-ahead.
4. Mutations: order place/cancel (dry-run default).

## Reachability Gate
- Decision: PASS
- Evidence: authenticated GET of /settings/orders and POST /graphql (getUserSubsidies) returned 200 with real data via stdlib curl and Chrome UA, using session cookie + page-meta tokens. Only /login is Cloudflare-challenged (not used by CLI).
