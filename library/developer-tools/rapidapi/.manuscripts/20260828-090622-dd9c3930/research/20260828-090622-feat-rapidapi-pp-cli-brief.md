# Research Brief: rapidapi-pp-cli

**Run:** 20260828-090622-dd9c3930 | **Date:** 2026-08-28
**Target:** RapidAPI Hub website (https://rapidapi.com/hub) — the marketplace itself
**Mode:** website → browser-sniff primary (chrome-devtools MCP, live logged-in session)

## What the website is

RapidAPI Hub is the world's largest API marketplace: ~79k public APIs, 50+ categories,
7.6M users, 250B+ API calls served. It's a Next.js SPA whose entire data layer is a
**GraphQL BFF at `POST https://rapidapi.com/gateway/graphql`**, fronted by:
- `GET /gateway/csrf` → CSRF token bootstrap
- cookie session auth + `x-csrf-token` header
- Next.js RSC prefetches for SSR pages

## Discovery evidence

- 225 unique GraphQL operations extracted from the platform JS bundle (chunk 6944).
- **14 core operations live-validated** against the real gateway with real response data:
  `searchApis`, `getApiBySlugAndOwner`, `getCategoriesByCtx`, `GetTopCategories`,
  `GetCollectionsCollapsed`, `getCollectionBySlug`, `getUserProfile`, `getHubMetrics`,
  `getApiVersionPlayground`, `activeUser`, `getUserSavedApis`, `getApiSubscriptions`,
  `getNotifications`, `getWorkspaceData`.
- Exact input shapes reverse-engineered and confirmed via the server's own validation errors:
  `SearchApiWhereInput`, `SearchApiOrderByInput` (`sortingFields`), relay `PaginationInput`
  (`first/after`), `MetricsInput` (`fromDate/toDate`), `GetSubscriptionInput`.
- Auth: cookie session; CSRF header required for POSTs; `x-rapid-role: admin` for admin ops.

## Competitive landscape (what exists)

| Tool | Surface | Gaps |
|---|---|---|
| `rapidapi` npm SDK | Calls provider APIs through RapidAPI | Not the hub itself; needs keys per API |
| Postman/Insomnia collections | Manual exploration | No CLI automation |
| `rcli`/`rapidapi-cli` (stale) | Hub search/detail via old REST | Dead; predates GraphQL BFF |
| Website + browser | Full surface | No scripting, no offline use |

**No existing CLI wraps the hub website's own GraphQL BFF.** This is a greenfield niche.

## Proposed CLI surface (feature manifest)

### Public marketplace (no login)
1. `rapidapi search <term> [--category] [--tags] [--limit] [--sort] [--json]` — full search with facets, scores, latency, service level
2. `rapidapi categories [--limit]` — top + all categories with weights/thumbnails
3. `rapidapi collections [--limit]` — curated collections
4. `rapidapi collection show <slug>` — collection detail with its APIs
5. `rapidapi api show <owner>/<slug>` — full API detail: endpoints, versions, billing plans, rating, owner
6. `rapidapi user show <username>` — user profile + published APIs
7. `rapidapi metrics` — hub-wide stats (public APIs, users, traffic)

### Account (needs login)
8. `rapidapi whoami` — active user + orgs + tenant
9. `rapidapi saved list` / `rapidapi saved add <apiId>` / `rapidapi saved remove <apiId>` — favorites
10. `rapidapi subscriptions list` — my API subscriptions with plans/status
11. `rapidapi notifications [--limit]` — unread notifications
12. `rapidapi workspace [--from --to]` — owned + subscribed APIs with metrics
13. `rapidapi auth login` / `auth logout` / `auth status` — cookie+CSRF session management (browser-cookie import)

### Offline/agent-native (novel)
14. Local SQLite store: cache search results, API details, categories for offline query
15. `--json` everywhere + stable exit codes for scripting
16. CSV/table output modes

## Runtime

- **Transport:** standard HTTP to `rapidapi.com` (proven reachable via curl + browser; Cloudflare CDN but no hard challenge in-session).
- **Auth:** `x-csrf-token` bootstrap + session cookies (importable via `auth login --chrome`), `rapid-client` header.
- **Client pattern:** GraphQL BFF — each operation is a `POST /gateway/graphql` with `operationName`/`query`/`variables`.
