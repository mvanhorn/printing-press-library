# Zameen CLI Brief

## API Identity
- **Domain:** Real estate / property marketplace. Zameen.com is Pakistan's dominant property portal (Dubizzle/EMPG group) — listings for buy & rent across homes, plots, and commercial, plus new projects, agencies/agents, and area price research.
- **Users:** Home buyers/renters, property investors (plot flipping is huge in PK), real-estate agents doing market research, and data analysts studying the PK property market.
- **Data profile:** ~14k homes-for-sale in Islamabad alone, ~27k in Lahore; hundreds of thousands of live listings nationwide. Each listing carries price (PKR), area (with Marla/Kanal units), beds/baths, location hierarchy (city → area → society), agency/agent, geo coordinates, photos, purpose, and timestamps.

## Discovered Surface (primary, credential-free, standard HTTP)
- **Transport:** `probe-reachability` → `standard_http` (confidence 0.95). Plain stdlib HTTP returns HTTP 200 with full server-rendered HTML. No Cloudflare challenge, no browser/clearance runtime needed. (Note: the site sits behind Cloudflare+CloudFront, so a User-Agent header is used, but no JS challenge fires.)
- **Search:** `GET https://www.zameen.com/{Category}/{location-slug}-{page}.html`
  - `Category` ∈ `Homes` | `Rentals` | `Plots` | `Commercial`
  - `location-slug` is a single token combining a name + numeric ID; **the numeric ID is authoritative** (e.g. `Islamabad-3`, `Lahore-1`, `Lahore_DHA_Defence-9`). Works at both city and area level with the same URL shape.
  - trailing `-{page}` is 1-based pagination; **25 hits/page**; `nbPages` up to ~1000+.
- **Response extraction:** each page embeds `window.state = {…}` (a Redux SSR blob, JS assignment). The listings live at `state.algolia.content.hits[]` with `nbHits`, `nbPages`, `hitsPerPage`. Extraction requires brace-balanced JS parsing → **hand-built Go client** (the generator's `html_extract` embedded-json mode targets `<script>` JSON blobs, not `window.state = …` assignments).
- **Listing hit schema (25 fields):** `id`/`objectID`, `externalID`, `title`, `price` (int PKR), `area` (float, sq-something), `rooms` (beds), `baths`, `purpose` (`for-sale`/`rent`), `category` (list; Homes/Plots/Commercial + subtype), `location` (hierarchical list: Pakistan → Province → City → Area → Society), `agency`, `contactName`, `phoneNumber`, `coverPhoto`, `photoCount`, `videoCount`, `geography` (lat/lng), `isVerified`, `product` (ad tier: superhot/hot/etc.), `createdAt`/`updatedAt` (unix), `slug`, `shortDescription`, `installments` (for plot files), `state` (active).
- **Property detail URL:** `/Property/{slug}-{externalID}-{locationID}-{seq}.html` (constructable from a hit's slug + externalID + leaf location ID for an "open in browser" command).
- **Filters:** Zameen encodes price/beds/baths/area filters in the **URL path** (long-tail), **not** query strings — confirmed `?beds_in`/`?price_min`/`?sort` are ignored. So the CLI applies price/beds/baths/area/purpose/verified filters and sort **client-side** over scanned pages (scan-and-filter pattern).
- **Additional surfaces observed in `window.state`** (secondary, hand-buildable): `areaGuide`, `priceTrends`/`areaTrends`/`popularityTrends`, `search.recommendations`, `similarProperties`, `seo.popularSearches`. New projects at `/new-projects/`; agencies at `/agents/{City}/{Agency-Slug-ID}/`.

## Reachability Risk
- **None/Low.** Direct stdlib + Surf-Chrome probes both returned HTTP 200; all 10+ live curls during discovery returned 200 with full data. A browser User-Agent header is included as defense-in-depth against future bot heuristics.
- Community scrapers historically leaned on Selenium/Apify because they scraped rendered DOM; the **embedded `window.state` JSON on the server-rendered page is far more robust** and is what this CLI uses.
- Residual risk: Zameen could add a JS challenge later, or rate-limit aggressive paging. Mitigated by per-source adaptive rate limiting and bounded `--max-scan-pages`.
- **We do NOT use** the raw AWS Elasticsearch endpoint or its embedded search credential exposed in the page (secret-protection rule; a credential must never ship in the CLI). The public search HTML surface needs no credential.

## Top Workflows
1. **Parametric listing search** — buy/rent × type × location × price × beds/baths/area, sorted, clean JSON/CSV output. The #1 use case.
2. **Saved searches + monitoring** — persist a named query, re-run it, diff against last run to surface **new listings** and **price drops**.
3. **Bulk export** — dump a full result set to CSV/JSON for market analysis (every community scraper reinvents this).
4. **Area / price research** — median price & price-per-marla by area, inventory counts, newest listings.
5. **Agency / new-projects discovery** — find agencies and their inventory; browse new developments and installment plans.

## Table Stakes (from homehunt + property-portal CLIs)
- Rich search filters (purpose, type, city/area, price min/max, beds, baths, area size).
- Pagination handling + result limits.
- Local store (SQLite) of scanned listings; offline search/SQL.
- Saved/named search configs.
- New-listing + price-drop diffing/alerts.
- JSON + CSV export.
- `--json`, `--select`, `--agent`, typed exit codes, `--dry-run`.

## Data Layer
- **Primary entities:** `listing` (property ad), `location` (city/area/society, hierarchical), `agency`, `project` (new development).
- **Sync cursor:** listings by `updatedAt`/`createdAt` (unix); scan pages of a saved query into the store, upsert by `externalID`.
- **FTS/search:** offline full-text over title/location/agency; SQL over price/area/beds for local analytics.
- **Diffing:** snapshots of query results keyed by `externalID` → detect added listings and price changes across runs.

## Codebase Intelligence
- No official/public API; no maintained wrapper. Community repos (IIvexII/zameen-com-scrapper, osmanrkhan/zameen.com-scraper, aliraheel626/zameen-scraper) are hobby HTML scrapers, 0–6 stars, mostly Selenium/BeautifulSoup → CSV. Apify actors and an unvetted RapidAPI "ZamBee" exist behind paywalls. **No existing Zameen CLI — greenfield.**
- Marla/Kanal are Pakistani area units (1 Marla ≈ 225–272 sq ft depending on region; 1 Kanal = 20 Marla). Price is PKR; large values (crore = 10M, lakh = 100k) are idiomatic.

## Product Thesis
- **Name:** `zameen-pp-cli` — "Every Zameen listing, in your terminal — with an offline store, saved-search monitoring, and price-drop alerts no scraper offers."
- **Why it should exist:** Zameen has no API and no alerts. Investors and agents manually refresh search pages daily. A CLI that searches with real filters, keeps a local mirror, diffs runs for new listings and price drops, and exports to CSV — over a robust credential-free surface — is genuinely useful and doesn't exist.

## Build Priorities
1. Hand-built `internal/zameen` client: fetch search page → extract `window.state.algolia.content` → typed `Listing`; pagination; adaptive rate limiting; bound scans.
2. `search` (filters + sort + client-side scan-and-filter), `get`/`open`, local store + `sync` + offline `search`/`sql`, CSV/JSON export.
3. Transcendence: saved searches, new-listing + price-drop diff/alerts, area price stats (median, price-per-marla), Marla/Kanal-aware area handling.
