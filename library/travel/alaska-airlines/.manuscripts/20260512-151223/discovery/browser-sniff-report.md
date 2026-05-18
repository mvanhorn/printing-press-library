# Alaska Airlines Browser-Sniff Report (2026-05-12 run)

## Capture method
CDP Network listener attached to browser-use's Chrome (port 60896). Manual flow driven via CDP Input.dispatchMouseEvent (real OS-level mouse, bypasses Auro shadow-DOM listener bindings). Listener PID retired after 2,432 events captured.

## Authenticated session
- Account: Matthew Van Horn / Atmos Gold / oneworld Sapphire
- Atmos Rewards #211405880 (cookie-based, Auth0 JWT in `guestsession`)
- Home airport: Hanalei, HNL
- TSA PreCheck: 984064388 (loaded on auto-fill)

## Booking funnel mapped (full chain captured)
1. `/` (home) → SSR with auro-* web components
2. `/search/results?O=SFO&D=SEA&OD=2026-11-27&DD=2026-11-30&A=1&RT=true&locale=en-us` — SvelteKit results page
3. Outbound fare selected → page expands in place ("CURRENTLY SELECTED")
4. Return fare selected → "Trip summary" view
5. `POST /search/results?/handleAddToCart` (SvelteKit form action) → cart loaded
6. `/search/cart?A=1&F1=<leg1>&F2=<leg2>&Slices=...&FARE=441.80&FT=rt&...` — cart page
7. `POST /search/cart?/checkout` → guest-info page
8. `/book/guest-info` — Travelers info entry (auto-prefilled from Atmos profile)
9. After travelers: → seats → review & pay → confirmation (NOT captured — stopped before personal data entry)

## Real API endpoints captured (all alaskaair.com unless noted)

### Reference / lookup
- `GET /search/api/citySearch/lookup/codeshare/{IATA}` — Airport details + codeshare info per code
- `GET /search/api/citySearch/getAllAirports` — Full AS airport list
- `GET /search/api/etinfo` — Electronic ticket metadata
- `GET /search/api/alaskaForBusiness` — Business-program flag info
- `GET /retaincontent/retrostack/api/getHeaderFooterLinks?variant=alaska&locale=en-us`
- `GET /api/cruises` — Cruise products
- `GET /retaincontent/retrostack/api/cars` — Cars retail content

### Search / shopping (SvelteKit)
- `GET /search/results?O=X&D=Y&OD=YYYY-MM-DD&DD=YYYY-MM-DD&A=N&C=N&L=N&RT=true|false&locale=en-us` — Page (SSR HTML or `+__data.json` for JSON)
- `POST /search/api/shoulderDates` — Flexible-date pricing matrix. Body: `{"origins":[],"destinations":[],"dates":[],"onba":false,"dnba":false,"numADTs":N,"numCHDs":N,...}`
- `POST /search/results?/handleAddToCart` — SvelteKit form action. Body: form-encoded `slices={"sessionId":..., "solutionSetIds":..., "solutionIds":...}`
- `GET /search/cart/__data.json?<deeplink params>` — Cart state JSON
- `POST /search/cart?/checkout` — SvelteKit form action → /book/guest-info
- `POST /search/api/mbxsession` — Session tracking. Body: `{"sessionId","solutionId","solutionSetId","isInternational","hasAwardPoints","isCodeshare","isInterline"}`
- `POST /search/api/trackEvent/{event_name}/{boolean}` — Analytics. Body: `{"userId","attributes":{}}`
- `POST /search/api/getFeatures/false` — Feature flags. Body: `{"userId"}`

### Account / Auth (authenticated)
- `GET /services/v1/myaccount/getloginstatus?t=<unix-ms>` — Login status check
- `GET /account/token` — User session token refresh
- `GET /atmosrewards/account/token` — Atmos Rewards token refresh
- `GET apis.alaskaair.com/1/marketing/loyaltymanagement/wallet/wallet/balance?mileagePlanNumber=N` — Atmos Rewards points balance

### Auth0 (issuer)
- `GET auth0.alaskaair.com/.well-known/jwks.json` — JWT signing keys
- Auth0 session cookies: `auth0`, `auth0_compat`, `did`, `did_compat`
- AS-domain session cookies: `AS_ACNT`, `AS_NAME`, `as_pers`, `guestsession`, `guestidentity`, `ASSession`, `ASSessionSSL`, `ASLBSA`, `ASLBSACORS`

### Bot detection
- `GET /_fs-ch-1T1wmsGaOgGaSxcX/check-detection` — FullStory / similar bot detection challenge
- Quantum Metric session beacons (filtered out as noise)
- AppDynamics RUM (filtered out)

## Replayability assessment

| Endpoint class | Replayable from Go binary (Surf + cookies)? |
|---|---|
| Reference (citysearch, getAllAirports, etinfo) | YES — public, GET, no CSRF |
| Search results (URL-deeplink + __data.json) | YES — URL params are self-encoding; HTML or JSON parseable |
| Shoulder dates (POST) | LIKELY YES — JSON body, no obvious nonce |
| Add to cart (SvelteKit form action) | UNCERTAIN — depends on whether sessionId/solutionId need CSRF; needs Surf with cookie session |
| Cart __data.json (GET) | YES — deeplinkable from F1/F2 params |
| Checkout handoff (POST /search/cart?/checkout) | UNCERTAIN — same as cart add |
| Atmos Rewards / account APIs | YES — authenticated via cookie session |
| Final pay submit | NO — captured stage not reached; assumed CSRF + 3DS + CAPTCHA |

## Auth model for printed CLI
- Type: `composed` cookie auth via `auth login --chrome` import
- Required cookies for authenticated calls: `AS_ACNT`, `AS_NAME`, `guestsession` (Auth0 JWT), `ASSession`, `ASSessionSSL`
- Auth0 JWT in `guestsession.AccessToken` has 30-min lifetime; refresh via `GET /account/token`

## Recommended CLI scope
- **`alaska-airlines-pp-cli airports {get|list}`** — citySearch + getAllAirports
- **`alaska-airlines-pp-cli search SFO SEA --date <YYYY-MM-DD> [--return <date>] [--pax 2A4C]`** — fetch /search/results/__data.json (or HTML), parse flights+fares
- **`alaska-airlines-pp-cli search flex SFO SEA --date <YYYY-MM-DD> --days 3`** — shoulderDates POST
- **`alaska-airlines-pp-cli wallet balance`** — Atmos Rewards balance (authenticated)
- **`alaska-airlines-pp-cli account status`** — login status (authenticated)
- **`alaska-airlines-pp-cli book prepare SFO SEA --date ... --pax ... --fare saver`** — builds the cart-deeplink URL and opens it in user's browser; user clicks "Continue to checkout" and completes payment manually
- **`alaska-airlines-pp-cli auth login --chrome`** — extracts cookies from Chrome's profile via cookie domain `.alaskaair.com`
- **`alaska-airlines-pp-cli auth status`** — validates Auth0 JWT in guestsession cookie

## What's NOT shippable
- **Full automated booking with saved card.** The pay submit POST requires session-bound CSRF + 3DS payment auth + likely CAPTCHA. Static Go binaries can't replay this. The "book --prepare" deeplink hand-off is the safe alternative — the user clicks the final pay button themselves.
