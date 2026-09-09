# SEEK Browser-Sniff Discovery Report

Run: 20260908-143250-21abe609 · Target: `https://au.seek.com/` · Backend: **chrome-MCP**
(user's logged-in Chrome, fresh capture tab) + direct HTTP probes.

## 1. User Goal Flow

- **Goal:** "Search for jobs by keyword + location, open a listing, read its full details."
- **Secondary (authenticated):** "See my saved searches and saved/applied jobs."
- **Steps completed:**
  1. Loaded `au.seek.com/software-engineer-jobs/in-Sydney-NSW` (SSR search results) — no client XHR (server-rendered).
  2. Clicked a job card → URL `?jobId=94483533` → fired `POST /graphql` ×4: `jobDetailsPersonalised`, `JobDetailsRecommendedJobs`, `GetMatchedQualities`, + one batched op.
  3. Opened "Listing time" filter → "Last 7 days" → URL `?daterange=7`; fired `POST /graphql`: `JobCountV7` (uses `jobSearchV7`), `SearchSavedAndAppliedJobs`, `GetSalaryNudge`, `GetBanner`; results re-rendered via a Next.js RSC `POST` (204) to the page route.
  4. Navigated to `/my-activity/saved-searches` (SSR) — 7 saved searches rendered; `window.__APOLLO_CLIENT__` cache holds `ApacSavedSearch` objects.
  5. Navigated to `/my-activity/saved-jobs` (SSR) — `viewer { savedJobs(first: 100) { id isActive notes isExternal … } emailAddress personalDetails }`.
- **Steps skipped:** applied-jobs page (inferred from `viewer.searchAppliedJobs`), destructive toggles (delete search / toggle alert) — not exercised (mutations on user data).
- **Coverage:** 5 of 5 planned read steps.

## 2. Pages & Interactions

| # | URL | Interaction |
|---|-----|-------------|
| 1 | `au.seek.com/software-engineer-jobs/in-Sydney-NSW` | page load |
| 2 | same + `?jobId=94483533` | clicked job title "Graduate Software Engineer" |
| 3 | `…/in-Sydney-NSW-2000?daterange=7` | opened "Listing time" dropdown, clicked "Last 7 days" |
| 4 | `au.seek.com/my-activity/saved-searches` | page load (authenticated) |
| 5 | `au.seek.com/my-activity/saved-jobs` | page load (authenticated) |

## 3. Browser-Sniff Configuration

- Backend: chrome-MCP (`mcp__claude-in-chrome__*`) — fresh tab in a new tab group, closed at end.
- In-page `fetch`/XHR interceptor installed after each navigation (page-load calls captured via `read_network_requests` metadata + Apollo cache extraction).
- Pacing: manual, ~1 call/2s; no 429s.
- Proxy-envelope: **not detected** (GraphQL BFF at a single `/graphql` path, standard `{operationName,query,variables}` bodies).
- Direct HTTP probes (curl, Chrome UA, no cookie) used to confirm/parametrise the REST search and GraphQL `jobDetails` / `jobSearchV7` surfaces.

## 4. Endpoints Discovered

| Method | Host / Path | Operation | Status | Content-Type | Auth |
|--------|-------------|-----------|--------|--------------|------|
| GET | `au.seek.com` `/api/jobsearch/v5/search` | — | 200 | application/json | public |
| GET | `www.seek.com.au` `/api/jobsearch/v5/search` | — (identical response) | 200 | application/json | public |
| POST | `au.seek.com` `/graphql` | `jobDetails(id)` | 200 | application/json | public |
| POST | `au.seek.com` `/graphql` | `jobDetailsPersonalised(id,zone,…)` | 200 | application/json | public (personalises if session) |
| POST | `au.seek.com` `/graphql` | `jobSearchV7(params)` / `JobCountV7` | 200 | application/json | public |
| POST | `au.seek.com` `/graphql` | `JobDetailsRecommendedJobs(jobDetailsId,zone,…)` | 200 | application/json | public |
| POST | `au.seek.com` `/graphql` | `GetMatchedQualities(jobDetailsId,locale)` | 200 | application/json | public |
| POST | `au.seek.com` `/graphql` | `viewer { searchSavedJobs(jobIds) searchAppliedJobs(jobIds) }` (`SearchSavedAndAppliedJobs`) | 200 | application/json | **cookie** |
| POST | `au.seek.com` `/graphql` | `viewer { savedJobs(first) emailAddress personalDetails }` | 200 | application/json | **cookie** |
| POST | `au.seek.com` `/graphql` | `viewer { salaryNudge(zone) }` (`GetSalaryNudge`) | 200 | application/json | **cookie** |
| GET | `au.seek.com` `/job/{id}` | HTML + embedded Apollo state | 200 | text/html | public |
| GET | `www.seek.com.au` `/job/{id}` | HTML | **403 (Cloudflare)** | text/html | public but gated |
| GET | `login.seek.com` `/time` | server time / session ping | 200 | — | — |

Telemetry (excluded from spec): `browser-intake-datadoghq.com`, `lcto.aips-sol.com`,
`seek-metrics-forwarder.cloud.seek.com.au`. Assets: `bx-branding-gateway.cloud.seek.com.au`
(logos), `image-service-cdn.seek.com.au`.

## 5. Traffic Analysis

- **Protocols:** `rest_json` (search, conf 0.95), `graphql` (details + account, conf 0.95),
  `ssr_embedded_data` (Next.js RSC pages, conf 0.8).
- **Auth signals:** same-origin session cookie on `.seek.com` set by `login.seek.com`; no
  `Authorization` header on any `/graphql` call; no CSRF token on read queries. `viewer`
  is the authenticated root field. Public surface needs nothing.
- **Parameter evidence:** REST search params confirmed by echo in the response
  `searchParams` object after a filtered probe: `siteKey, sourcesystem, keywords, where,
  page, pageSize, classification, subclassification, worktype, salaryrange, salarytype,
  daterange, sortmode, workarrangement, distance, locale`. `sortmode` values:
  `KeywordRelevance` (default), `ListedDate`. GraphQL `jobDetails($id: ID!)` and
  `jobSearchV7($params: JobSearchV7QueryInput!)` are the verified field signatures.
- **Protection signals:** legacy `www.seek.com.au/job/{id}` HTML is Cloudflare-gated (403);
  no gate on the JSON APIs or on `au.seek.com/job/{id}`.
- **Generation hints:** `standard_http`; primary host `https://au.seek.com`;
  `requires_browser_auth` for `me/*` commands only.

## 6. Coverage Analysis

Exercised: job search (REST), job details + recommended + matched-qualities (GraphQL),
saved searches, saved jobs, saved/applied status, salary nudge (GraphQL `viewer`).
Likely missed / not transcribed: full `jobDetailsPersonalised` and `GetMatchedQualities`
selection sets; company-profile GraphQL (`companyProfile(zone:)` field exists on
`jobDetails` and returns null for jobs without a profile — needs a job that has one);
career-feed / recommended-jobs home surface; profile/résumé endpoints (out of scope).
Brief mentions "company reviews" — `companyProfile.shouldDisplayReviews` is present but
the reviews payload itself was not captured.

## 7. Response Samples

**`GET /api/jobsearch/v5/search`** (200, `application/json`, ~32KB) — top-level:
```json
{"data":[{"advertiser":{"id":"63228293","description":"Arturia AI"},
  "bulletPoints":[],"classifications":[{"classification":{"id":"6281","description":"Information & Communication Technology"},
  "subclassification":{"id":"6290","description":"Engineering - Software"}}],
  "companyName":"Arturia AI","id":"94483533","isFeatured":false,
  "listingDate":"2026-09-08T03:09:03Z","listingDateDisplay":"2h ago",
  "locations":[{"label":"Macquarie Park, Sydney NSW","countryCode":"AU"}],
  "roleId":"software-engineer","salaryLabel":"","teaser":"Graduate Software Engineer — …",
  "title":"Graduate Software Engineer","workTypes":["Full time"],
  "workArrangements":{"data":[{"id":"1","label":{"text":"On-site"}}]}}],
 "totalCount":601,"sortModes":[{"isActive":true,"name":"Relevance","value":"KeywordRelevance"},
  {"isActive":false,"name":"Date","value":"ListedDate"}],
 "location":{"description":"Sydney NSW 2000","whereId":"16351","defaultDistanceKms":50},
 "searchParams":{...echoed...},"info":{"timeTaken":49}}
```

**`POST /graphql` `jobDetails(id)`** (200, `application/json`, ~4.4KB):
```json
{"data":{"jobDetails":{"job":{"id":"94483533","title":"Graduate Software Engineer",
  "phoneNumber":null,"isVerified":false,"abstract":"…","content":"<HTML 3456 chars>",
  "status":"Active","listedAt":{"label":"2h ago","dateTimeUtc":"2026-09-08T03:09:03.858Z"},
  "salary":null,"shareLink":"https://au.seek.com/job/94483533",
  "workTypes":{"label":"Full time"},
  "advertiser":{"id":"63228293","name":"Arturia AI","isVerified":true},
  "location":{"label":"Macquarie Park, Sydney NSW"},
  "classifications":[{"label":"Engineering - Software (Information & Communication Technology)"}],
  "products":{...}},
  "companyProfile":null,
  "companySearchUrl":"https://au.seek.com/Arturia-AI-jobs/at-this-company"}}}
```

**`POST /graphql` `SearchSavedAndAppliedJobs`** query body (from interceptor):
```graphql
query SearchSavedAndAppliedJobs($jobIds: [String!]!) {
  viewer { searchSavedJobs(jobIds: $jobIds) { jobId } searchAppliedJobs(jobIds: $jobIds) { jobId } } }
```
`ApacSavedSearch` fields (Apollo cache): `id`, `name(languageCode)`, `query(languageCode)`,
`countryCode`, `createdDate`, `newToYouCountLabel`, `subscribeToNewJobs`.

## 8. Rate Limiting Events

None. No 429s. ~12 API calls total across the session at ~0.5 req/s.

## 9. Authentication Context

- Authenticated session **used** (user confirmed logged-in Chrome; `AUTH_SESSION_AVAILABLE=true`).
- Transfer method: chrome-MCP (no cookie transfer — capture ran inside the user's own tab).
- Auth-only surface: `viewer { … }` — saved searches, saved jobs, applied jobs, saved/applied
  status lookups, salary nudge, personal details, email address.
- Auth scheme: **same-origin session cookie on `.seek.com`** (set by `login.seek.com`).
  No `Authorization` header, no visible CSRF token on read queries.
- Cookie-replay validation (Step 2d): **in-browser confirmed** (viewer queries returned the
  user's real data); **out-of-browser replay not tested** — session cookie values were
  deliberately not extracted. The generated CLI emits `auth login --chrome` (press-auth
  companion); first login / Phase 5 live smoke validates replay.
- Session state excluded from manuscript archiving (no cookie values written anywhere).

## 10. Bundle Extraction

Not run — the interactive sniff + direct probes covered the shippable surface and the
GraphQL schema is enforced server-side (bundle paths would not give valid field selections).
