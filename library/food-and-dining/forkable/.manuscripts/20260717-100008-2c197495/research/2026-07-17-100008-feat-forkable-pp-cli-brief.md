# Forkable CLI Brief

## API Identity
- Domain: Corporate office-lunch delivery & catering automation (forkable.com)
- Product surface researched: the "my-account" SPA at `forkable.com/mc/` (Vue.js), calling `forkable.com/api/v2` (same-origin, cookie/session backed).
- Users: office/team admins and members who manage recurring team lunches, individual meals, group orders, and event catering.
- Data profile: users/members, teams, companies, orders, meals, meal preferences (dietary/allergy/likes-dislikes), deliveries, restaurants, billing/payments, group orders, catering events.

## Reachability Risk
- None. `probe-reachability https://forkable.com/mc/` → `mode: standard_http`, confidence 0.95, no bot protection (stdlib + surf-chrome both 200). Plain HTTP works; auth is a browser session, not a WAF/clearance gate.
- Probe-safe endpoint used: GET `https://forkable.com/mc/` (public SPA shell).

## Auth
- Type: browser session (cookie) + `X-CSRF-Token` header. Bundle also references `AUTH_CLIENT_ID`, `SessionId`, `tokenAuthenticate`, `loginWithIntegration` (SSO via Okta/Rippling/Entra).
- Runtime plan: Chrome cookie import (`auth login --chrome`) + session replay. No public API key exists.
- Endpoints are constructed dynamically in JS (string concatenation w/ variables), so **static extraction cannot fully map the surface** — authenticated live capture is required.

## Top Workflows (hypotheses, to confirm at capture)
1. View my upcoming/past orders and the meal chosen for each day.
2. Browse the day's meal options across restaurants and see what's auto-selected.
3. Manage my meal preferences (diet, allergies, likes/dislikes).
4. Admin: view team roster/members, deliveries, and billing/spend.
5. Group orders / catering events overview.

## Table Stakes (read-only)
- List orders (by date range), get order detail.
- List meals / daily options; get meal detail.
- List members/roster; get member.
- Get/show preferences.
- List deliveries; delivery status.
- Billing/invoices/spend summary.
- Restaurants near a location.

## Data Layer
- Primary entities: order, meal, member/user, preference, delivery, restaurant, invoice/payment, team, company, group_order.
- Sync cursor: likely by date window (orders/deliveries are date-keyed) — confirm at capture.
- FTS/search: meals (name/restaurant), restaurants, members.

## Reachability / Discovery method
- Discovery = authenticated browser-sniff (browser-use / agent-browser / manual HAR). No spec, no docs, no community tooling exists.

## Community / Ecosystem (Step 1.5a preliminary)
- Official public developer API: **none found** (help center exposes only support/sales/security/status).
- GitHub/npm/PyPI/MCP/CLI wrappers: **none found** (the one repo named `forkable` is an unrelated sales workspace).
- Advertised integrations: Slack notifications; Okta/Rippling/Microsoft Entra SSO+SCIM provisioning. No public API, no subscribable webhooks (order updates go out via email/Slack/SMS).

## User Vision
- Read-only CLI: query/list/search/aggregate the user's Forkable lunch program from the terminal and via MCP. No mutations (no meal swaps, member provisioning, or payments) per user decision.

## Product Thesis
- Name: forkable-pp-cli ("Forkable CLI")
- Why it should exist: Forkable has zero programmatic surface today — no API, no docs, no tooling. A read-only CLI + MCP server lets teams script and query their lunch program (orders, meals served, spend, preferences, deliveries) offline with a local SQLite mirror — capabilities the web SPA does not offer.

## Build Priorities
1. P0: SQLite store for all captured entities + `sync` + offline `search` + `sql`.
2. P1: read commands for every captured resource (orders, meals, members, preferences, deliveries, billing, restaurants).
3. P2 (transcendence, pending capture): meals-served history/frequency, spend trends over time, upcoming-delivery digest, preference-vs-served drift, roster diffing.

## Reachability Gate (Phase 1.9)
- Decision: PASS
- Reason: authenticated browser capture contains real non-challenge traffic (9 GraphQL read ops returned data, no errors). Unauthenticated `me` query returns HTTP 401 — expected for session-auth API, not a block.
- Runtime: standard_http (probe-reachability 0.95). Session cookie + x-csrf-token required; replayable via direct HTTPS.
