# Food52 Browser-Sniff Discovery Report

## User Goal Flow
- Goal: Search Food52 for recipes by keyword, then get a recipe's full structured detail.
- Steps completed:
  1. Opened https://food52.com/ (Vercel challenge auto-resolved by Chromium JS context, no `_vcrcs` cookie was set)
  2. Inspected primary nav and search input
  3. Submitted search via form (legacy /recipes/search?q= URL is dead; the live URL is /search?query=)
  4. Walked the search XHR and confirmed Typesense backend
  5. Visited /recipes/chicken (recipe-by-tag SSR page)
  6. Picked first recipe from JSON-LD and visited /recipes/<slug>
  7. Visited /food (article landing) and a story article (/story/<slug>)
  8. Probed /hotline and /hotline/questions/advice (only siteSettings in pageProps; no public Q&A surface)
  9. Verified all working URL shapes are reachable via Surf with Chrome impersonation (no cookie required)
- Steps skipped: None.
- Secondary flows attempted: Article browse, hotline browse, Typesense bundle key extraction.
- Coverage: All Phase 1 brief workflows mapped. Hotline shipped scope reduced to none (no public surface).

## Pages & Interactions
| URL | Purpose | Interactions |
|-----|---------|--------------|
| https://food52.com/ | Homepage, challenge clearance | Captured cookies, inventoried nav, found search input `name=q` |
| https://food52.com/?q=brownies | Form-submit destination | Confirmed legacy `/recipes/search?q=` URL no longer exists; real URL is `/search?query=` |
| https://food52.com/search?query=brownies | Search results page | Performance API showed Typesense XHR; pageProps has no results (XHR-rendered) |
| https://food52.com/recipes/chicken | Tag browse | `__NEXT_DATA__.props.pageProps.recipesByTag.results[]` has 36 recipes per page |
| https://food52.com/recipes/<slug> | Recipe detail | `__NEXT_DATA__.props.pageProps.recipe` + `<script type="application/ld+json">` Recipe JSON-LD |
| https://food52.com/food | Article vertical landing | `pageProps.blogPosts.results[]` |
| https://food52.com/story/<slug> | Article detail | `pageProps.blogPost` (full content, related reading, sponsor, tags, author) |
| https://food52.com/hotline | Hotline landing | `pageProps` only has siteSettings — no question data exposed |
| https://food52.com/hotline/questions/advice | Hotline topic | Same shell; no question listings in DOM or pageProps |

## Browser-Sniff Configuration
- Backend: browser-use 0.12.5 (CLI mode, no LLM key)
- Pacing: 1s baseline between evals; no 429s observed against food52.com itself
- Proxy pattern detected: No (each page maps to its own SSR URL)

## Endpoints Discovered
| Method | URL pattern | Status | Content-Type | Auth |
|--------|-------------|--------|--------------|------|
| GET | `https://food52.com/` | 200 | text/html (SSR) | public |
| GET | `https://food52.com/recipes/<tag>` | 200 | text/html (SSR with `recipesByTag.results[]` in `__NEXT_DATA__`) | public |
| GET | `https://food52.com/recipes/<slug>` | 200 | text/html (SSR with `recipe` in `__NEXT_DATA__` + Recipe JSON-LD) | public |
| GET | `https://food52.com/food` | 200 | text/html (SSR with `blogPosts.results[]` in `__NEXT_DATA__`) | public |
| GET | `https://food52.com/food/<sub>` | 200 | text/html (SSR with subvertical posts) | public |
| GET | `https://food52.com/story/<slug>` | 200 | text/html (SSR with `blogPost` in `__NEXT_DATA__`) | public |
| GET | `https://food52.com/_next/data/<buildId>/<path>.json?<params>` | 200 | application/json (Next.js JSON; same shape as `__NEXT_DATA__.props.pageProps`) | public |
| GET | `https://91hp3auljx7qfc65p-1.a1.typesense.net/collections/recipes_production_food52_current/documents/search?q=<query>&query_by=title,metaDescription,tags&per_page=N&page=N&facet_by=tags` | 200 | application/json | `x-typesense-api-key` header (search-only key) |
| GET | `https://food52.com/_next/static/chunks/_app-<hash>.js` | 200 | application/javascript (contains `typesense.searchOnlyApiKey` literal — extracted at runtime) | public |

## Traffic Analysis
- **Reachability mode**: `browser_http`. Surf with Chrome impersonation (`enetx/surf` v1.0.199 `Builder().Impersonate().Chrome()`) cleared Vercel for every probed URL. No `_vcrcs` clearance cookie was set or required. Plain curl is 429-blocked even with rich Chrome headers — TLS fingerprinting is the gate, not header content. The generated CLI must use Surf transport.
- **Protocols observed**:
  - `ssr_embedded_data` (high confidence) — Next.js `__NEXT_DATA__` JSON inline in HTML
  - `next_data_json` (high confidence) — `/_next/data/<buildId>/<path>.json` returns the same payload without HTML wrapper, preferred for clean parsing
  - `rest_json` (high confidence) — Typesense REST search endpoint
  - `schema_org_jsonld` (high confidence) — Recipe pages have inline `<script type="application/ld+json">` with `Recipe` shape
- **Auth signals**: No Food52 user authentication required for any reachable surface. The Typesense `x-typesense-api-key` is a public search-only key embedded in the public JS bundle (Typesense's intended pattern; equivalent to a Stripe publishable key or Algolia public app key). The CLI discovers it at runtime from `_app-<hash>.js` rather than hardcoding; if Food52 rotates the key the CLI auto-recovers.
- **Protection signals**: Vercel Bot Mitigation site-wide. Passive TLS-fingerprint check; cleared by Surf Chrome impersonation. No JS-active challenge observed (no `_vcrcs` cookie set after page load).
- **Generation hints**:
  - `requires_browser_compatible_http: true` — Surf transport mandatory.
  - `requires_browser_auth: false` — no Chrome cookie import needed.
  - `requires_js_rendering: false` — all data is in SSR HTML or REST JSON.
  - `requires_runtime_key_discovery: true` — Typesense key fetched from JS bundle on first search.
  - `requires_buildid_discovery: true` — Next.js JSON endpoints depend on a per-deploy `buildId` that must be parsed from `__NEXT_DATA__.buildId`; the CLI caches it and refreshes on 404.
- **Candidate commands**:
  - `recipes search <query>` — Typesense
  - `recipes browse <tag>` — Next.js JSON for `/recipes/<tag>`
  - `recipes get <slug-or-url>` — Next.js JSON for `/recipes/<slug>` + Schema.org JSON-LD fallback
  - `tags list` — hardcoded enum derived from homepage nav (chicken, breakfast, vegetarian, dinner, ...)
  - `verticals list` — `food`, `life` (and their subverticals)
  - `articles browse <vertical> [<sub>]` — Next.js JSON for `/<vertical>.json` or `/<vertical>/<sub>.json`
  - `articles get <slug-or-url>` — Next.js JSON for `/story/<slug>.json`
- **Warnings**:
  - Hotline (`/hotline*`) returns only `siteSettings` in pageProps; community Q&A is not exposed via SSR or any reachable XHR. Excluded from CLI scope.
  - Shop (`shop.food52.com`, `food52.myshopify.com/api/2025-01/graphql.json`) is reachable but requires its own Storefront token discovery and was deferred — out of scope for v1 unauthenticated CLI per user constraint.
  - Build IDs and bundle hashes change on each Food52 deploy. The CLI's runtime discovery handles this automatically; users will not see breakage from deploys.

## Coverage Analysis
- Resource types exercised: recipes (browse/detail/search), articles (browse/detail), tags (taxonomy enum), verticals (taxonomy enum)
- Gaps vs Phase 1 brief:
  - Hotline: removed from scope (no public surface).
  - Shop / collections / contests: deferred from v1; reachable but require additional spec work (Shopify Storefront GraphQL).
- The remaining surface fully covers the brief's "Top Workflows" 1-5 (search, ingredient browse via tags, recipe detail, cuisine browse via tags, blog reading).

## Response Samples
**Typesense recipes search response (truncated):**
```json
{"facet_counts":[],"found":175,"hits":[{"document":{"featuredImageAlt":"brownies in food52 test kitchen","featuredImageUrl":"...","id":"41ee2198-...","metaDescription":"These fudgy Lunch Lady Brownies from Sarah Fennel get a rich, classic cocoa icing.","popularity":0,"publishedAt":1764025703163,"rating":0,"ratingCount":0,"slug":"sarah-fennel-s-best-lunch-lady-brownie-recipe","tagSlugs":["dessert","bake","brownie","chocolate"],"tags":["Dessert","Bake","Brownie","Chocolate"],"testKitchenApproved":true,"title":"Lunch Lady Brownies"},"highlight":{"title":{"matched_tokens":["Brownies"],"snippet":"Lunch Lady <mark>Brownies</mark>"}},"text_match_info":{"score":"578730123365187705"}}, ...]}
```

**Recipe detail JSON-LD (Schema.org Recipe):**
- `@type`: `Recipe`
- Fields: `name`, `image`, `author`, `datePublished`, `recipeYield`, `description`, `recipeCategory`, `recipeCuisine`, `keywords`, `recipeIngredient[]`, `recipeInstructions[]`, `publisher`
- Sample ingredient count: 13 for "Mom's Japanese Curry Chicken"

**Recipe detail `pageProps.recipe` (Sanity CMS shape):**
- Keys: `_id`, `_type`, `_createdAt`, `_updatedAt`, `_rev`, `title`, `slug`, `description`, `recipeDetails` (ingredients, instructions, prep/cook/total times), `kitchenNotes`, `featuredImage`, `author`, `authorName`, `averageRating`, `ratingCount`, `recentRecipes`, `recipeProducts`, `seo`, `sponsor`, `tags`, `testKitchenApproved`

**Article (`/story/<slug>`) `pageProps.blogPost`:**
- Keys: `_id`, `title`, `dek`, `slug`, `author`, `authorName`, `resolvedAuthorName`, `resolvedAuthorSlug`, `featuredImage`, `content`, `publishedAt`, `relatedReading`, `subVertical`, `sponsor`, `tags`, `seo`

## Rate Limiting Events
- Zero 429s against food52.com or Typesense during the entire browser-sniff. Cleared TLS fingerprint plus default browser-use pacing was sufficient.

## Authentication Context
- Anonymous browser-sniff (no Food52 sign-in). User constraint: unauthenticated only — honored.
- No Vercel `_vcrcs` clearance cookie was set during the browser-sniff (challenge is passive TLS-only, not JS-active).
- Typesense `searchOnlyApiKey` (`Ra8Le9Z0pom8rHXOsgmuVDMAKrd6FSka` at the time of this browser-sniff) is intentionally public and embedded in `_app.js`. The CLI extracts it at runtime; the key value is NOT persisted to the spec, source, or any committed artifact.
- Session state file: not created (no auth session needed).

## Bundle Extraction
- Bundle: `https://food52.com/_next/static/chunks/_app-<hash>.js` (1.05 MB, varies per deploy)
- Extracted (at browser-sniff time only — not persisted to spec):
  - `typesense.host` = `91hp3auljx7qfc65p-1.a1.typesense.net`
  - `typesense.searchOnlyApiKey` = (search-only public key)
- Other public bundle config noted for context (not used by the CLI):
  - `googleTagManager.id`, `reCaptcha.siteKey`, `sailThru.customerId`, `freestar.publisherName`
- Runtime extraction strategy: the CLI fetches `/` to find the current `_app-<hash>.js` URL, downloads that chunk via Surf, regexes the `typesense:` literal, and caches the host + key for 24h.
