# CookUnity Browser-Sniff Discovery Report

## 1. User Goal Flow
- **Goal:** Browse the weekly menu and load the complete meal list (offline meal-planning source).
- **Steps completed:**
  1. Navigated to `www.cookunity.com` (logged-in) → redirected to app host `subscription.cookunity.com`.
  2. Confirmed authenticated session (Auth0 `is.authenticated` cookie, `ajs_user_id`).
  3. Loaded the date-keyed menu (`/?date=2026-08-04`); captured the SDUI menu load.
  4. Extracted the 17 lazy meal-cluster references from `clustered-results`.
  5. Replayed one meal cluster (`clustered-result`) with the correct auth → 13 MEAL cards with full `properties`.
- **Coverage:** Menu discovery complete (primary goal). Account/orders/subscription discovered via GraphQL responses (secondary).

## 2. Pages & Interactions
- `subscription.cookunity.com/` (home; menu categories rendered) — read nav/links, Performance API hosts.
- `subscription.cookunity.com/?date=2026-08-04` (date-keyed menu) — installed fetch/XHR interceptor, clicked category, captured GraphQL + SDUI calls.
- Replayed `GET /sdui-service/web/view/menu/components/clustered-result?...` in page context with the app's own token.

## 3. Browser-Sniff Configuration
- **Backend:** chrome-MCP was attempted first but the Chrome extension had no per-site permission for cookunity.com (read_page/JS/network all denied even after a grant attempt). Pivoted to **browser-harness (browser-use v0.1.8)** attached to the running, logged-in Chrome via CDP (user enabled "Allow remote debugging").
- **Capture method:** in-page `window.fetch`/XHR interceptor + CDP `Page.addScriptToEvaluateOnNewDocument`; direct in-page `fetch` replay with the app's token for full response bodies.
- **Pacing:** manual, a handful of requests; no 429s observed.
- **Proxy pattern:** not a proxy-envelope. Standard REST/SDUI + GraphQL over distinct paths.

## 4. Endpoints Discovered

| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| GET | `/sdui-service/web/view/menu/{date}/clustered-results` | 200 | application/json (SDUI) | auth-required |
| GET | `/sdui-service/web/view/menu/components/clustered-result` (params: currentDeliveryDate, filterBy, categoryId, subcategoryId, reference, view, currentOrderStatus) | 200 | application/json (SDUI) | auth-required |
| GET | `/sdui-service/view/v1/cart/{date}` | 200 | application/json (SDUI) | auth-required |
| GET | `/sdui-service/forethought/token` | 200 | application/json | auth-required |
| POST | `/subscription-back/graphql/user` | 200 | application/json | auth-required |
| POST | `/subscription-back/graphql/public` | 200 | application/json | public/auth |

GraphQL `user` operations observed (by response data keys): `users`, `upcomingDays`, `isExpressEnabled`, `isAllowedToSeeMembership`, `getUserChallenges`, `deliveryDateStatus`/`upcomingDayV2Detail`, `getNextOrderRecommendation`, `modals`. GraphQL `public`: `referralConfig`.

## 5. Traffic Analysis
- **Protocols:** SDUI (server-driven-UI JSON) for the menu/cart; GraphQL for account/orders; REST for token/misc.
- **Auth signals:** `Authorization` header carrying a **raw Auth0 JWT with NO `Bearer ` prefix**, plus custom headers `platform: web` (and `cu-platform`, `accept-version` on some calls). Token source: Auth0 SPA cache in `localStorage` (`@@auth0spajs@@::E3AWy6rDb3S3ErYliO64fnY171Ec1xhf::https://cookunity.com::openid ...`), audience `https://cookunity.com`, scope `openid profile email offline_access`, ~24h expiry, refresh token present.
- **Protection:** none at transport (marketing origin 200 over stdlib HTTP; app API replays over standard HTTP once the token header is supplied).
- **Reachability mode:** `standard_http`. The printed CLI ships standard HTTP transport; no clearance cookie, no resident browser.

## 6. Coverage Analysis
- **Exercised:** weekly menu (17 clusters across Meals/Breakfast/Bundles/Proteins/Drinks/Treats), meal cards (full detail), account/orders/deliveries (GraphQL), cart.
- **Meal record fields (from `MEAL.properties`):** id, inventoryId, sku, name, description, image, publicUrl, chef{id,firstname,lastname}, price/finalPrice/prices[] (per box size + express), calories, ratioCarb/Fat/Protein, nutritionalInfo{calories,carbs,fat,fiber,protein,sodium,sugar,saturatedFat,servingSize}, allergens[], cuisines[], meatType, filterBy[] (diet tags), stars, reviews, stock/inStock, category, ingredients[], heatInstructions{Microwave,Oven}, weight, isPremiumMeal, isFavorite, tagsLevel1/2/3, characteristics[].
- **Likely missed:** review text, chef bios, full order-mutation flows (add-to-cart POST), historical week menus (each week is a distinct date query).

## 7. Response Samples
- `discovery/samples/menu-clustered-results.json` — menu chrome (calendar, filters, section headers, 17 `FULL_MENU_LAZY_CLUSTER` refs). **PII (delivery address) redacted.**
- `discovery/samples/meal-cluster.json` — one cluster of 13 `MEAL` cards with full `properties` (clean; no PII).

## 8. Rate Limiting Events
None. A handful of requests; no 429s.

## 9. Authentication Context
- **Authenticated session used:** yes, via the user's running logged-in Chrome (CDP attach).
- **Auth-required endpoints:** all `/sdui-service/*` and `/subscription-back/graphql/user`.
- **Auth scheme:** `Authorization: <raw Auth0 JWT>` (no `Bearer ` prefix) + `platform: web` header. Token lives in localStorage (Auth0 SPA cache), NOT a cookie — so a cookie-only `auth login --chrome` cannot capture it; the CLI reads a pasted/env token (`COOKUNITY_TOKEN`). Auth0 refresh-token flow (client_id `E3AWy6rDb3S3ErYliO64fnY171Ec1xhf`, token URL `auth.cookunity.com/oauth/token`) is a candidate transcendence feature for auto-refresh.
- **Session/token state excluded from manuscript archiving:** yes — the user's JWT was used only in-page for replay and was never written to any file. All saved samples were scanned and scrubbed (address PII removed).

## 10. Bundle Extraction
Not run (SDUI + GraphQL discovery was sufficient for the primary goal).
