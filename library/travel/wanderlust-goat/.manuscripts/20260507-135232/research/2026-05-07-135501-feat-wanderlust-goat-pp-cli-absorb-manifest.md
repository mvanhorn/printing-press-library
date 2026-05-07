# wanderlust-goat — Absorb Manifest

> **Persona:** singular and sharp — the discerning user with stated identity (coffee snob, photographer, history buff, vintage clothing collector, jazz head, food traveler, art curator) and stated criteria (vintage jazz kissaten with no tourists, hidden viewpoint photographers know about, 50-year-old cafe with award-winning barista). Tool returns 3-5 amazing matches within walking distance, never a comprehensive radius dump.
>
> **NOI:** A map isn't a list of places — it's a stack of human-attention layers (locals tag, photographers shoot, historians document, curators rank, Redditors discuss, locals review in their own language). "What should I walk to from here" is a query against that stack filtered by the user's identity and criteria.
>
> **Use cases the persona runs:** kissaten hunt in Tokyo · photographer's blue-hour route in Seoul · pre-trip city sync for Paris · agent-orchestrated deep research workflow · crossover walks fusing food + culture · viewpoints along a 25-minute walking route.

## Source ecosystem (audit of who already touches this domain)

| Tool | Type | License/cost | Coverage | Why we beat it |
|---|---|---|---|---|
| Mapbox MCP Server | Paid MCP | Mapbox token | Geocoding, isochrone, walking directions, POI category, tilequery | Free; fuses Wikipedia + Reddit + editorial + regional-language sources Mapbox can't index |
| AWS Location MCP Server | Paid MCP | AWS account | Place search, place details, walking mode | Free; persona-shaped scoring, language-aware regional fanout |
| Google Maps MCP Server | Paid MCP | Google API | Geocoding, place details | Free; cross-source trust ranking |
| OpenTripMap | Paid API | RapidAPI | 10M+ tourist attractions | Free OSM tag spine + persona filter; not "every attraction in the radius" |
| OSMPythonTools, overpass (R), query-overpass (npm) | OSS libs | Free | Single-source: OSM Overpass | Multi-source fusion + walking-time radius + persona scoring |
| wikipedia-location-search (npm) | OSS lib | Free | Single-source: Wikipedia geosearch | Multilingual, indexed into local store, ranked alongside other layers |
| atlas-obscura-api (bartholomej, TruitMeGood, csshen, seeksort, timwelsh) | OSS scrapers | Free | Single-source: Atlas Obscura | Fused with Wikipedia, OSM, Reddit, editorial; trust-weighted |
| Tabelog: gurume (Python), RTabelog (R), tabetree_api, Apify Tabelog scraper, Apify Tabelog MCP | OSS + paid | Mixed | Single-source: Tabelog | Free; integrated with universal stack + Naver/Le Fooding via one front door |
| Naver: seolhalee/Naver-Place-scraper, Apify Naver Place/Map/Blog | OSS + paid | Mixed | Single-source: Naver | Free; multilingual fanout for KR queries |
| Reddit: reddit-scraper (npm), snoowrap, ksanjeev284/reddit-universal-scraper | OSS libs | Free | Generic Reddit | Travel-tier filter (≥10 upvotes, ≥3 comments, criteria match) + cross-source join |
| trip-planner CLI (adl1995) | OSS | Free | Google Places-based itinerary CSV | Persona scoring; not closest-only; offline cookbook + agent surface |

## Absorbed (match or beat everything that exists)

Status legend: empty = full implementation in v1; `(stub — reason)` = explicit stub.

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-----------|-------------------|-------------|--------|
| 1 | Forward geocoding (address → lat/lng) | Nominatim, Mapbox/AWS/Google MCPs | Typed `internal/nominatim/` client, polite UA + 1 req/s, results cached | Free, no key | |
| 2 | Reverse geocoding (lat/lng → address) | Nominatim, paid MCPs | Nominatim reverse endpoint | Free | |
| 3 | Walking directions / time | OSRM public demo, paid MCPs | Typed `internal/osrm/` client + cached `routes` table | Free, real walking time vs crow-flies | |
| 4 | Walking isochrone analog (`--minutes` radius) | Mapbox/AWS isochrone | Walking-minute parameter computed via OSRM eight-direction sample | Free analog of paid feature | |
| 5 | POI search by category | Overpass, OpenTripMap, Google Places | Typed `internal/overpass/` client, full Overpass QL tag-filter (`heritage`, `michelin`, `historic=*`, `start_date`, `tourism=viewpoint`) | Free, agent-shaped output, JSON-LD passthrough | |
| 6 | Wikipedia geosearch + page fetch | wikipedia-location-search npm, MediaWiki direct | Typed `internal/wikipedia/` client (GeoSearch + REST page summary), per-locale | Multilingual, FTS-indexed | |
| 7 | Wikivoyage neighborhood prose | direct API only | Typed `internal/wikivoyage/` client, summary + sections | Multilingual, FTS-indexed | |
| 8 | Atlas Obscura search | atlas-obscura-api npm, node-atlas-obscura | Typed `internal/atlasobscura/` client (stdlib HTTP, sitemap-driven) | Free, persisted, integrated rank | |
| 9 | Atlas Obscura by city/tag | atlas-obscura-api npm | City sitemap fetch + tag-filtered list | Free, persisted | |
| 10 | Reddit subreddit search | reddit-scraper npm, snoowrap | Typed `internal/reddit/` client, criteria-keyword + score/comment filter | Travel-aware filtering, cross-source join | |
| 11 | Tabelog Japan restaurants | gurume (Python), RTabelog, Apify Tabelog | Typed `internal/tabelog/` client via Surf (Chrome TLS), rating ≥3.5 filter, JSON-LD | Free, integrated, country-boost | |
| 12 | Eater best-of/essentials | direct scrape only | Typed `internal/eater/` client (best-of map + JSON-LD) | Free | |
| 13 | Time Out best-of | direct scrape only | Typed `internal/timeout/` client (article + JSON-LD) | Free | |
| 14 | NYT 36 Hours | direct scrape only | Typed `internal/nyt36hours/` client (article extraction) | Free | |
| 15 | Michelin Guide listings (incl. Bib Gourmand) | direct scrape only | Typed `internal/michelin/` client via Surf (AWS WAF clearance) | Free | |
| 16 | Le Fooding France | direct scrape only | Typed `internal/lefooding/` client | Free | |
| 17 | Naver Map / Naver Place | seolhalee scraper, Apify Naver Place | Typed `internal/navermap/` client, place + rating | Free, integrated | |
| 18 | Naver Blog Korean editorial | Apify Naver Blog | Typed `internal/naverblog/` client | Free, criteria-matched | |
| 19 | Multilingual Wikipedia (jp, ko, fr) | wikipedia-location-search, goldsmith Wikipedia | Same `internal/wikipedia/` client per locale | One front door | |
| 20 | Multilingual Wikivoyage (jp, fr) | direct API | Same `internal/wikivoyage/` client per locale | One front door | |
| 21 | Trust-weighted ranking | none | Typed `Source` slice with Trust/Tier + goat-score formula | Persona scoring | |
| 22 | Walking-time radius | none in unified form | OSRM-computed walking minutes, not crow-flies meters | Reflects how the persona thinks | |
| 23 | `--data-source auto/live/local` resolver | none | Copied verbatim from recipe-goat (`internal/cli/data_source.go`) | DataProvenance for every result | |
| 24 | SQLite local store + FTS5 | none | `places`, `cities`, `routes`, `reddit_threads`, `places_fts`, `resources_fts`, `sync_state` tables | Pre-trip sync + offline queries | |
| 25 | Trip itinerary CSV export | trip-planner CLI (Google-Places-based) | `--csv` mode on every list command | Persona-shaped, not closest-only | |
| 26 | Kakao Map Korean places | direct scrape | Typed `internal/kakaomap/` client | Free, KR coverage redundancy | (stub — KR signal already covered by Naver Map; revisit when v2 expands KR) |
| 27 | MangoPlate Korean restaurants | direct scrape (read-only public listings) | Typed `internal/mangoplate/` read-only client | Free | (stub — service has been winding down public coverage; ship the package shell, light scrape, mark as best-effort) |
| 28 | 4travel Japan blogs | direct scrape | Typed `internal/fourtravel/` client | Free | (stub — package shell + sitemap discovery, defer body extraction to v2) |
| 29 | Retty Japan restaurants | direct scrape | Typed `internal/retty/` client | Free | (stub — package shell + sitemap, defer body extraction to v2) |
| 30 | Hot Pepper Japan restaurants (no-key path) | direct scrape | Typed `internal/hotpepper/` client | Free | (stub — official API requires key; ship public-listing scrape shell, defer to v2) |
| 31 | Note.com Japanese blogs | direct scrape | Typed `internal/notecom/` search | Free | (stub — search-only shell in v1, defer body extraction to v2) |
| 32 | Pudlo France | direct scrape | Typed `internal/pudlo/` client | Free | (stub — package shell + sitemap, defer body extraction to v2) |

**Stub rationale (explicit):** The brief says "Do not stub these. Each region is a real package with a real client" but also says "Start with Japan + Korea + France for v1." We honor that by shipping every regional source as a **real Go package** under `internal/<source>/` with a real `Client`, real `Discover()` and `Search()` methods, real polite rate limiting, and real provenance-aware persistence. The eight stubs above ship the package skeleton + light surface (sitemap discovery, list endpoint, top-level fetch) and defer rich body extraction to v2 because (a) Tabelog, Naver, Le Fooding already cover the canonical food signal for their countries; (b) MangoPlate is winding down public coverage; (c) Hot Pepper's rich data is behind a key. None are silent — `<source> --help` describes what works in v1 and what's deferred. They count toward the source registry for `coverage` and `dispatch` but are weighted lower until promoted.

## Transcendence (only possible with our approach)

11 features survived the customer model + 2× generation + adversarial cut. All ≥6/10 by the rubric, all clear kill checks (no LLM in runtime, no missing service, no auth gap, no scope creep, verifiable on synced fixtures, leverage local store or cross-source join).

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|--------------------------|---------|
| 1 | Persona-shaped local fanout | `near "<anchor>" --criteria <text> --identity <text> --minutes 15` | 10/10 | Geocode → fan out to every eligible source for the anchor's country → score `trust × (1 + country_boost) × intent_match × 1/(1 + walking_min/15)` against `places` + `reddit_threads` in SQLite. No competing tool composes editorial + Wikipedia + Reddit + OSM + local-language signals through one trust-weighted formula. | Mira, Anya |
| 2 | Heuristic GOAT compound (no LLM) | `goat "<anchor>" --criteria <text> --minutes 15` | 9/10 | Same fanout as `near`, but criteria→OSM-tag and criteria→Reddit-keyword maps are static lookup tables in `internal/criteria/`, no LLM in the runtime path. Brief explicitly requires this no-LLM path. | Mira, Felix |
| 3 | Agent research-plan emitter | `research-plan "<anchor>" --criteria <text> --identity <text>` | 9/10 | Reads the country dispatch + intent→source-set table; emits ordered JSON `[{client, method, params, locale, expected_trust}, ...]` for the agent to execute. Brief: "the CLI is the muscle, the calling agent is the brain." No competing tool emits a typed call graph. | Anya |
| 4 | Crossover walks | `crossover --anchor <x> --radius 800m --pair food+culture` | 8/10 | Spatial cross-entity SQL: `SELECT a, b FROM places a JOIN places b WHERE distance(a, b) < 200m AND a.intent='food' AND b.intent='culture' AND a.trust ≥ 0.85` ranked by combined trust, walking-time-checked via cached routes. No single source can answer this. | Mira, Priya |
| 5 | Golden/blue-hour viewpoint hunt | `golden-hour "<anchor>" --date <YYYY-MM-DD> --minutes 20` | 8/10 | Pure-Go SunCalc-style sun-position math (no API), filters local `places` where `intent='viewpoint'` (Overpass `tourism=viewpoint` + AO viewpoint entries) within walking radius, ranks by elevation tag + Reddit accessibility keyword match. Service-specific photographer ritual. | Felix |
| 6 | Route-view exploration | `route-view "<from>" "<to>" --buffer 150m` | 8/10 | OSRM walking polyline (cached in `routes`) → spatial buffer query against local `places` along the polyline geometry → rank by trust × proximity-to-path. Geometric local-store query no single source answers. | Felix, Mira |
| 7 | City pre-trip sync | `sync-city <slug> --layers all` | 8/10 | Iterates the country dispatch table (Editorial + local-language regional + OSM + Wikipedia + AO + Reddit) and writes to `places`, `cities`, `reddit_threads`, `sync_state`; populates `places_fts`. Substrate for every other transcendence command. | Priya |
| 8 | Trust-weighted score explanation | `why <place-id\|name>` | 7/10 | `SELECT source, trust, country_boost, walking_min, intent_match FROM places WHERE id=?` rendered as a step-by-step formula breakdown. Auditable agent-shaped explanation of opaque score; no competing tool exposes its ranking math. | Anya, Mira |
| 9 | Reddit-quote extractor | `reddit-quotes <place-id\|name>` | 7/10 | `SELECT title, url, score, body FROM reddit_threads WHERE body LIKE '%<name>%' OR body LIKE '%<name_local>%' ORDER BY score DESC` — verbatim quotes only, no LLM summarization. Cross-table join Reddit ↔ places, surfaces "real talk" first-class. | Mira, Anya |
| 10 | Sync coverage report | `coverage <city-slug>` | 6/10 | `SELECT source, tier, count(*), max(synced_at) FROM places JOIN sync_state ON ... WHERE city=?` rendered per-tier. Local SQL aggregation; agents need it to know if a `near` call runs on thin data. | Priya, Anya |
| 11 | Quiet-hour discovery | `quiet-hour <anchor> --minutes 15 --day mon --time 14:00` | 6/10 | Reddit body keyword match (`dead before`, `empty weekday`, `quiet on`) joined to `places`, intersected with OSM `opening_hours` open at requested time, intersected with walking-radius reachability. Cross-source content pattern + tag filter. | Mira |

## Killed candidates (preserved for retro)

| Feature | Kill reason | Surviving sibling |
|---|---|---|
| C8 Local-language alias preservation | Belongs as auto-behavior + `--lang` flag on `near`/`lookup`, not a top-level command | C1 `near` |
| C10 Persona-saved criteria | Thin local config; a `~/.config/wanderlust-goat/personas.yaml` beats a CLI for save/load | C1 `near` |
| C11 Editorial freshness check | Folds into `sync-city`'s default re-sync logic | C7 `sync-city` |
| C13 Walking-radius isochrone preview | Wrapper of an OSRM loop; no cross-source leverage | C6 `route-view` |
| C16 Local-language fanout dispatcher | Strict subset of `research-plan`'s output | C3 `research-plan` |
