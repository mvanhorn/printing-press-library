# wanderlust-goat CLI Brief

## API Identity
- Domain: walkable place discovery, fused multi-source local-knowledge surface
- Users: travelers with stated identity and stated criteria ("coffee snob into 70s kissaten culture", "photographer hunting hidden viewpoints", "vintage clothing collector"). Singular, sharp, non-comprehensive.
- Data profile: places (lat/lng, name, source, intent, trust, distance, walking-time), routes (OSRM walking polylines), neighborhood prose (Wikivoyage), curated picks (editorial scrapes), local-language gems (Tabelog, Naver, Le Fooding), Reddit threads.

## Reachability Risk
- **None for the universal stack.** Nominatim, OSM Overpass, OSRM public demo, Wikipedia/Wikivoyage REST, and Reddit JSON are all free, key-less, and well-supported. Polite rate limiting required for Nominatim and Overpass.
- **Low for editorial scrapes.** Eater, Time Out, NYT 36 Hours, and Michelin Guide listings are publicly accessible HTML; structured data (JSON-LD `Restaurant`, `Place`) commonly present. Bot detection is uncommon at low concurrency. Cache aggressively to SQLite.
- **Medium for Atlas Obscura.** AO has occasional Cloudflare gates. Mitigation: route through `cliutil.AdaptiveLimiter`, fall back to RSS/sitemap, label provenance.
- **Medium for Tabelog.** TLS-fingerprint sensitive. Use Surf (Chrome-impersonated client) like recipe-goat.
- **Low for Naver/Kakao.** Naver Map web pages and Naver Blog search render public; Kakao Map similar. Korean-language UA helps.
- **Low for Le Fooding.** Static-rendered editorial.

## Top Workflows
1. **"What should I walk to from here right now?"** — `near "Park Hyatt Tokyo" --criteria "vintage jazz kissaten, no tourists, great pour-over" --identity "coffee snob, into 70s Japanese kissaten culture" --minutes 15`. Returns 3-5 ranked, sourced, taste-matched picks with local-language names preserved.
2. **Agent-orchestrated deep research.** Agent calls `research-plan` to get a JSON query plan, executes per-source primitives in a loop, fuses the results. The CLI exposes both the plan and the primitives.
3. **Pre-trip city sync.** `sync-city Tokyo` pulls editorial best-of, Reddit threads, Wikipedia/Wikivoyage, OSM POI tags, Atlas Obscura entries into local SQLite for offline access.
4. **Crossover discovery.** `crossover --radius 800m` finds restaurants within 200m of a Wikipedia-notable historic site (food + culture in one walk).
5. **Golden-hour hunt.** `golden-hour "Eiffel Tower" --date 2026-06-15` computes sunrise/sunset/blue-hour locally (SunCalc-style), pairs with nearby viewpoints filtered by photographer-known crowd-beat heuristics.
6. **Route-view exploration.** `route-view "Shibuya Station" "Yoyogi Park"` enumerates interesting things along the walking path, not just at endpoints.

## Table Stakes
- Geocoding (address → lat/lng).
- Walking-distance/time (real OSRM, not crow-flies).
- POI search by category and tag.
- Wikipedia GeoSearch + page fetch.
- Reddit thread search by subreddit.
- Editorial best-of scrapes (Eater, Time Out, Michelin, NYT 36 Hours, Bib Gourmand).
- Local SQLite store with FTS5 search.
- `--json`, `--select`, `--csv`, `--compact` output modes; auto-JSON when piped.
- `--dry-run` on every command that hits the network.
- `--data-source auto/live/local` resolver with `DataProvenance`.

## Data Layer
- Primary entities: `places` (id, lat, lng, source, intent, trust, name, name_local, address, country, region, walking_time_min, distance_m, why_special, raw_json), `cities` (slug, name, country, last_synced_at, source_counts), `routes` (cached OSRM polylines), `reddit_threads` (subreddit, title, url, score, comments, body, criteria_match).
- Sync cursor: `sync_state` table (per-source last_run, etag, cursor).
- FTS5: `places_fts` (name, name_local, why_special, address), `resources_fts` (generic for non-place rows).
- Trust + Tier baked into typed `Source` slice (matches recipe-goat pattern); Trust drives goat-score; Tier ranks editorial vs community vs OSM tag.

## Codebase Intelligence
- **Reference architecture: recipe-goat (food-and-dining).** Same fused-multi-source GOAT shape, same per-source typed Client packages under `internal/<source>/`, same `data_source.go` resolver, same sync flag set, same trust-weighted scoring formula. Reuse this skeleton verbatim.
- **No DeepWiki for the synthesized CLI** (it's the first wanderlust-goat ever generated). Per-source codebase intelligence exists where applicable: nominatim-org/Nominatim, project-osrm/osrm-backend, drolbr/Overpass-API, MaxRoach/atlas_obscura (Python wrapper, useful for sitemap structure).
- Auth: none for any v1 source. Polite User-Agent string ("wanderlust-goat-pp-cli/0.1 (+contact)") required by Nominatim and Wikipedia.
- Rate limiting: Nominatim 1 req/s absolute. Overpass 2 concurrent recommended. OSRM public demo no published limit but polite caps. Reddit ~60 req/min unauthenticated. Editorial sites: 1-2 req/s with jitter.

## User Vision (USER_BRIEFING_CONTEXT)
The user provided the complete brief upfront and explicitly told the agent not to re-derive it. Direct quotes worth preserving:

- **NOI:** "A map isn't a list of places. It's a stack of human-attention layers — what locals tag, what photographers shoot, what historians document, what curators rank, what discerning travelers tell strangers about on Reddit, what locals review in their own language. 'What should I walk to from here' is a query against that stack — filtered by the user's own taste and identity, not by what's closest."
- **Anti-feature:** comprehensiveness. "If it returns 40 cafes, it failed. If it returns the 3 cafes that match the user's stated taste, it won."
- **Architecture:** "The CLI is the muscle, the calling agent is the brain." CLI exposes powerful per-source primitives + a `research-plan` meta command that produces a JSON query plan. Also ships a `goat` compound that runs heuristic orchestration without an LLM, so the CLI works standalone.
- **Killer feature:** language-aware regional fanout. "Locals discuss the actual gems on local-language sites; tourist traps dominate English sources." Tabelog (JP), Naver (KR), Le Fooding (FR) are real packages with real clients in v1, not stubs. Documented extension pattern for additional regions.
- **Output shape:** ranked + sourced + one-line "why it's special" per item, with local-language names preserved alongside transliterations (e.g. `珈琲 美美 (Kohi Bibi)`).
- **Trust weights** (per source) and **walking-time radius** (not meters) are explicit in the brief — bake into the goat-score formula.
- **Defer:** Foursquare (free tier collapses 2026-06-01), Yelp (no free tier since 2024), Google Places (cost), Mapillary/Flickr (v2 photo-density layer).
- **OSRM:** public demo with documented self-host upgrade path in README.

## Source Priority
This is **not** a combo CLI in the priority-gate sense — it's a fused multi-source CLI where the persona-shaped compound commands (`near`, `goat`, `research-plan`) are the headline. No single source leads the README. Trust weights live in the goat-score formula, not in priority ordering. Per-source confidence:

| Tier | Sources | Trust | Notes |
|------|---------|-------|-------|
| Foundation | Nominatim, OSRM, OSM Overpass | 1.0, 1.0, 0.90 | Free, key-less, deterministic. |
| Editorial (curated by humans) | Michelin (free side), NYT 36 Hours, Wikipedia, Eater, Time Out, Wikivoyage | 0.95–0.85 | Scraped, high signal. |
| Local-language regional (v1) | Tabelog (JP), Le Fooding (FR), Naver high-rated (KR) | 0.90, 0.90, 0.85 | +0.05 country-match boost. |
| Hidden gems | Atlas Obscura | 0.80 | Browser-sniff if direct fetch hits CF. |
| Crowd | Reddit (≥10 upvotes, ≥3 comments) | 0.75 | Filter low-signal threads. |

## Product Thesis
- **Name:** wanderlust-goat
- **Display name:** Wanderlust GOAT
- **Headline:** What a knowledgeable local with great taste would tell you to walk to from here — fused across the editorial, local-language, and crowd layers no single tool ranks together.
- **Why it should exist:** Every existing "near me" tool returns the closest 40 things. The user wants the 3 things that match their stated taste. No tool does language-aware regional fanout (the actual locals know the gems and discuss them on local-language sites). No tool ranks editorial + Wikipedia + Reddit + OSM tags through one trust-weighted formula. No tool exposes both an agent-orchestration JSON query plan AND a heuristic standalone compound.

## Build Priorities
1. **Foundation (Priority 0):** Internal YAML spec with `places` (geocode + store), `pois` (Overpass), `routes` (OSRM), `wikipedia` (GeoSearch + page), `wikivoyage` (page), `reddit` (search). Internal SQLite with `places`, `cities`, `routes`, `reddit_threads`, `places_fts`, `resources_fts`, `sync_state` tables. Copy `data_source.go`, sync flag set, and trust+tier `Source` slice from recipe-goat.
2. **Universal source clients (Priority 1a):** typed `Client` per source under `internal/<source>/`: nominatim, overpass, osrm, wikipedia, wikivoyage, atlasobscura (browser-sniff), reddit, eater, timeout, nyt36hours, michelin (incl. bib gourmand).
3. **Regional source clients (Priority 1b — v1 = JP + KR + FR):** tabelog, fourtravel, retty, hotpepper, jpwiki, jpwikivoyage (Japan); navermap, kakaomap, mangoplate, naverblog, kowiki (Korea); lefooding, pudlo, frwiki, frwikivoyage (France). Documented extension pattern for IT/DE/UK/SE/CN/ES.
4. **Trust + tier ranking (Priority 1c):** typed `Source` slice with Trust, Tier, CountryMatchBoost. Implement goat-score formula:
   `score = trust * (1 + country_boost) * intent_match * (1 / (1 + walking_minutes/15))` with editorial multiplier and Reddit comment-density tiebreak.
5. **Compound commands (Priority 2 / transcendence):** `near`, `goat`, `research-plan`, `route-view`, `crossover`, `golden-hour`, `sync-city`. All hand-written. Persona-first absorb manifest framing.
6. **Polish (Priority 3):** Surf integration for TLS-sensitive sources (Tabelog, Atlas Obscura), README OSRM self-host instructions, tests for goat-score and language-fanout dispatch logic.

## Non-Goals (v1)
- Foursquare, Yelp, Google Places (defer per brief).
- Mapillary/Flickr photo-density (v2).
- Comprehensive lists. The output is intentionally narrow — the persona's matched picks, not the complete radius dump.
- Browser-resident transports. Browser is allowed for *discovery* (Atlas Obscura sniff) but the printed CLI uses replayable HTTP, Surf-impersonated HTTP, or HTML/SSR/JSON-LD extraction at runtime.
