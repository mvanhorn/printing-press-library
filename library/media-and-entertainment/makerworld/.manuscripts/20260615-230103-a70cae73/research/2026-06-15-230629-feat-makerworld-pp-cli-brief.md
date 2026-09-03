# MakerWorld CLI Brief

## API Identity
- **Domain:** MakerWorld — Bambu Lab's free 3D-model sharing platform (Sept 2023). Competitor to Printables / Thingiverse / Cults3D, tightly integrated with Bambu Lab printers & Bambu Studio/Handy.
- **Users:** 3D-printing hobbyists & designers who browse/download print-ready models ("designs"), publish their own, enter design contests, run crowdfunding, and earn redeemable points.
- **Data profile:** Read-heavy catalog. Core entities: **designs** (models) → **instances** (print configs) → **plates/3MF files**; **designers** (creators); **categories/tags**; **comments & ratings**; **remixes/related**; **collections/favorites**; **contests**; **points/boosts**.

## Surface & Hosts (CRITICAL — ground-truth from live probes)
Two interchangeable hosts serve the SAME `/v1/<service>/...` JSON API:
- **`https://api.bambulab.com/v1`** — the app API. **No Cloudflare, public reads need NO auth** (confirmed via plain curl). **← CLI base_url.**
- **`https://makerworld.com/api/v1`** — the web API. Same data; HTML pages are Cloudflare-gated (Surf clears, probe `mode: browser_http`) but the `/api/` JSON path answered a plain browser-UA GET.
- Auth (Bearer Bambu Cloud JWT, ~90-day TTL) is required only for **downloads, favorites, likes, account/messages**. Sent only to `api.bambulab.com`, never to CDN/S3.

### Confirmed ANONYMOUS endpoints (HTTP 200, no auth, no Cloudflare)
| Service | Endpoint | Returns |
|---|---|---|
| search | `GET /search-service/homepage/nav` | category list (navKey enum) |
| search | `GET /search-service/select/design/nav?navKey=&offset=&limit=` | browse Trending / category (hits[], total) |
| search | `GET /search-service/design/{id}/relate` | related designs (total 353 on sample) |
| search | `GET /search-service/recommand/youlike` | "recommended for you" (note API typo `recommand`) |
| design | `GET /design-service/design/{designId}?trafficSource=browse` | full detail: title, summary, coverUrl, designCreator, instances[], counts |
| design | `GET /design-service/design/{id}/remixed` | remixes of a design |
| comment | `GET /comment-service/commentandrating?designId=&offset=&limit=&type=0&sort=0` | comments + star ratings (total 69 on sample) |

### Auth-required endpoints (Bearer JWT)
`GET /design-service/instance/{instanceId}/f3mf?type=download` (3MF download), `POST /design-service/design/{id}/like`, `GET /design-service/favorites/designs/{userId}`, `GET /design-service/my/design/like`, `GET /design-service/my/favorites/listlite`, `GET /user-service/my/messages`, `GET /point-service/boost/boostdesign`.

### navKey category enum (from live `homepage/nav`)
`Following`, `Foryou`, `Trending`, `category_100` Art, `category_200` Fashion, `category_300` Hobby & DIY, `category_400` Household, `category_500` Education, `category_600` Miniatures, `category_700` Tools, `category_800` Toys & Games, `category_900` 3D Printer, `category_1000` Props & Cosplays, `category_2000` Generative 3D, `LaserCut` Laser & Cut.

## Reachability Risk
- **Low for the read surface.** `api.bambulab.com` answered every public read GET with no auth and no Cloudflare. No 403/429 seen on the JSON host.
- **One gap → browser-sniff target:** keyword search `GET /search-service/select/design?keyword=` exists (HTTP 200) but returns `total:0` for server-originated requests — the documented "search needs browser session state" behavior. Browser-sniff must capture the exact working keyword-search request (params/headers/cookies).
- Probe-safe endpoint used: `GET /search-service/select/design/nav?navKey=Trending` (read-only, anonymous).

## Top Workflows
1. **Browse/discover** trending or by category, ranked by likes/downloads/prints.
2. **Search** by keyword + filter (the one browser-sniff-gated path).
3. **Inspect a model**: details, instances, tags, ratings, remixes, related.
4. **Track a designer / collection** over time (new uploads, rising models).
5. **Download** a model's 3MF (auth-gated) → feed a slicer/printer.

## Table Stakes (absorb from existing tools)
- Apify scrapers (model details, designer stats, search) → match with browse/search/detail/designer commands, but offline + agent-native + SQLite.
- `schwarztim/bambu-mcp` → model detail, instance resolution, 3MF download, related/remix, comments. Match all.
- "3D GO" multi-platform app → search/collections/preview. Match MakerWorld slice.

## Data Layer
- **Primary entities:** designs, designers (creators), instances, comments/ratings, categories, collections.
- **Sync cursor:** offset/limit pagination per navKey/keyword; store designs+creators+ratings in SQLite for offline FTS search and trend tracking.
- **FTS/search:** local FTS over title/summary/tags/creator — the offline angle no scraper offers.

## Codebase Intelligence
- Source: `schwarztim/bambu-mcp` (`docs/cloud-api-reference.md`, `docs/design-service-api.md`, `src/makerworld.ts`).
- Auth: Bearer JWT (Bambu Cloud token); web path uses Cloudflare cookies. designId (numeric, from URL) ≠ modelId (alphanumeric, e.g. `US517b87ab155b42`). Download = design → default instance id → `/instance/{id}/f3mf?type=download`.
- Data model: design → instances → plates → 3MF; creator profiles; star ratings via comment-service.
- Rate limiting: none observed on api.bambulab.com public reads (be polite: adaptive limiter in client).

## Product Thesis
- **Name:** `makerworld-pp-cli` — "MakerWorld from the terminal."
- **Why it should exist:** No CLI exists for ANY of these 3D-model platforms. MakerWorld's clean anonymous JSON API makes a fast, offline-searchable, agent-native catalog mirror possible — track designers, watch contests/trends, and pull model metadata without a browser. The transcendence layer (local SQLite + trend deltas) is something no scraper or the official web UI offers.

## Build Priorities
1. Core read surface against `api.bambulab.com/v1` (browse, categories, design detail, related, remixes, comments/ratings) — anonymous, standard HTTP.
2. Keyword search wired from browser-sniff capture.
3. Local SQLite store + `sync` + offline FTS `search`.
4. Optional Bearer-token tier: download 3MF, favorites/likes, account.
5. Transcendence: designer-watch deltas, trend movers, rating/quality signals over the local mirror.
