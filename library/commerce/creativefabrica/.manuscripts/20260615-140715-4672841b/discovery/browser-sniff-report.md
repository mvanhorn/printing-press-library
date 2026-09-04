# Creative Fabrica Browser-Sniff Discovery Report

## 1. User Goal Flow
- Goal: "Search the catalog, open an asset, view freebies, and view my library."
- Steps completed: homepage (logged-in, CF cleared) → search "watercolor flowers"
  → change sort (Newest) → apply Type facet → navigate to /my-account/.
- Coverage: public catalog surface fully mapped; authenticated /my-account/
  redirected to /login/ during a full-reload navigation (client-auth race) so
  the GraphQL personal-library ops were not captured this session.

## 2. Pages & Interactions
- `/` homepage (profile load, Cloudflare cleared by real headed Chrome)
- `/search/?query=watercolor%20flowers` (in-page search submit)
- sort change → "Newest first" (fired Algolia re-query — KEY DISCOVERY)
- Type facet "Graphics" (fired Algolia re-query)
- `/my-account/` → redirected to `/login/` (auth cookies present but route raced)

## 3. Browser-Sniff Configuration
- Backend: browser-use (headed, Chrome Default profile). Headless was blocked by
  Cloudflare ("Just a moment..."); headed real Chrome cleared it.
- probe-reachability: mode=browser_http (Surf-Chrome clears CF; stdlib 403).
- Pacing: ~1 req/s, no 429s.
- Proxy pattern: none. Two distinct backends observed (see below).

## 4. Backends Discovered
| Backend | Host | Purpose | Auth | Runtime transport |
|---|---|---|---|---|
| Algolia (primary) | `{appId}-dsn.algolia.net` `/1/indexes/*/queries` | Catalog search/browse/facets | public search key (query param `x-algolia-api-key`) + `x-algolia-application-id`; **referer-restricted** (requires Origin/Referer headers) | standard HTTPS — NO Cloudflare |
| GraphQL gateway | `graphql-gw.creativefabrica.com/query` | homepage modules, product detail, account/library/favorites | cookie session (`cfauth_uid`/`cfauth_sig`/`PHPSESSID`/`bp_ut_session`) | behind site; not captured (hydration-timing) |
| Content (secondary) | `{appId}-dsn.algolia.net` index `prod_items` | blog/tutorials/classes | separate public content key | standard HTTPS |
| Next.js SSR pages | `www.creativefabrica.com` | product/category/designer/daily-gifts HTML+JSON-LD | none (public) / cookie (account) | Surf/browser_http (CF) |

## 5. Algolia Surface (primary, fully verified via direct curl)
- Endpoint: `POST https://{appId}-dsn.algolia.net/1/indexes/*/queries`
- Credentials: public, search-only key (`NEXT_PUBLIC_ALGOLIA_API_KEY`) + public
  app id (`NEXT_PUBLIC_ALGOLIA_APP_ID`). Auto-discoverable from the site's JS
  bundle at runtime. **Key values are NOT stored in any run artifact** (session
  scratch only). Required: `Origin`/`Referer: https://www.creativefabrica.com`.
- Products index: `prod_Productsv2` (relevance); sort replica
  `prod_Productsv2_trending_newest` (newest). ~20,560,000 products.
- Request body: `{requests:[{indexName, query, page, hitsPerPage, facets[],
  filters, maxValuesPerFacet, userToken}]}`.
- Facets available: `type`, `category`, `designer.designerId`,
  `designer.designerName`, `hasPod`, `isFree`, `price`, `hasPromotions`,
  `extraSettings.level`, `extraSettings.source`.
- Type facet values: Graphics (13.1M), Community (6.5M), Fonts (270k),
  Crafts (210k), Embroidery (137k), Laser Cutting (129k), Bundles (59k),
  3D SVG (36k), 3D Printing (5.8k), Classes, Knitting.
- 724 categories (top: Graphics, Crafts, Illustrations, T-shirt Designs, Icons,
  Patterns, Transparent PNGs, Print Templates, Backgrounds, Logos, Freebies...).
- `isFree:true` → 89,387 free items. `hasPod:true` → 11.48M POD/commercial items.
- Hit fields: objectID, type, category[], tags[], name_en, description_en,
  image, url, designer{designerId,designerName,designerUrl}, price, regularPrice,
  isFree, hasPod, hasPromotions, outsideSubscription, isExclusive, date (unix),
  popularity, usedFonts[].

## 6. Coverage Analysis
- Exercised: product search, sort, facet filtering, free/POD filtering, facet
  enumeration (types, categories, designers).
- Missed (deferred to a follow-up amend): authenticated personal library /
  favorites / downloads (GraphQL gateway + cookie auth), curated daily-gifts
  set, content index (prod_items) detail.

## 7. Authentication Context
- Authenticated session used (user's Chrome Default profile). Auth cookies
  present: cfauth_uid, cfauth_sig, cfauth_utp, PHPSESSID, bp_ut_session,
  bp_user-role (Creative Fabrica + BuddyPress/WordPress session). Cookie/composed
  auth would power a future `auth login --chrome` for personal-library commands.
- Session state excluded from manuscript archiving (session scratch dir only).

## 8. Runtime Decision (v1)
- v1 printed CLI uses Algolia over **standard HTTPS** — no Cloudflare, no browser,
  fully replayable. Public search creds auto-discovered at runtime (env override).
- Product detail served from Algolia objectID metadata (rich hit fields).
- Personal-library / daily-gifts / content = documented future enhancements.
