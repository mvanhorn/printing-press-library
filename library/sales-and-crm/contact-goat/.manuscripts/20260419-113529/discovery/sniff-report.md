# Happenstance Sniff Report

## User Goal Flow
- **Primary sniff goal**: Read the user's network - list recent searches, get search details, list friends, list research dossiers, see feed
- **Steps completed**:
  1. Open happenstance.ai (logged-in headed browser session)
  2. Install fetch + XHR interceptors
  3. Click existing search result (SPA navigation, interceptors preserved)
  4. Probe response shapes for /api/user, /api/user/limits, /api/dynamo/recent, /api/research/recent, /api/dynamo?requestId, /api/friends/list, /api/feed
  5. Extract cookies from browser session
  6. Validate cookie replay via curl outside browser (HTTP 200 WITH cookies, HTTP 204 WITHOUT)
- **Steps skipped**: Creating a new search (would burn a credit - decided to rely on published developer.happenstance.ai OpenAPI spec for write endpoints)
- **Secondary flows attempted**: None - read-side coverage was sufficient from existing-search navigation
- **Coverage**: 6/6 planned probe steps completed

## Pages & Interactions
1. happenstance.ai/search - SPA root, logged-in view with nav (/feed, /connectors, /friends, /groups, /search, /research, /integrations)
2. Clicked /search/31b2fed2-a9d3-4397-8faf-2bbb58828385 - existing search. Triggered full read-side API fan-out.
3. In-page fetch probes for each discovered /api/* endpoint

## Sniff Configuration
- **Backend**: browser-use 0.14.x (headed mode, session name "contact-goat-happenstance")
- **Pacing**: 1s initial delay, no 429s encountered, effective rate ~2 req/s
- **Proxy pattern detection**: NOT proxy-envelope. Distinct /api/<resource> paths, not a single proxy endpoint.
- **GraphQL BFF detection**: NOT a GraphQL BFF. Normal REST.

## Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| GET | /api/user | 200 | application/json | auth-required |
| GET | /api/user/limits | 200 | application/json | auth-required |
| GET | /api/dynamo/recent | 200 | application/json | auth-required |
| GET | /api/dynamo?requestId={uuid} | 200 | application/json | auth-required |
| GET | /api/search/{uuid}/suggested-posts | 200 | application/json | auth-required |
| GET | /api/research/recent | 200 | application/json | auth-required |
| GET | /api/research/history?page&limit | 200 | application/json | auth-required |
| GET | /api/friends/list | 200 | application/json | auth-required |
| GET | /api/feed?unseen&limit | 200 | application/json | auth-required |
| GET | /api/notifications?page&limit | 200 | application/json | auth-required |
| GET | /api/clerk/referrer?referrerId={uuid} | 200 | application/json | auth-required |
| GET | /api/uploads/status/user | 200 | application/json | auth-required |
| POST | clerk.happenstance.ai/v1/client/sessions/{id}/tokens | 200 | application/json | auth-required (refresh) |

All /api/* endpoints sit on same-origin happenstance.ai. Clerk token refresh is on the clerk.happenstance.ai subdomain.

## Coverage Analysis
Exercised: user, limits, searches (read), research (read), friends, feed, notifications, uploads-status, user-by-uuid lookup.

Likely missed (would need an interactive action to capture):
- POST /api/dynamo (create search) - not triggered, would burn a credit
- POST /api/research (create research) - same
- POST /api/feed/posts (create post) - not triggered
- POST /api/feed/{id}/like, /api/feed/{id}/comment - not triggered
- Groups endpoints (/api/groups/...) - not exercised, user had none
- Integrations endpoints (/api/integrations/...) - not exercised

For create/mutate endpoints, reference the public developer.happenstance.ai/openapi.json spec which documents POST /v1/search, POST /v1/research, POST /v1/search/{id}/find-more with Bearer auth. The web-app paths likely mirror these under /api/. A follow-up sniff with a fresh search-create would confirm.

## Response Samples
**/api/user** (truncated):
```json
{"id":"user_xxx","uuid":"<uuid>","email":"<email>","linkedinUrl":"<url>","fullName":"<name>","planId":"free","settings":{...},"timezone":"<tz>"}
```

**/api/user/limits**:
```json
{"searchesRemaining":14,"searchesRenewalDate":"2026-05-01T00:00:00.000Z","dailyPostsRemaining":3}
```

**/api/dynamo/recent** (array):
```json
[{"request_id":"<uuid>","request_subject":"AI engineers Bellevue","timestamp":"2026-04-19T..."}]
```

**/api/friends/list** (array):
```json
[{"id":"<uuid>","name":"<name>","imageUrl":"<url>","connections":328583}]
```

**/api/feed?limit=5** (truncated):
```json
{"items":[{"id":"<uuid>","item_type":"user_post","created_at":"...","data":{"type":"user_post","post_id":"...","author_name":"...","content":"...","is_friend":true,"affiliation":{"type":"friend"}}}]}
```

**/api/dynamo?requestId=** (truncated - rich nested structure with logs array):
```json
[{"user_id":"...","followup":{"type":"results_limit"},"include_my_connections":true,"request_content":[{"type":"p","children":[{"text":"<query>"}]}],"logs":[{"type":"SEARCH_SCOPE","message":{"total_search_scope":434933,"user":{"total":3360,"by_source":{"LinkedIn":3360}},"friends":[...]}}],...}]
```

## Rate Limiting Events
None encountered. ~15 requests over ~90 seconds at ~1s pacing. No 429s. Clerk JWT rotated once during the sniff (normal 60s TTL refresh).

## Authentication Context
- **Session transferred**: Yes, via headed browser-use login (user logged in fresh).
- **Auth-only endpoints**: ALL 12 discovered /api/* endpoints require auth. /api/user returns HTTP 204 + `x-clerk-auth-status: signed-out` without cookies.
- **Auth header scheme discovered**: NONE. No Authorization header on any API request. Auth is purely cookie-based.
- **Auth cookie (primary)**: `__session` (Clerk JWT, RS256, ~60s TTL)
- **Supporting cookies**: `__session_K8Qez0yT` (instance-suffixed variant), `__refresh_K8Qez0yT` (refresh token), `__client_uat` and `__client_uat_K8Qez0yT` (client update timestamps), `clerk_active_context` (active session ID).
- **Token refresh**: `POST clerk.happenstance.ai/v1/client/sessions/{sessionId}/tokens` rotates the `__session` JWT every ~60s. The CLI's `auth login --chrome` should either (a) extract cookies fresh on each invocation from Chrome's cookie jar, or (b) implement the refresh-token dance.
- **Cookie replay verdict**: PASS. With cookies -> HTTP 200 + JSON. Without -> HTTP 204 + `x-clerk-auth-status: signed-out`.
- **Session state exclusion**: session-state.json will be removed before Phase 5.6 archive per secret-protection.md.

## Auth spec (for generator)
```yaml
auth:
  type: cookie
  cookie_domain: happenstance.ai
  cookies:
    - __session
    - __session_K8Qez0yT
    - __refresh_K8Qez0yT
    - __client_uat
    - __client_uat_K8Qez0yT
    - clerk_active_context
```

The CLI's `auth login --chrome` should extract these cookies from Chrome's cookie jar at command invocation time. If `__session` is expired (decode JWT exp claim), either (a) prompt the user to open happenstance.ai in Chrome to refresh, or (b) POST to clerk.happenstance.ai/v1/client/sessions/{id}/tokens with the `__client` cookie to refresh.

## Bundle Extraction
Not run. Interactive sniff yielded 12 endpoints covering all primary reads. Bundle extraction is a supplementary step for admin/edge endpoints; not needed for v1.

## Notes for Phase 2 generate
- Do NOT run `printing-press sniff` on this capture - the spec is already authored in happenstance-sniff-spec.yaml.
- Pass `--spec $RESEARCH_DIR/happenstance-sniff-spec.yaml` in Phase 2.
- Client pattern is NOT proxy-envelope - use default.
- Auth type is `cookie` with composed cookies - the generator's cookie-auth template handles `auth login --chrome`.
