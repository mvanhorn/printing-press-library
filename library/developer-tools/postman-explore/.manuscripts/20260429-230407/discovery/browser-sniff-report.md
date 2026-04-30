# Postman Explore — Browser-Sniff Discovery Report

## User Goal Flow

**Primary goal:** Search and discover community-contributed Postman Collections by name (e.g., "stripe") and by category, including browsing the homepage feeds for each entity type.

**Steps completed (re-sniffed via direct proxy probing after browser automation was blocked):**

1. Opened postman.com/explore with browser-use → blocked by Cloudflare Turnstile (`Just a moment...` page; auto-solve did not pass after 30 seconds)
2. Tried agent-browser as fallback → also blocked by Turnstile
3. Reachability probe (`printing-press probe-reachability`) confirmed `mode: browser_http` — Surf with Chrome TLS fingerprint clears the protection
4. Direct `POST /_api/ws/proxy` with Chrome User-Agent + Origin header → HTTP 200, proxy API is NOT challenged. Discovery pivoted to direct API probing
5. Probed `publishing` service — confirmed `entityType ∈ {collection, workspace, api, flow}` browse, detail, and counts endpoints
6. Probed `search` service — confirmed `/search-all` works with `queryText` alone and with dotted `queryIndices`
7. Probed `categories` endpoints — `/v2/api/category` and `/v2/api/category/{slug}` work
8. Probed `team` endpoints — `/v1/api/team?publicHandle=` returns workspace array; `/v1/api/team/{numericId}` returns small profile
9. Probed sort enum on browse — only `popular` works; `recent`, `new`, `week`, `alltime` return HTTP 400 "Invalid sort type"; `featured` consistently times out
10. Probed `minMonthlyForkCount` query parameter — works, useful for trending discovery
11. Probed alternative service names (api-network, category, catalog, network-entity, homepage, networkentity) — all return `invalidServiceError`. Only `publishing` and `search` are valid services on the proxy

**Steps skipped:** Authenticated flow (out of scope; user explicitly scoped to anonymous community discovery).

**Coverage:** 8 of 8 logical endpoints exercised against live network with HTTP 200 responses.

## Pages & Interactions

Pages were not browser-loaded (blocked by Turnstile). Equivalent direct API probes hit the same endpoints the website's XHR layer uses:

| Action | Endpoint exercised | Outcome |
|--------|-------------------|---------|
| "Browse popular collections" | `GET /v1/api/networkentity?entityType=collection&sort=popular` | 200, 5 entities |
| "Browse popular workspaces" | `GET /v1/api/networkentity?entityType=workspace&sort=popular` | 200 |
| "Browse popular APIs" | `GET /v1/api/networkentity?entityType=api&sort=popular` | 200 |
| "Browse popular flows" | `GET /v1/api/networkentity?entityType=flow&sort=popular` | 200 |
| "Filter by Developer Productivity category" | `GET /v1/api/networkentity?entityType=collection&categoryId=4` | 200 |
| "Trending: high monthly forks" | `GET /v1/api/networkentity?...&minMonthlyForkCount=1000` | 200 |
| "Open collection detail" | `GET /v1/api/networkentity/10289` | 200 (Salesforce Platform APIs) |
| "Network counts" | `GET /v1/api/networkentity/count` | 200 (725k+ collections) |
| "Category list" | `GET /v2/api/category` | 200, 12 categories |
| "Category detail" | `GET /v2/api/category/artificial-intelligence` | 200 |
| "Team profile (numeric)" | `GET /v1/api/team/1292807` | 200 (Salesforce Developers) |
| "Team workspaces (handle)" | `GET /v1/api/team?publicHandle=stripedev` | 200, 100 workspaces |
| "Search 'stripe' across all types" | `POST /search-all body={queryText:'stripe'}` | 200 |
| "Search 'webhook' narrowed to collections" | `POST /search-all body={queryText:'webhook',queryIndices:['runtime.collection']}` | 200 |
| "Search multi-index" | `POST /search-all body={queryIndices:['runtime.collection','collaboration.workspace','apinetwork.team','flow.flow']}` | 200, all types returned |

## Browser-Sniff Configuration

- **Backend used:** Direct HTTP probing via curl with Chrome User-Agent (browser-use 0.12.5 and agent-browser 0.25.3 both blocked by Cloudflare Turnstile on the HTML page)
- **Pacing:** ~1s between requests, no rate limiting encountered
- **Effective rate:** ~0.7 req/s (24 probe requests over ~35 seconds)
- **Proxy pattern detection:** `proxy-envelope` confirmed. Single proxy URL `https://www.postman.com/_api/ws/proxy` with body envelope `{service, method, path, body?}`. Two services: `publishing` and `search`.

## Endpoints Discovered

| Method | Path | Service | Status | Content-Type | Notes |
|--------|------|---------|--------|--------------|-------|
| GET | /v1/api/networkentity | publishing | 200 | application/json | Browse list. `entityType` required. `sort=popular` only working sort. |
| GET | /v1/api/networkentity/{id} | publishing | 200 | application/json | Numeric `id`, NOT `entityId` UUID |
| GET | /v1/api/networkentity/count | publishing | 200 | application/json | Network-wide entity counts |
| GET | /v2/api/category | publishing | 200 | application/json | 12 categories, sorted by spotlight |
| GET | /v2/api/category/{slug} | publishing | 200 | application/json | Full category with featured entities |
| GET | /v1/api/team | publishing | 200 | application/json | Requires `publicHandle` query; returns workspaces |
| GET | /v1/api/team/{id} | publishing | 200 | application/json | Small profile by numeric id |
| POST | /search-all | search | 200 | application/json | Body envelope; data keyed by entity type |

## Traffic Analysis

- **Protocols observed:** `rpc_envelope` (confidence 0.95), `rest_json` (confidence 0.6 — modeled in spec but actual transport is RPC)
- **Auth signals:** No auth required (`type: none`, confidence 1.0). Verified by anonymous probes returning HTTP 200 across all endpoints
- **Protection signals:** Cloudflare HTML challenge (`cf-mitigated: challenge`, Turnstile script, `__cf_bm` cookie). HTML pages gate behind Turnstile; the proxy API does not.
- **Generation hints:** `client_pattern:proxy-envelope`, `uses_browser_http_transport`, `no_auth_required`, `skip_clearance_cookie`

## Coverage Analysis

- **Resource types exercised:** collections, workspaces, APIs, flows, teams, categories, search (all five entity types from the brief)
- **Likely missed:** None — every endpoint on the spec was exercised. The proxy explicitly rejects every other service name we tried, and the publishing service rejected every path that wasn't already in the spec
- **Brief alignment:** Brief mentions five entity types — all are confirmed reachable. Brief mentions filtering by category and sorting — both confirmed (with correction: only `popular` sort works)

## Response Samples

Sample shapes captured under `$DISCOVERY_DIR/probe-*.json`. Highlights:

**Browse response (`/v1/api/networkentity`):**
```json
{
  "data": [{ "id": 10289, "entityId": "12721794-67cb9baa-...",
             "entityType": "collection", "name": "Salesforce Platform APIs",
             "summary": "...", "metrics": [...], "categories": [...],
             "redirectURL": "...", "redirectURLV2": "..." }],
  "meta": { "limit": 5, "offset": 0, "count": 5, "totalCount": 727369,
            "publisherInfo": { "team": [...] }, "model": "network",
            "action": "public" }
}
```

**Counts response (`/v1/api/networkentity/count`):**
```json
{
  "data": { "apiCount": 25667, "collectionCount": 727139, "flowCount": 6396,
            "notebookCount": 827, "workspaceCount": 316362, "teamCount": 169400 },
  "meta": { "model": "networkentity", "action": "count" }
}
```

**Search response (`/search-all`, no queryIndices):**
```json
{
  "data": {
    "collection": [{ "score": 464.20, "normalizedScore": 25.74,
                     "document": { "id":"...", "name":"...", "publisherHandle":"..." } }],
    "workspace": [...], "api": [...], "team": [...], "request": [...]
  },
  "meta": { "queryText": "stripe",
            "total": { "collection":296, "workspace":86, "api":4, "team":155, "flow":0, "request":10000 },
            "count": { "collection":3, "workspace":3, "api":3, "team":3, "request":3 } }
}
```

## Rate Limiting Events

None encountered. ~24 probes at ~0.7 req/s did not trigger any 429.

## Authentication Context

No authenticated session used. The user's stated scope is anonymous community discovery; no `auth login` path is generated. Verified by:
- All 17 successful probes returned HTTP 200 with no auth headers
- Network-wide entity counts and category lists are readable without any credentials
- Session state was not captured; nothing to strip from manuscripts

## Bundle Extraction

Skipped — direct API probing produced complete endpoint coverage in under 35 seconds. No need to grep JS bundles when the proxy itself is open and self-describing through error messages.

## Corrections vs. Existing Catalog Spec

The catalog Spec at `catalog/specs/postman-explore-spec.yaml` was largely correct (it was sniffed in March 2026). This re-sniff identified one important correction:

| Field | Old | New | Reason |
|-------|-----|-----|--------|
| `listNetworkEntities.sort` enum | `[popular, recent, featured, new, week, alltime]` | `[popular]` only | All other values return 400; `featured` times out |
| `searchAll.queryIndices` | optional, dotted form documented but not enforced | optional, MUST be dotted (`runtime.collection`, etc.) — simple names rejected | Verified via 400 errors |
| `searchAll.required` | `[queryText, size, queryIndices]` | `[queryText]` only | `queryIndices` and `size` are optional; defaults work |

A new query parameter `minMonthlyForkCount` was discovered on `listNetworkEntities` and added to the spec.
