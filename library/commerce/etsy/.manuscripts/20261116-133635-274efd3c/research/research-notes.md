# Etsy Open API v3 research notes (2026-11-16)

## 0. Spec
- URL: https://www.etsy.com/openapi/generated/oas/3.0.0.json -> HTTP 200, application/json. Saved as `etsy-openapi-v3.json`.
- `openapi: 3.0.2`, `info.version: 3.0.0`, title "Etsy Open API v3", server `https://openapi.etsy.com`. 77 paths, 107 operations, 83 component schemas. Global `security: [{api_key: []}]`.
- Operations by tag: ShopListing 17, Shop ShippingProfile 14, Shop Return Policy 6, Shop Section 5, Shop ProcessingProfiles 5, Shop Receipt 4, Shop Receipt Transactions 4, ShopListing File 4, ShopListing Image 4, ShopListing Video 4, Shop Economic Operators 4, Shop 4, ShopListing Inventory 3, ShopListing Personalization 3, ShopListing Translation 3, Payment 3, BuyerTaxonomy 2, SellerTaxonomy 2, Review 2, Ledger Entry 2, Other 2, Shop HolidayPreferences 2, ShopListing VariationImage 2, UserAddress 2, User 2, ShopListing Product 1, Shop ProductionPartner 1. (An op can carry >1 tag.)
- securitySchemes:
  - `api_key`: apiKey, header `x-api-key`. Spec description: "Every request to a v3 API endpoint must include this data in the format `keystring:shared_secret`."
  - `oauth2`: authorizationCode; authorizationUrl `https://www.etsy.com/oauth/connect`; tokenUrl `https://openapi.etsy.com/v3/public/oauth/token`. Scopes: address_r, address_w, email_r, listings_d, listings_r, listings_w, profile_r, profile_w, shops_r, shops_w, transactions_r, transactions_w.
- 76 ops need OAuth (api_key + oauth2 with scopes); 31 are API-key-only (list: buyer-taxonomy nodes/properties; seller-taxonomy nodes/properties; listings/{id}, listings/active, shops/{id}/listings/active, listings/batch, shops/{id}/listings/featured; listing images, videos, personalization, properties, translations, variation-images; listing reviews + shop reviews; openapi-ping; shipping-carriers; shops/{id}, shops (search), users/{id}/shops; shop return policies (list/get); shop sections (list/get); shop-sections/listings; `POST /v3/application/scopes` (token scope introspection)).
- Used scopes in ops: address_r, email_r, listings_d, listings_r, listings_w, shops_r, shops_w, transactions_r, transactions_w (profile_r/w, address_w defined but unused in ops).
- Spec quirk: the spec's `Other` tag has only openapi-ping + scopes. `ErrorSchema` = `{"error": string}` (required).
- Hosts: docs say `api.etsy.com/v3` and `openapi.etsy.com/v3` are equivalent. Docs show token URL `https://api.etsy.com/v3/public/oauth/token`; spec shows `openapi.etsy.com`. Both work per docs (not probed, token exchange needs a live user).

## 1. Auth, exactly
Sources: https://developers.etsy.com/documentation/essentials/authentication/ and /requests/ (fetched raw, 200).
- **Verdict: `x-api-key` MUST be `keystring:shared_secret`.** Authentication page: "Every request to a v3 endpoint must include an x-api-key header containing your keystring and shared secret separated by a colon: `x-api-key: <keystring>:<shared_secret>`". Both from https://www.etsy.com/developers/your-apps.
- Change announcement: etsy/open-api Discussion #1529 "ACTION REQUIRED: Requests Without Shared Secret Will Be Rejected After Feb 9, 2026" (bot post, Feb 2, 2026; "previously shared via email and earlier announcements"): "Starting February 9, 2026, all OpenAPI requests that do not include a shared secret will be rejected." https://github.com/etsy/open-api/discussions/1529. (A third-party client claims Jan 18, 2026 enforcement; unconfirmed.)
- Shared secret rotation shipped 2026-10-14 (Discussion #1748): self-serve in Developer Portal, both old and new secrets accepted during rotation.
- Failure body when bare keystring is sent (observed in probe): `{"error":"API key not found or not active, or incorrect shared secret for API key."}` (403). A community post (#1801, Nov 2026) shows another variant: `{"error":"Invalid API key: should be in the format 'keystring:shared_secret'."}`.
- Authorization header for scoped endpoints: `Authorization: Bearer <user_id>.<token>`; the access token already carries the numeric user_id prefix ("12345678.O1zL..."), refresh token too. Use whole string as given.
- OAuth 2.0 authorization code + PKCE (mandatory, S256): authorize `GET https://www.etsy.com/oauth/connect` params response_type=code, client_id (keystring), redirect_uri (https only, exact-match registered URI), scope (space-separated), state, code_challenge, code_challenge_method=S256. Verifier 43-128 chars [A-Za-z0-9._~-].
- Token: `POST https://api.etsy.com/v3/public/oauth/token` (x-www-form-urlencoded) grant_type=authorization_code, client_id, redirect_uri (optional), code, code_verifier. Response: access_token, token_type Bearer, expires_in 3600, refresh_token, scope (may be subset of requested).
- Lifetimes: access token 1 hour ("functional life of 1 hour", expires_in 3600); refresh token 90 days ("longer functional lifetime (90 days)"). Refresh: grant_type=refresh_token, client_id, refresh_token; returns a new access + refresh token (rotate and store the new refresh token). Scope can't change on refresh. Legacy: grant_type=token_exchange with legacy_token (OAuth1).
- Scopes (docs): address_r read shipping addresses; address_w update/delete address; email_r read user profile; listings_d delete listings; listings_r read inactive/expired listings; listings_w create/edit listings; profile_r/profile_w private profile; shops_r/shops_w; transactions_r (purchase and sales data, buyers and sellers); transactions_w update sales data. v2-era favorites/cart/recommend scopes exist in docs history but no active ops.
- Public (API-key-only) endpoints: see section 0 list (active listing search, listing/shop lookup, taxonomy, reviews, images, return policies, sections, ping, shipping-carriers).
- Access tiers: apps start in a "Personal Access" vs "Commercial Access" model; new keys may be "pending approval"; see section 7.

## 2. Rate limits
Source: https://developers.etsy.com/documentation/essentials/rate-limits/
- Limits are per API key (QPS and QPD). QPD is a sliding rolling-24h window (bucketed), not midnight reset. Order evaluated: QPS first, then QPD.
- Example header values on the page: limit per second 150, per day 100000 (examples; actual numbers visible per app in Developer Portal; older docs cited 10 QPS / 10,000 QPD default for new apps - unverified here).
- Response headers on success: `x-limit-per-second`, `x-remaining-this-second` (the page spells it "x-remaining-this-seco..." truncated in rendering; verify against the real header at build time), `x-limit-per-day`, `x-remaining-today`.
- Exceeded: HTTP 429 with `retry-after` (seconds). Release 2025-10-24 fixed an invalid Retry-After on daily quota hits. Recommend exponential backoff. Higher quota: email developer@etsy.com with app description + QPD/QPS estimate.
- Probe observation: the 403s did not return rate-limit headers (only CDN headers x-served-by/x-cache/x-timer).

## 3. Pagination
- Spec: `limit` integer 1-100, default 25; `offset` integer min 0 default 0 on list endpoints (listings/active, receipts, ledger-entries, etc.). Responses are `{count, results[]}`.
- listings/active: `sort_on` (created|price|updated|score) only works with a search option (keywords, region, ...); `score` always descending. A documented max-offset cap was NOT found in the spec; community reports of ~12,000-result search depth are not verified. Treat deep offset as unreliable; prefer syncing a shop's own listings via `shops/{id}/listings` (state filter) with offset/limit.
- receipts: min_created/max_created unix ts (min 946684800), was_paid, was_shipped, sort_on created|updated|receipt_id.

## 4. Key entities and endpoints (all under /v3/application)
- Shop: GET shops/{shop_id}, GET shops?shop_name=, GET users/{user_id}/shops, PUT shops/{id}; sections; holiday prefs; return policies; production partners; economic operators (GPSR, new Oct 2026).
- Listings: shops/{shop_id}/listings (state active/inactive/draft/expired; requires listings_r for non-active), POST create draft (listings_w), PATCH/PUT update, DELETE (listings_d), listings/active (search), listings/batch, shops/{id}/listings/featured. Sub-resources: images, videos (multi-video since Sept 2026), files, inventory (GET/PUT, `enable_scale_validation` Oct 2026), products, properties, translations, personalization, variation-images.
- Orders: shops/{id}/receipts (+ GET/PUT one), receipts/{id}/transactions, receipts/{id}/tracking (POST create shipment), shops/{id}/transactions.
- Money: shops/{id}/payment-account/ledger-entries (+ /payments), shops/{id}/payments, receipts/{id}/payments. All transactions_r.
- Shipping: shipping profiles (14 ops: profiles, destinations, upgrades), shipping-carriers, processing profiles (5 ops, replaces legacy processing days).
- Reviews: listings/{id}/reviews, shops/{id}/reviews (key-only). Taxonomy: seller-taxonomy/nodes(+properties), buyer-taxonomy/nodes(+properties). Users: users/{id}, user addresses.
- Recent changes (etsy/open-api Announcements): 2025-04-15 deprecated variation property_ids breaking change (#1378); Oct 2025 Processing Profiles transition; Feb 9 2026 shared secret enforcement; 2026-03-24 general release; 2026-09 multi-video listings, EU ECGT fields (#1721); 2026-09-30 new taxonomy property filters; 2026-10 inventory scale validation, listing+inventory webhooks (#1747: listing.created/updated/deleted, inventory.created/updated/deleted/out_of_stock, plus order events), shared secret rotation, `featured_rank` removed from UpdateListing (#1750), GPSR economic operators (#1774, enforced 2027-02-01).

## 5. Error model
- Spec: `ErrorSchema` = `{"error": "<message>"}`. Documented statuses per op: 400, 401, 403, 404, 409 (some), 429, 500. 
- Observed: 403 `{"error":"API key not found or not active, or incorrect shared secret for API key."}` for a bare keystring. 401 for missing/invalid Bearer; 403 for scope/permission denial (spec text: "attempted to perform an operation it is not allowed to"). 429 with retry-after.

## 6. Competitors / ecosystem
- **@profplum700/etsy-v3-api-client** (TypeScript SDK + `@profplum700/etsy-cli`): https://github.com/profplum700/etsy-v3-api-client; 1 star, pushed 2026-11-16; GitHub release v3.0.0 (2026-04-01); npm SDK latest 4.1.0 (modified 2026-10-24), ~2,393 downloads last month; CLI latest 2.5.4 (2026-03-30), ~35 downloads/mo. Covers OAuth PKCE, token refresh, listings and receipts commands, delete behind confirm. Only real npm CLI with traction; tiny.
- **@cprice70/etsy-cli** (npm 0.1.3, 2026-03-21, ~26 downloads/mo): Open API v3 wrapper, `etsy listings list --state draft`, JSON output.
- **etspi** (PyPI): CLI to manage shop/listings/variants as JSON (not examined in depth).
- MCP: **Bl0ck154/etsy-seller-mcp-hub** https://github.com/Bl0ck154/etsy-seller-mcp-hub (3 stars, v0.5.0 2026-11-02, pushed 2026-11-09, MIT): 79 tools + CLI (etsyctl) + local HTTP gateway, read-only by default, inventory/order reports, Node 22+. **aserper/etsy-mcp** https://github.com/aserper/etsy-mcp (13 stars, MIT, pushed 2026-04-02): 37 tools, OAuth PKCE. **administrativetrick/etsy-mcp-server** (21 stars, last push 2025-12-12): search/shop info. jeffkimble/etsy-mcp-server (0 stars, 2025-07, stale). Etsy's own "Dev MCP Server" is docs-only (endpoint schemas, no shop access).
- No existing "etsy-pp-cli"; no SQLite-mirror CLI found. Whitespace: local store + offline analytics/bulk edit.

## 7. Reachability risk
- Sources checked: etsy/open-api Discussions (Q&A), repo issues of wrapper repos (aserper/etsy-mcp: 1 open, OAuth server.address bug; Bl0ck154 hub: all closed; profplum700: 14 open, mostly dev chores; none about 403s).
- Biggest risk = **app approval/access**, not endpoint breakage: Nov 2026 Q&A is flooded with "Seller app instantly rejected", "Previously Approved Seller App Now Showing Rejected" (#1804), "Personal app ... banned without notice" (#1808; 35 draft creates ~300 writes, 10-min polling, paginated /listings/active keyword searches by a brand-new shop), "Commercial Access request banned twice" (#1764), "Remove developer mode from my shop" (many). Apps with new shops/low-history can be banned for modest automation. Keep call volume low, cache, avoid polling.
- #1801 (2026-11-12): 403 on openapi-ping despite "Personal Access" status; Etsy support reply: credentials re-activated, retry. So a 403 may be key status not format.
- Feb 9, 2026 shared-secret enforcement broke every bare-keystring client; any wrapper/MCP written earlier needs update.
- Rate limit Retry-After bug fixed 2025-10-24.

## 8. Probe (3 GETs, key only as `$ETSY_API_KEY`, value never logged)
Header sent: `x-api-key: $ETSY_API_KEY` (bare; the env var is presumed to be the keystring only).
- GET /v3/application/openapi-ping -> **403** `{"error":"API key not found or not active, or incorrect shared secret for API key."}`
- GET /v3/application/seller-taxonomy/nodes -> **403**, same body shape.
- GET /v3/application/listings/active?limit=1 -> **403**, same body shape.
- No x-limit-* headers on the 403s (response headers: content-type application/json, Fastly x-served-by/x-cache/x-timer only).
- Interpretation: consistent with the documented mandatory `keystring:shared_secret` format (or the key is inactive/pending). Not retried with a guessed secret. Build step needs `ETSY_SHARED_SECRET` (or `ETSY_API_KEY` as `keystring:secret`) from the user to verify live.

## 9. User pain points
1. **Access/approval opacity.** Personal vs Commercial access, apps "Rejected"/"Banned" without reason, "Developer Mode" shops stuck (Discussions #1804, #1808, #1764, many "remove developer mode" posts, Nov 2026).
2. **No bulk export/analytics path.** #1766 "Seller API Access - Automating Etsy's Existing CSV Export for Private Inventory Management" (2026-10-24): sellers want programmatic export of what CSV export already gives.
3. **Breaking-change churn for integrations:** shared secret (Feb 2026), variation property deprecation (Apr 2025), processing-profiles migration, `featured_rank` removal (Oct 2026), GPSR enforcement 2027-02-01.
(Renewal fees/bulk edit pain is domain knowledge; not separately cited here.)

## 10. Power-user workflows for a SQLite mirror
- Bulk tag/title/price/description edits with diff/dry-run (listings_w); tag/material/alt-text/photo-count quality audit across all listings.
- Expiring-listing renewal planner (state=expired / ending_tsz window; listing fees $0.20 each).
- Sales/fee reports from ledger entries + payments + receipts (monthly P&L, fees by type, payouts).
- Unshipped-order queue (receipts was_paid=true & was_shipped=false), tracking upload.
- Inventory/variation audit: out-of-stock, SKU gaps, price spread, sale-price consistency; webhook-free polling replacement via local sync.
- GPSR readiness check across EU-selling listings (manufacturer + responsible person missing) ahead of 2027-02-01.

## 11. Homepages
- Product: https://www.etsy.com  Developer portal: https://www.etsy.com/developers (Your Apps), docs https://developers.etsy.com/documentation, reference https://developers.etsy.com/documentation/reference, repo/community https://github.com/etsy/open-api.
