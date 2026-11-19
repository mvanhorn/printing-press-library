# Etsy CLI Brief

## API Identity
- Domain: Etsy Open API v3 (`https://openapi.etsy.com/v3/application`; `api.etsy.com/v3` equivalent). Seller-side marketplace API: shops, listings and their sub-resources, orders (receipts/transactions), money (payments, ledger entries), shipping/processing profiles, reviews, taxonomy.
- Users: Etsy sellers running one (sometimes several) shops; developers and agents automating shop ops. Buyers are a minor audience (public listing search, reviews).
- Data profile: Per-shop catalogs of hundreds to tens of thousands of listings. Each listing carries tags, materials, images, inventory, variations and properties. There's a steady stream of receipts and transactions, and a ledger of fees, payouts and refunds. Mostly owner-private data behind OAuth; a public slice (active listings, shop profile, reviews, taxonomy) needs only the API key.
- Spec: official OpenAPI 3.0.2 at `https://www.etsy.com/openapi/generated/oas/3.0.0.json` (HTTP 200). 77 paths, 107 operations, 83 schemas. Saved to `research/etsy-openapi-v3.json`.
- Homepage: https://www.etsy.com (product). Developer portal: https://www.etsy.com/developers. Docs: https://developers.etsy.com/documentation. Community: https://github.com/etsy/open-api.

## Auth (load-bearing — the generator must get this right)
- `x-api-key: <keystring>:<shared_secret>` on **every** request. This is from the spec's own `api_key` description and the Authentication docs page. It has been enforced since 2026-02-09 (etsy/open-api #1529). A bare keystring returns 403 `{"error":"API key not found or not active, or incorrect shared secret for API key."}`.
- Env vars: `ETSY_API_KEY` (keystring) + `ETSY_SHARED_SECRET`. The CLI composes the header from the two. It should also accept `ETSY_API_KEY` already in `keystring:secret` form (detect the colon), so a single-var setup works. Never log either value.
- OAuth 2.0 authorization code + **PKCE S256 (mandatory)**.
  - Authorize URL: `https://www.etsy.com/oauth/connect`. The `redirect_uri` must be https and exactly match the registered URI.
  - Token URL: `POST https://api.etsy.com/v3/public/oauth/token` (form-encoded).
  - Lifetimes: access token 1 h, refresh token 90 d.
  - The refresh token rotates on every refresh, so persist the new one.
  - Requests send `Authorization: Bearer <user_id>.<token>`. The numeric user_id prefix is already part of the token string; send it whole.
- Scopes: `address_r address_w email_r listings_d listings_r listings_w profile_r profile_w shops_r shops_w transactions_r transactions_w`.
  - `auth login` defaults to a read-only set: `listings_r shops_r transactions_r email_r`.
  - Write scopes require an explicit opt-in flag.
- Endpoint split:
  - 31 operations need only the API key: active-listing search and lookup, shop lookup and search, reviews, taxonomy, listing images/videos/properties, return policies, sections, shipping-carriers, `openapi-ping`.
  - 76 operations need OAuth plus the API key.

## Reachability Risk
- **Medium**, driven by account policy, not transport.
  - Nov 2026 community Q&A shows apps "Rejected" or "Banned" without reason (#1804, #1808, #1764). One personal app was banned after ~300 draft-creating writes and 10-minute polling on a new shop.
  - #1801: a valid "Personal Access" key got 403 on `openapi-ping` until Etsy support re-activated it. So a 403 can mean key status, not format.
- Implications for the CLI:
  - Keep call volume low and cache aggressively. That's the whole point of the local store.
  - No built-in polling loops.
  - Writes are opt-in, dry-run by default, and throttled.
  - `doctor` must distinguish four cases: missing shared secret (format error), inactive or pending key (403 with a valid format), missing scope (403 on a scoped call), and rate limit (429 + `retry-after`).
- Probe with the user's bare key: 3/3 returned 403 (expected; the shared secret is missing). No `x-limit-*` headers came back on the 403s. Live verification needs `ETSY_SHARED_SECRET` from the user.
- Rate limits are per key: QPS plus a rolling-24h QPD. Headers: `x-limit-per-second`, `x-remaining-this-second` (verify the spelling live), `x-limit-per-day`, `x-remaining-today`. Over-limit returns 429 with `retry-after`.
- Probe-safe endpoint: `GET /v3/application/openapi-ping` (API-key-only, no side effects).

## Top Workflows
1. **Mirror my shop locally and query it offline.** Sync listings (all states), inventory, receipts, transactions, ledger entries and reviews into SQLite, then search, filter and report without spending API quota.
2. **Order fulfilment queue.** List unshipped paid receipts with buyer and shipping details. Add tracking (`POST receipts/{id}/tracking`) behind a confirm gate.
3. **Money reports.** Monthly sales, Etsy fees by type, refunds and payouts from ledger entries and payments, giving a P&L per period and per listing.
4. **Catalog hygiene and bulk edits.** Audit listings for missing tags, materials, alt text or photos, low tag counts, and expiring or expired items (renewal planner). Bulk price/tag/title edits run with a diff and dry-run, then apply in throttled batches.
5. **Inventory audit.** Out-of-stock variations, SKU gaps and price spread across variations. Also a GPSR-readiness check (economic operators) before the 2027-02-01 enforcement date.

## Table Stakes
- Full CRUD coverage of the spec: shops, listings plus all sub-resources, receipts, transactions, tracking, payments, ledger, shipping/processing profiles, return policies, sections, reviews, taxonomy, users and addresses.
- OAuth PKCE login with token refresh and rotation (competitors aserper/etsy-mcp and profplum700 both have this).
- JSON output for agents, plus table output for humans.
- Pagination handling (limit 1–100, default 25; offset). No max offset is documented, so don't assume deep offsets work. Prefer syncing a shop's own listings via `shops/{id}/listings?state=` over deep `listings/active` search.
- Read-only by default (Bl0ck154/etsy-seller-mcp-hub sets that bar). Destructive operations (`listings_d`, deletes) require explicit confirmation.

## Data Layer
- Primary entities: `shops`, `listings` (state, tags, materials, prices, quantity, ending/updated timestamps), `listing_inventory`/`listing_products`/`offerings`, `listing_images` (alt text, rank), `receipts`, `transactions`, `ledger_entries`, `payments`, `reviews`, `shipping_profiles`, `taxonomy_nodes` (cached, rarely changes).
- Sync cursor:
  - Receipts: `min_created`, or `sort_on=updated` with a stored high-water mark.
  - Listings: per-state full pages using `updated_timestamp` as the high-water mark.
  - Ledger entries: `min_created`/`max_created` windows.
  - Reviews: `min_created`.
- FTS/search: listing title, description, tags and materials; review text; buyer names and messages on receipts (PII, so keep them local and never in proofs).
- PII note: receipts carry buyer names, emails and shipping addresses. The store lives in `~/.local/share/etsy-pp-cli/` only. Redact them from dogfood proofs and manuscripts.

## Codebase Intelligence
- The closest prior art (MCP servers and the profplum700 SDK) are thin request mirrors. None has a local store, offline analytics or bulk-edit dry-run. Anything written before Feb 2026 that sends a bare keystring is now broken.

## Product Thesis
- Name: `etsy-pp-cli` ("Etsy shop command center").
- Why it should exist: Sellers' real jobs are fulfilment, fee and payout reconciliation, catalog hygiene and bulk edits. Etsy's dashboard and CSV exports do these poorly, and existing tools are thin request wrappers that burn quota and invite bans. A local SQLite mirror makes every report and audit free after one sync, and gives agents a safe, read-first surface. Writes run dry-run-first with throttled batching, which respects the quota and the ban risk.

## Build Priorities
1. Correct auth: composed `keystring:shared_secret` header plus OAuth PKCE login/refresh/rotation, with `doctor` diagnosing format, status, scope and rate-limit failures.
2. Full-spec command surface (read-only commands first-class, writes gated) plus incremental `sync` into SQLite for listings, inventory, receipts, transactions, ledger, payments and reviews.
3. Offline transcendence: `orders unshipped`, `money report`, `listings audit`, `listings expiring`, `inventory audit`, and FTS `search` across the mirror.
4. Bulk edit with diff/dry-run/apply (throttled, resumable) for price, tags and title.
5. Rate-limit-aware client: honors `retry-after`, surfaces the remaining quota, and runs no polling loops.

## Reachability Gate (Phase 1.9, 2026-11-16)
- Probe: `GET /v3/application/openapi-ping` (API-key-only, side-effect free).
- No key: 403 `{"error":"Invalid API key: should be in the format 'keystring:shared_secret'."}`; with user's keystring:shared_secret: 403 `{"error":"API key not found or not active, or incorrect shared secret for API key."}`.
- Tier/permission hints from 4xx body: "not active", "shared secret" — key/app status, not bot protection (JSON bodies from the API host, no challenge HTML).
- OAuth grant probe (authorizationCode + PKCE S256, scope listings_r, read-only, no code exchange): www.etsy.com redirected to `/oauth2/signin` with HTTP 403 and no OAuth error code → WARN (web sign-in page bot screening of curl; not an OAuth misconfiguration). Redirect URI is user-registered and unknown to the spec; generated `auth login` must take `--redirect-uri` / `ETSY_REDIRECT_URI`.
- Verdict: WARN — API reachable and answering programmatically; live verification blocked until the user's app key is active.

## Live key verification (2026-11-16, after user fixed env)
- `GET /openapi-ping` with `x-api-key: keystring:shared_secret` -> 200. Earlier 403s were the two values swapped between ETSY_API_KEY and ETSY_SHARED_SECRET.
- This app's real limits (response headers): `x-limit-per-second: 5`, `x-limit-per-day: 5000`, `x-remaining-this-second`, `x-remaining-today` (spelling confirmed). Docs' 150/100000 are examples only.
- Design consequence: default call budgets must be small (market snapshot default --max-calls well under 5000/day; sync paced <=5 QPS; doctor reports x-remaining-today).
