# RapidAPI Hub Website — Browser-Sniff Discovery Report

**Run:** 20260828-090622-dd9c3930
**Target:** https://rapidapi.com/hub (the RapidAPI marketplace website itself)
**Backend:** chrome-devtools MCP (user's logged-in Chrome session, tab-scoped interceptor + network log)
**Date:** 2026-08-28

## 1. User Goal Flow

- **Goal:** Study the entire RapidAPI Hub website thoroughly — every page, command, and data surface — and produce a complete CLI for the website.
- Steps completed:
  1. Loaded `https://rapidapi.com/hub` (logged-in session) — captured network log: SSR Next.js app, 163 requests.
  2. Identified the app's data layer: **GraphQL BFF at `POST /gateway/graphql`** (4 calls per page load) + **Next.js RSC prefetches** (`?_rsc=<hash>`).
  3. Discovered CSRF bootstrap: `GET /gateway/csrf` → `{csrfToken}`.
  4. Enumerated the app's JS bundle graph (59 chunks, ~3MB) and extracted **225 unique GraphQL operations** from chunk `6944` (the platform GraphQL client).
  5. Live-validated 14 core operations against the real gateway, iterating on exact input shapes via the server's `BAD_USER_INPUT` / `GRAPHQL_VALIDATION_FAILED` hints.
  6. Walked search (`/search?term=weather`), category sidebar, collections, API detail, user profile, saved APIs, notifications, workspace, subscriptions, hub metrics.
- Coverage: 14 of 14 planned core surfaces validated live; 225 ops catalogued from bundle.

## 2. Pages & Interactions

| URL | Purpose | Interaction |
|---|---|---|
| https://rapidapi.com/hub | Marketplace home (hero, top categories, collections, API explorer) | Scroll, snapshot; fired 3 GraphQL + RSC prefetches |
| https://rapidapi.com/search?sortBy=ByRelevance | Full search (79,213 results, 50+ category facets) | Typed "weather", Enter; clicked sort |
| https://rapidapi.com/search?term=weather&sortBy=ByRelevance | Search results for "weather" | Snapshot, infinite scroll |
| /gateway/csrf | CSRF bootstrap | Fired via fetch |
| /gateway/graphql | GraphQL BFF | Fired 20+ live queries via interceptor |

## 3. Browser-Sniff Configuration

- Backend: chrome-devtools MCP (equivalent to chrome-MCP playbook: fresh tab, in-page fetch interceptor, network log).
- Pacing: ~1.2 req/s effective; zero 429s observed.
- Proxy pattern: **Not a REST proxy-envelope — a GraphQL BFF.** All data flows through `POST /gateway/graphql` with `operationName` + `query` + `variables`. No `x-proxy-routes` needed; instead treat each GraphQL operation as a spec path `POST /gateway/graphql#OperationName`.
- Client pattern: `graphql-bff`.

## 4. Endpoints Discovered

All GraphQL operations hit `POST https://rapidapi.com/gateway/graphql`. "Auth" column = whether the op needs the logged-in session (cookie-based).

| Method | Path/Operation | Status | Content-Type | Auth |
|---|---|---|---|---|
| POST | graphql#GetTopCategories | 200 | application/json | public |
| POST | graphql#GetCollectionsCollapsed | 200 | json | public |
| POST | graphql#getCollectionBySlug | 200 | json | public |
| POST | graphql#getCategoriesByCtx | 200 | json | public |
| POST | graphql#getCategories | 200 | json | public |
| POST | graphql#searchApis | 200 | json | public |
| POST | graphql#getApiBySlugAndOwner | 200 | json | public |
| POST | graphql#getApiById | 200 | json | public |
| POST | graphql#getApisByIds | 200 | json | public |
| POST | graphql#getUserProfile | 200 | json | public |
| POST | graphql#getHubMetrics | 200 | json | public |
| POST | graphql#getApiVersionPlayground | 200 | json | public |
| POST | graphql#activeUser | 200 | json | auth-required |
| POST | graphql#getUserSavedApis | 200 | json | auth-required |
| POST | graphql#getApiSubscriptions | 200 | json | auth-required |
| POST | graphql#getCtxSubscribedApis | 200 | json | auth-required |
| POST | graphql#getNotifications | 200 | json | auth-required |
| POST | graphql#getWorkspaceData | 200 | json | auth-required |
| GET | /gateway/csrf | 200 | json | public (auth-aware) |
| GET | /<route>?_rsc=<hash> | 200 | text/x-component | public/auth |
| POST | /authentication/login, /logout | 200 | json | public (login) |
| GET | /gateway/invite/:token/action/accept | 200 | json | auth |

## 5. Traffic Analysis

- **Protocol:** `graphql_bff` (confidence 0.98) — single POST endpoint, operationName-routed.
- **Auth signals:** cookie-based session (`rapidapi-context-id` cookie, `rapidapi-context` localStorage); CSRF token via `GET /gateway/csrf` → header `x-csrf-token`. Optional `x-rapid-role: admin` for admin ops. No Bearer/API-key header on the website's own BFF.
- **Parameter evidence:** search filters (`term`, `categoryNames`, `tags`), pagination (`first/after` relay), sorting (`sortingFields: [{by, fieldName}]`), metrics (`fromDate/toDate`).
- **Protection:** Cloudflare CDN (no challenge observed in-session); recaptcha on login form.
- **Generation hints:** `graphql_bff`, `auth_cookie_session`, `csrf_token_required`, `requires_authenticated_session_for_account_ops`.
- **Candidate commands:** `search`, `categories`, `collections`, `collection show`, `api show`, `user show`, `whoami`, `saved`, `subscriptions`, `notifications`, `workspace`, `metrics`, `csrf`.
- **Warnings:** none blocking — all 14 live probes returned 200 with real data. The 225-op bundle catalog includes console/studio/analytics ops that need deeper auth scopes (not all live-tested).

## 6. Coverage Analysis

- Resource types exercised: marketplace (search/categories/collections/API detail), user (profile/saved/active), account (subscriptions/workspace/notifications), hub (metrics), auth (csrf/login).
- Likely missed (documented in catalog, not live-tested): provider analytics, transactions/billing, team/org admin, app authorizations, tutorials, issues, NAC registrations, 2FA — all present in the bundle's 225-op catalog and implementable in the CLI.

## 7. Response Samples

All samples truncated to first ~1-2KB; full validated payloads were captured in-session.

- **searchApis** → `{"data":{"products":{"nodes":[{"id":"api_...","thumbnail":"https://rapidapi-prod-apis.s3.amazonaws.com/...","name":"meteostat","description":"Historical <em>Weather</em> & Climate Data API","slugifiedName":"meteostat","pricing":"FREEMIUM","updatedAt":"2025-01-08T06:57:13.504Z","categoryName":"<em>Weather</em>","isSavedApi":false,"score":{"popularityScore":9.9,"avgLatency":245,"avgServiceLevel":99,"avgSuccessRate":99},"user":{"id":4504574,"username":"meteostat","name":"Meteostat","type":"User"}},...],"facets":{"category":[...]},"pageInfo":{"endCursor":...,"hasNextPage":true},"total":...}}}`
- **getApiBySlugAndOwner** → `{"data":{"apiBySlugifiedNameAndOwnerName":{"id":"api_...","name":"meteostat","title":"meteostat","description":"Historical Weather & Climate Data API","gatewayIds":[],"createdAt":"1591788398052","status":"ACTIVE","longDescription":"...","apiType":"http","quality":{"score":211},"owner":{"id":"4504574","name":"Meteostat","slugifiedName":"meteostat"},"versions":[{"id":"apiversion_...","name":"v1","current":true}],"version":{"endpoints":[{"id":"apiendpoint_...","isGraphQL":false,"route":"/point/monthly","method":"GET","name":"Monthly Point Data","group":"apigroup_..."}]},"billingPlans":[...],"rating":{"rating":...,"votes":...},"subscriptionsCount":...,"websiteUrl":...}}}`
- **getCategoriesByCtx** → `{"data":{"categoriesByCtx":[{"id":"category_...","name":"Cybersecurity","weight":49,"thumbnail":"https://rapidapi-prod-collections.s3.amazonaws.com/category/Cyber%20Security.svg.xml","shortDescription":"...","slugifiedName":"cybersecurity","color":"rgba(185,205,255,0.4)"},...]}}`
- **getCollectionBySlug** → `{"data":{"collection":{"id":"collection_...","title":"Recommended APIs","slugifiedKey":"recommended-apis","collectionType":"PUBLIC","apis":[{"__typename":"Api","id":"api_...","name":"ExerciseDB","isFavorite":false,"pricing":"FREEMIUM"},...]}}}`
- **activeUser** → `{"data":{"activeUser":{"name":"Som Samantray","id":"10835499","mashapeId":"...","email":"...","username":"somsamantray","entity":{"id":10835499,"type":"User","status":"ACTIVE"},"billingType":"STRIPE","paymentProvider":"STRIPE","verified":true,"organizations":[]},"tenant":{"id":"1"}}}`
- **getApiSubscriptions** → `{"data":{"getApiSubscriptions":{"count":6,"rows":[{"id":10600991,"status":"ACTIVE","userId":10835499,"apiId":"api_...","billingPlanVersion":{"name":"V1","price":0,"period":"MONTHLY"}},...]}}}`
- **getWorkspaceData** → `{"data":{"workspaceData":{"ownedApis":{"apis":[{"id":"api_...","name":"👋 Demo Project"}],"metrics":{"averageErrorRate":0,"subscribers":0,"totalApis":1,"totalRequests":0}},"subscribedApis":{"apis":[{"name":"YouTube Media Downloader"},{"name":"IMDb"}],"subscriptions":[{"status":"ACTIVE"},...]}}}}}`
- **getHubMetrics** → `{"data":{"publicMetrics":{"publicApis":{"totalValue":78808},"users":{"totalValue":7688295},"activeApiConsumers":{"currentPeriodValue":176215},"totalApiTraffic":{"totalValue":250686083357}}}}`
- **getNotifications** → `{"data":{"newNotificationsByUserId":[{"id":10857387,"type":"info","isRead":false,"title":"Subscription Restored","body":"..."}]}}`
- **getUserProfile** → `{"data":{"userProfile":{"__typename":"User","id":"4504574","name":"Meteostat","username":"meteostat","bio":"The Weather's Record Keeper","publishedApisList":[{"id":"api_...","name":"meteostat","pricing":"FREEMIUM"}]}}}`
- **getUserSavedApis** → `{"data":{"userSavedApis":[]}}`
- **GetTopCategories** → `{"data":{"categories":[{"name":"Data","slugifiedName":"data","type":"category"},...]}}`
- **GetCollectionsCollapsed** → `{"data":{"collections":[{"id":"collection_...","title":"Recommended APIs","slugifiedKey":"recommended-apis","weight":2},...]}}`
- **getApiVersionPlayground** → `{"data":{"apiVersion":{"id":"apiversion_...","name":"v1","apiVersionType":"rest"}}}`

## 8. Rate Limiting Events

None observed. 20+ live GraphQL calls at ~1.2 req/s; all 200.

## 9. Authentication Context

- Authenticated session used (user logged into RapidAPI in their Chrome).
- Transfer method: none needed — chrome-devtools MCP drives the user's live logged-in Chrome session directly.
- Auth-required endpoints (only reachable with session): `activeUser`, `getUserSavedApis`, `getApiSubscriptions`, `getCtxSubscribedApis`, `getNotifications`, `getWorkspaceData`, account/console ops.
- Auth scheme: cookie session + `x-csrf-token` header (fetched from `/gateway/csrf`); no Authorization header on the BFF.
- Session state was not written to any artifact; only operation names and redacted response samples were captured.

## 10. Bundle Extraction

- Bundle analyzed: `https://rapidapi.com/hub/_next/static/chunks/6944-00dec59c7c12894e.js` (platform GraphQL client) plus 58 sibling chunks.
- API base URL discovered: `https://rapidapi.com/gateway/graphql` (POST).
- Endpoints found only in the bundle (not exercised): full 225-op catalog — analytics, transactions, billing, org/team admin, app authorizations, 2FA, tutorials, issues, NAC, certificates, workflows.
- Config extracted: header `rapid-client: default-auth-v2-dashboard-service`, admin header `x-rapid-role: admin`, input shapes for SearchApiWhereInput/SearchApiOrderByInput/PaginationInput/MetricsInput/GetSubscriptionInput, all fragments (ApiInfo, EndpointInfo, BillingPlanInfo, BillingPlanVersionInfo, ApiSubscriptionInfo, notification, Gateway, ApplicationAuthorization).
