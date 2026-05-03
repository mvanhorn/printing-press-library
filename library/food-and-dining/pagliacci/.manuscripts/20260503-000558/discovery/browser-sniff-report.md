# Pagliacci Browser-Sniff Report (v2 run, 2026-04-26)

## User Goal Flow
- Goal: Order a pizza for delivery (with auth context)
- Steps completed: Homepage load, /order menu page load, /loyalty visit, /locations click navigation, authenticated XHR capture, JS bundle endpoint extraction
- Steps skipped: Cart add, OrderPrice POST, OrderSend POST (would require submitting a real order)
- Coverage: Public + authenticated surface well-covered. Auth-behind endpoints (Customer profile, AddressName, RewardCard, StoredCredit, StoredCoupons, OrderList, OrderListItem, OrderListPending, QuoteBuilding) all observed via real XHR with valid 200 responses.

## Pages & Interactions
1. Homepage `pagliacci.com/` — loaded via browser-use `--headed` (login flow)
2. Login page — user logged in via fresh form submission (prior Chrome session cookies were session-scoped and dropped)
3. `/order` (menu) — loaded post-login, fired ~20 API calls including authenticated profile/orders/rewards
4. `/locations` — clicked via SPA navigation, fired more QuoteStore + TimeWindow calls
5. `/loyalty` — Contentful CMS page, no pag-api calls

## Browser-Sniff Configuration
- Backend: browser-use 0.12.5 (CLI mode, no LLM key required)
- Session transfer: Two-step. First attempted agent-browser auto-connect from running Chrome (saved 8 pagliacci.com cookies including customerId + authToken). Then quit Chrome, started browser-use with `--profile "Default"`. **The customerId/authToken cookies are session-scoped — they did not survive the Chrome quit.** Fell back to fresh headed login via `browser-use --headed --session pagliacci-auth open https://pagliacci.com/login`. After user login, captured cookies in the headed session.
- Pacing: 1-2s between major navigations. No 429s observed.
- Proxy pattern detected: No (direct REST endpoints, no envelope)
- Effective rate: ~1 req/s during interactive interactions; ~3 req/s during page-load bursts (browser-controlled)

## Endpoints Discovered

### Observed via XHR/Performance API (live capture)
| Method | Path | Status | Auth | Notes |
|--------|------|--------|------|-------|
| GET    | /Version | 200 | public | API version check |
| GET    | /Store | 200 | public | All store locations |
| GET    | /QuoteStore | 200 | public | All store quote metadata |
| GET    | /QuoteStore/{id} | 200 | public | One store's quote |
| POST   | /QuoteStore/{id} | 200 | public | Compute quote with cart |
| GET    | /MenuTop/{storeId} | 200 | public | Featured menu items |
| GET    | /MenuCache/{storeId} | 200 | public | Full menu |
| GET    | /MenuSlices | 200 | public | Today's slices across stores |
| GET    | /TimeWindowDays/{storeId}/{type} | 200 | public | Available days for DEL/PICK |
| GET    | /TimeWindows/{storeId}/{type} | 200 | public | All available windows |
| GET    | /TimeWindows/{storeId}/{type}/{date} | 200 | public | Slots for a specific date |
| GET    | /SiteWideMessage | (cached) | public | Site banner text |
| POST   | /Login | 200 | public | Sets customerId+authToken cookies |
| GET    | /Customer/{id} | 200 | auth-required | Customer profile |
| GET    | /AddressName | 200 | auth-required | Saved addresses |
| GET    | /StoredCredit | 200 | auth-required | Account credit |
| GET    | /StoredCoupons | 200 | auth-required | Saved coupons |
| GET    | /RewardCard | 200 | auth-required | Reward card balance |
| GET    | /OrderList/{page}/{size} | 200 | auth-required | Paginated order history |
| GET    | /OrderListItem/{orderId} | 200 | auth-required | Full order detail |
| GET    | /OrderListPending | 404 | auth-required | No pending orders for this customer (404 expected when empty) |
| GET    | /QuoteBuilding/{buildingId} | 200 | auth-required | Cart state |

### Discovered via JS bundle extraction (`main.BPDM6VAU.js`, 2.7 MB)
Supplementary endpoints found in the bundle but not exercised during the live sniff:

| Method | Path | Source | Notes |
|--------|------|--------|-------|
| POST   | /Logout | bundle | Invalidate session |
| POST   | /Register | bundle | Account creation |
| POST   | /PasswordForgot | bundle | Password reset request |
| POST   | /PasswordReset | bundle | Password reset apply |
| GET    | /ConfirmEmail/{token} | bundle | Email confirm |
| POST   | /CreateToken | bundle | Token refresh |
| POST   | /AddressInfo | bundle | Address validate / autocomplete |
| GET    | /AddressInfo/{id} | bundle | Get address by ID |
| POST   | /AddressName | bundle | Create saved address |
| DELETE | /AddressName/{id} | bundle | Delete saved address |
| POST   | /OrderPrice | bundle | Price an order |
| POST   | /OrderSend | bundle | Submit an order |
| GET    | /OrderClone/{id} | bundle | Reorder transform |
| GET    | /OrderSuggestion/{customerId} | bundle | Personalized suggestions |
| GET    | /OrderListGC | bundle | Gift-card-purchase orders |
| POST   | /ProductPrice | bundle | Product pricing |
| GET    | /Store/{id} | bundle | Single store |
| GET    | /CouponSerial/{serial} | bundle | Coupon lookup |
| GET    | /RewardHistory/{customerId}/{count} | bundle | Reward history |
| GET    | /StoredCredit/{id} | bundle | Credit entry |
| DELETE | /StoredCredit/{id} | bundle | Remove credit |
| GET    | /StoredGift | bundle | Saved gift cards |
| GET    | /StoredGift/{id} | bundle | Gift card by ID |
| DELETE | /StoredGift/{id} | bundle | Remove gift card |
| GET    | /CheckGift/{id}/{pin} | bundle | Public gift balance check |
| GET    | /GiftValue/{id} | bundle | Gift current value |
| POST   | /TransferGift | bundle | Transfer gift balance |
| POST   | /Feedback | bundle | Submit feedback |
| GET    | /Feedback/{id} | bundle | Get feedback by ID |
| GET    | /AccessDevice | bundle | List devices |
| DELETE | /AccessDevice/{id} | bundle | Revoke device |
| POST   | /MigrateQuestion | bundle | Migration question |
| POST   | /MigrateAnswer | bundle | Migration answer |

**Total endpoint inventory:** 22 live + 33 bundle-only = ~55 endpoints (some overlap with live).

## Traffic Analysis Summary
- **Protocols:** `rest_json` (98% confidence)
- **Auth signal:** `composed` type, `Authorization` header, cookies `customerId` + `authToken`, domain `pagliacci.com`
  - Construction confirmed via XHR header interceptor: `Authorization: PagliacciAuth 2432962|DAB10EFD-5637-449C-A873-CAF8D92491FB`
  - Format string: `PagliacciAuth {customerId}|{authToken}`
- **Reachability:** `standard_http` (95% confidence) — direct curl/HTTPS replay works once cookies are extracted
- **Generation hints:** `requires_browser_auth`, `composed_auth`
- **Warnings:** None

## Coverage Analysis
**Exercised:** stores (Store, QuoteStore), menu (MenuTop, MenuCache, MenuSlices), scheduling (TimeWindowDays, TimeWindows), customer profile (Customer), address book (AddressName), credit (StoredCredit), rewards (RewardCard, StoredCoupons), orders (OrderList, OrderListItem, OrderListPending, QuoteBuilding).

**Likely missed at runtime, captured via bundle:** address validation (AddressInfo POST), order placement (OrderPrice/OrderSend POST), product pricing (ProductPrice POST), gift cards (StoredGift, CheckGift, GiftValue, TransferGift), account migration (MigrateAnswer, MigrateQuestion), device management (AccessDevice), email confirm (ConfirmEmail), reward history (RewardHistory).

These are still emitted in the spec because the bundle proves they exist; the generated CLI provides commands for them and the user can exercise them when needed.

## Response Samples
- `Store`: array of objects with `ID`, `Name`, `Address`, `City`, `State`, `Zip`, `GPS`, `OpenHour`, `CloseHour`, etc.
- `MenuCache`: nested categories with products, prices, descriptions, image URLs
- `MenuSlices`: array of available slices today by store
- `QuoteStore/{id}` (POST): `{"ID":492,"Delivery":"30","Drone":"X","Pickup":"15"}` — wait minutes for delivery, drone status (X = not available), pickup wait
- `TimeWindowDays`: `[{"ID":20260425,"Day":"2026-04-25T...","Available":true},...]`
- `TimeWindows/{storeId}/{type}/{date}`: `{"Allowed":["2026-04-26T11:00:00","2026-04-26T11:15:00",...]}` — array of allowed slot timestamps
- `OrderListItem/{id}`: full order: `{"ID":...,"Nightly":172,"OrderDate":"...","Store":490,"Building":...}`
- `Customer/{id}` (cached in localStorage): `{"data":{"ID":...,"Phone":"...","Email":"...","Name":"...","Condiment":"CP","LastMethod":"DELIVERY","OrderCount":22},"expiration":"..."}`
- `QuoteBuilding/{id}`: `{"Extra":0}` placeholder when cart is empty

## Rate Limiting Events
None. 26 API requests across the session, no 429s, no backoffs needed.

## Authentication Context
- **Used authenticated session:** Yes
- **Transfer method:** Fresh headed login via `browser-use --headed --session pagliacci-auth open https://pagliacci.com/login`. Prior auto-connect grab from Chrome captured the cookies but they were session-scoped and dropped on Chrome quit.
- **Auth header scheme:** `Authorization: PagliacciAuth {customerId}|{authToken}` — composed client-side from cookies
- **Cookie domain:** `pagliacci.com`
- **Cookies needed:** `customerId`, `authToken`
- **Auth-required endpoints (verified):** /Customer/{id}, /AddressName, /StoredCredit, /StoredCoupons, /RewardCard, /OrderList/*, /OrderListItem/*, /OrderListPending, /QuoteBuilding/*
- **Session state archiving:** session-state.json will be deleted before manuscript archive (Phase 5.6)

## Bundle Extraction
- Bundle URL: `https://pagliacci.com/main.BPDM6VAU.js` (2.7 MB)
- API base discovered: `https://pag-api.azurewebsites.net/api`
- Endpoints found only in bundle: ~33 (see bundle table above)
- Auth construction: `Authorization: PagliacciAuth ${e.customerId}|${e.authToken}` — exact string from bundle
- Required header: `Version-Num: 1.3` per bundle source (also present in prior spec)
