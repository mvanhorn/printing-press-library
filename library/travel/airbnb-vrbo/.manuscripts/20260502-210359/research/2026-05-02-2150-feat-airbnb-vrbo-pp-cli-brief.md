# Airbnb + VRBO Combo CLI — Brief

## API Identity
- **Domain:** Short-term vacation rental aggregation + host-direct booking arbitrage.
- **Users:** Travelers shopping vacation rentals, agents booking trips, power users who care about platform fees, anyone who's ever paid 15 percent service fee on a $4k Airbnb.
- **Data profile:** Listings (10K-100K rows for an active user), hosts (PMC + individual), price history (per listing per date pair), wishlists (auth-gated), direct-site URLs (resolved + cached).

## Reachability Risk
- **Airbnb:** LOW for the SSR HTML path (openbnb pattern, anonymous, 443-star MCP works). HIGH for `/api/v3` GraphQL without auth (DataDome). We default to SSR.
- **VRBO:** MEDIUM. Akamai bot manager active; Surf-Chrome TLS impersonation + GET / warmup → wait → search/detail clears it (probe-reachability returned `browser_http`, not `browser_clearance_http`). Live agent-browser tested it and got blocked due to rapid-clicking heuristic; warmup pattern is essential.
- **Web search backends:** All standard_http; pluggable so failures degrade gracefully.

Evidence: ScrapingBee + Apify + Scrapfly all confirm Airbnb anti-bot is aggressive in 2025. openbnb-org/mcp-server-airbnb has 443 stars and uses SSR scrape successfully. VRBO Akamai gate observed live (2026-05-02 capture: "Bot or Not?" page after rapid navigation).

## Top Workflows
1. **Find direct booking site for a saved Airbnb/VRBO listing** — the killer arb. User has a URL, wants the host's direct site to skip the platform fee. Browser competitors exist (getaway.direct, hichee.com); no CLI does it.
2. **Plan a trip with platform-vs-direct savings ranking** — search both platforms, run cheapest on top results, return ranked list. One command.
3. **Cross-platform same-property match** — same condo on Airbnb $400 vs VRBO $340; the diff is the platform's price discrimination.
4. **Price-drop watchlist** — add saved listings, cron checks for drops, exit-code-7 signals action.
5. **Host portfolio analysis** — what does this PMC manage; do they have a direct site; what's their inventory.

## Table Stakes
- Airbnb search by location/dates/guests (openbnb has it; we match)
- VRBO search by location/dates/guests (Apify actors have it; we match)
- Listing detail with full pricing breakdown (both platforms)
- Geocoding (Photon → Nominatim chain)
- `--json`, `--select`, `--csv`, `--quiet` on every list/get
- `doctor` (auth + reachability + backend probe)
- MCP server (Cobra-tree mirror with read-only annotations)

## Data Layer
- **Primary entities:** listings, hosts (PMCs + individuals), prices (listing × date-pair), wishlists, watchlist, direct-sites (URL + last-checked-at), photos (for fingerprint).
- **Sync cursor:** `(source, listing_id, last_seen_at)` per source; price snapshot table is append-only for diff queries.
- **FTS:** Joined search across listing.title + listing.description + host.name + host.city.
- **Sync flow:** `sync` populates wishlists (auth) + watchlist (local) + their listings + their hosts + a price snapshot. Idempotent.

## Codebase Intelligence (DeepWiki-equivalent from MCP source extraction)
- **openbnb-org/mcp-server-airbnb:** Uses `cheerio` + URL search params on `/s/{slug}/homes` and `/rooms/{id}`. Walks `niobeClientData[0][1]` Apollo cache. No auth, no DataDome workaround. Brittleness: `#data-deferred-state-0` selector is single point of failure. Apply: replicate the Go equivalent with goquery.
- **VRBO via Stevesie + Apify writeups:** Single GraphQL endpoint (`POST /graphql`), APQ enabled (sha256Hash + query fallback), Akamai cookies via warmup. `propertySearch` is the confirmed search op; detail/autosuggest/availability operationNames TBD at runtime. Apply: build with op-name discovery in dogfood.
- **Auth (Airbnb GraphQL):** httpOnly session cookies + Arkose challenge mutation observed. Apply: `auth login --chrome` imports cookies; refresh on 401.

## User Vision (from briefing)
> Use a parallel api key to triple search airbnb, vrbo and "the internet" for the exact same listing to get the cheapest rate by searching for the host's name like "tahoegetaways" has their own site.

The tagline writes itself: **Skip the platform fee.** This vision is the central organizing principle, not a feature among many.

## Source Priority
- **Primary:** Airbnb — SSR HTML scrape (openbnb-pattern, no auth) for search/listing; optional GraphQL for wishlists with cookie auth.
- **Secondary:** VRBO — POST /graphql with Akamai warmup + Surf TLS impersonation; SSR `__PLUGIN_STATE__` as cheap entry.
- **Enrichment service:** Web search backend (Parallel/DDG/Brave/Tavily, pluggable). User said "doesn't have to be parallel shrug" — backend-agnostic.
- **Economics:** Free path (Airbnb + VRBO + DDG-backed cheapest) requires zero API keys. Paid backends are quality upgrades, never requirements.
- **Inversion risk:** None — primary is genuinely the headline source; web-search is an enrichment, not a peer.

## Product Thesis
- **Name:** `airbnb-vrbo-pp-cli` (binary). Tagline: "Skip the Airbnb/VRBO platform fee."
- **Why it should exist:** The "book direct" workflow is documented (CNBC, Explore.com, manual reverse image search guides) and the community already does it manually. Two browser extensions automate it (getaway.direct, hichee.com). NO CLI does it. A scriptable, agent-native, JSON-outputting version unlocks: trip-planner agents, cron-based price watchers, programmatic comparison shopping, integration into existing booking workflows. The free path requires zero API keys, removing the adoption barrier.

## Build Priorities
1. **Priority 0 (foundation):** Local SQLite store with listings/hosts/prices/wishlists/watchlist tables; openbnb-equivalent Airbnb SSR client; VRBO GraphQL client with Akamai warmup; pluggable search-backend interface.
2. **Priority 1 (absorbed):** All 24 commands in the absorb manifest's "Absorbed" table — Airbnb search/get/wishlist/similar/autosuggest, VRBO search/get/availability/autosuggest, doctor/auth/sql/search/sync/export/MCP.
3. **Priority 2 (transcendence):** All 10 commands in the absorb manifest's "Transcendence" table — cheapest, plan, compare, match, watch, host portfolio, wishlist diff, find-twin, fingerprint.

## Risks / Open Questions
- VRBO `propertyDetail` / `availability` / `autosuggest` operationNames unconfirmed (HAR not captured). Build with placeholder + discover at first dogfood; patch in run.
- Airbnb DataDome may eventually break the SSR-scrape path. Mitigation: ship `--ignore-robots-txt` flag; document the SSR fallback in the README.
- Web search rate limits on free DDG; recommend Brave/Tavily free tiers for production.
