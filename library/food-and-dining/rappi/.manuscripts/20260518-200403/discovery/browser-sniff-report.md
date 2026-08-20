# Rappi browser-sniff report

## Target
- URL: https://www.rappi.com.mx/
- Country: Mexico (MX)
- Tech: Next.js App Router, SSG (Static Site Generation), CloudFront-fronted, web-version 7.0.0
- Reachability: standard HTTP (probe-reachability returned `standard_http` with 0.95 confidence)

## Method
Direct HTTP probing with a real-Chrome User-Agent (`Mozilla/5.0 ... Chrome/131`) and `Accept-Language: es-MX,es;q=0.9` headers. No browser session, no proxies, no XHR replay. Tested from a US datacenter IP.

## Pages probed (all 200 OK except where noted)

| URL                                                       | Status | Notes                                              |
|-----------------------------------------------------------|--------|----------------------------------------------------|
| `/`                                                       | 200    | Homepage. CTAs link to category landing pages.     |
| `/restaurantes`                                           | 200    | National restaurants landing.                      |
| `/ciudad-de-mexico/restaurantes`                          | 200    | CDMX restaurant list, 30 restaurants/page.         |
| `/ciudad-de-mexico/restaurantes/category/hamburguesas`    | 200    | Category filter works.                             |
| `/ciudad-de-mexico/restaurantes/category/sushi`           | 200    | Category filter works.                             |
| `/guadalajara/restaurantes`                               | 200    | GDL restaurants.                                   |
| `/monterrey/restaurantes`                                 | 200    | MTY restaurants.                                   |
| `/restaurantes/10000295-el-farolito`                      | 200    | Restaurant detail page.                            |
| `/restaurantes/10000295-El-Farolito-ciudad-de-mexico`     | 429    | Rate limit hit on alternate-form URL (avoid this form). |
| `/tiendas/tipo/market`                                    | 200    | Supermarket list, 51 stores/page.                  |
| `/tiendas/tipo/farmatodo`                                 | 200    | Pharmacy list.                                     |
| `/tiendas/tipo/liquor`                                    | 200    | Liquor stores list.                                |
| `/catalogo/products/a-1`                                  | 200    | Alphabetized product catalog index.                |
| `/_next/data/<build>/index.json`                          | 404    | No `_next/data` exposure.                          |
| `/api/v3/stores`                                          | 503    | Public JSON API not reachable from datacenter IPs. |

## What's reachable via plain HTTP

**SSR HTML + structured JSON-LD.** Every reachable page embeds 3-4 `<script type="application/ld+json">` blocks with `schema.org` types: `BreadcrumbList`, `ItemList` (for list pages), `Restaurant`, `Brand`, `FAQPage`, `OrganizationOpeningHoursSpecification`. The richest data shape is on restaurant detail pages — full `Restaurant` schema with:
- `name`, `image`, `logo`, `url`, `@id`
- `servesCuisine` (array)
- `address.streetAddress`
- `openingHoursSpecification` (array of dayOfWeek + opens + closes)
- `geo.latitude` / `geo.longitude` (real coordinates)
- `aggregateRating` (ratingValue, ratingCount, bestRating, author)
- `potentialAction` (OrderAction)

**List pages have `ItemList` JSON-LD blocks** with 30-51 entries per page. Restaurant entries include name, image, url, `servesCuisine`, `aggregateRating.ratingValue`, `aggregateRating.reviewCount`. Store list entries have minimal shape (name + url + position).

**Restaurant detail pages include menu CATEGORY names** (Promociones, Entradas, Tacos, Costras, Volcanes, etc.) via `data-testid="menuOption"` markup, but **NOT menu items or prices**. Menu items load via post-hydration XHR, which is not reachable from datacenter IPs.

## What is NOT reachable

- **JSON XHR endpoints** — `/api/v3/...` returns 503 from datacenter IPs. Commercial scrapers (Apify parseforge, yasmany.casanova; Bright Data) all require residential proxies. Treat the JSON API as off-limits for v1.
- **Menu items with prices** — loaded via XHR after page hydration, behind the JSON API gate. v1 CLI exposes menu category names (server-rendered) but not item-level data.
- **Cart / orders / user account** — requires login. Out of v1 scope by design.
- **Mobile app endpoints** — signed requests (X-Signature pattern), fragile across releases. Explicit non-goal.

## Spec emitted
`research/rappi-browser-sniff-spec.yaml`. Six endpoints across four resources:
- `restaurants list-city`, `restaurants list-category`, `restaurants get` (city × category × detail)
- `stores list-by-type`, `stores get`
- `catalog browse-letter` (alphabetized product index)
- `promotions list`

All endpoints use `response_format: html` with `html_extract.mode` set to `links` for index pages (filters `<a>` tags by `link_prefixes`) or `page` for detail pages (extracts title, description, image, canonical URL, anchor list). Novel-feature commands in Phase 3 will layer JSON-LD parsing on top of these endpoints for richer ratings/cuisine/geo/hours data.

## Generator hints written

A `traffic-analysis.json` is written for the generator at `discovery/traffic-analysis.json` with `reachability.mode = standard_http`. The generator should NOT emit Chrome cookie import or browser-clearance code; plain stdlib HTTP works.

## Replayability assessment

**PASS for the SSR surface.** Every reachable page returns 200 OK from a plain stdlib HTTP client with a real-Chrome User-Agent and `Accept-Language: es-MX`. The CLI's runtime transport can use standard HTTP.

**FAIL for the JSON XHR surface.** Datacenter IPs return 503 on `api/v3/...`. A real residential session would be needed for that surface; it is excluded from v1.
