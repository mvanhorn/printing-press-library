# Airbnb + VRBO Combo CLI — Absorb Manifest

**API name:** airbnb-vrbo (slug). Binary: `airbnb-vrbo-pp-cli`.
**Tagline:** "Skip the Airbnb/VRBO platform fee. Find the host's direct booking site in one command."

## Source priority (confirmed in gate)
1. **Airbnb** (primary) — SSR HTML scrape (openbnb pattern, MIT-licensed) for search + listing; optional GraphQL persisted-query mode for wishlists/calendar with cookie auth.
2. **VRBO** (secondary) — `POST /graphql` with Akamai warmup + Surf-Chrome TLS impersonation; SSR `__PLUGIN_STATE__` for fast metadata.
3. **Web search backend** (enrichment, pluggable) — Parallel.ai (paid, best), DuckDuckGo HTML (free default), Brave Search API (free tier), Tavily (free tier).

## Tools surveyed (every one feeds the absorb)

| Tool | Type | Stars | Auth | What we absorb |
|---|---|---|---|---|
| openbnb-org/mcp-server-airbnb | MCP server (TS) | 443 | None (anonymous) | Full SSR-scrape pattern, `airbnb_search`, `airbnb_listing_details`, geocoding chain (Photon→Nominatim) |
| vedantparmar12/airbnb-mcp | MCP server | <50 | None | Comparison + smart-filter tool ideas |
| johnbalvin/pyairbnb | Python SDK | unknown | None | Python reference patterns (not absorbed directly) |
| Apify ecomscrape/vrbo-property-search-scraper | Hosted scraper | n/a | Apify token | VRBO `propertySearch` operation shape, geo input, paging |
| Apify easyapi/vrbo-property-listing-scraper | Hosted scraper | n/a | Apify token | VRBO property detail shape |
| Apify jupri/vrbo-property | Hosted scraper | n/a | Apify token | Input schema for propertyId convention |
| markswendsen-code/mcp-vrbo | MCP server (DOM) | <20 | None | DOM-based extraction patterns |
| Edioff/vrbo-scraper | GitHub repo | <30 | None | Replay-based VRBO scraping |
| Stevesie VRBO API | No-code service | n/a | None | HAR-based GraphQL endpoint documentation |
| getaway.direct | Chrome extension | n/a | None | **Killer-feature competitor**: finds direct booking sites for Airbnb/VRBO/Booking/Expedia. Browser-only. |
| hichee.com | Web app | n/a | None | **Killer-feature competitor**: 13M-listing index, reverse-image search method. Browser-only. |
| BookDirect.com / Houfy.com | Marketplaces | n/a | None | Host-side direct booking aggregators (not absorbed; they're our destination, not source) |
| AirDNA | Paid market research | n/a | Paid | Listing↔revenue correlation (out of scope) |

## Absorbed features (match or beat everything that exists)

| # | Feature | Source | Our Implementation | Added Value | Status |
|---|---|---|---|---|---|
| 1 | Airbnb search by location | openbnb_search | `airbnb search <location>` — SSR scrape `/s/{slug}/homes` | Offline FTS, --json, --select, agent-native, MCP-exposed | shipping |
| 2 | Airbnb search filters (dates/guests/price/property-type/rooms) | openbnb_search | Same command, full flag set | Saved searches, replay from history | shipping |
| 3 | Airbnb geocoding (Photon → Nominatim) | openbnb util.ts | `airbnb geocode <location>` | Local cache TTL, --json | shipping |
| 4 | Airbnb listing detail | openbnb_listing_details | `airbnb get <id>` — SSR scrape `/rooms/{id}` | Local cache, --select for partial fields | shipping |
| 5 | Airbnb listing pricing for dates | StaysPdpBookItQuery (GraphQL) | `airbnb price <id> --checkin --checkout --guests` | Compare against VRBO same dates, store per-day | shipping |
| 6 | Airbnb wishlists (read user's saved) | WishlistIndexPageQuery (GraphQL, auth) | `airbnb wishlist list` | Local sync, cross-device, JSON | shipping |
| 7 | Airbnb wishlist items | WishlistItemsAsyncQuery | `airbnb wishlist items <wishlist-id>` | Joined with local price history | shipping |
| 8 | Airbnb similar listings | SimilarListingsCarouselQuery | `airbnb similar <listing-id>` | Stored locally, dedupe across sessions | shipping |
| 9 | Airbnb autosuggest (location autocomplete) | AutoSuggestionsQuery | `airbnb autosuggest <prefix>` | Used by `search` --interactive | shipping |
| 10 | VRBO search | propertySearch GraphQL + SSR | `vrbo search <location> --checkin --checkout --guests` | Same UX as Airbnb search, normalized output schema | shipping |
| 11 | VRBO property detail | propertyDetail GraphQL + `__PLUGIN_STATE__` SSR | `vrbo get <id>` | Cached, --select | shipping |
| 12 | VRBO availability/calendar | availability GraphQL (op name to confirm at runtime) | `vrbo availability <id>` | Per-night view, blocked dates | shipping |
| 13 | VRBO autosuggest | autosuggest GraphQL (op name TBD) | `vrbo autosuggest <prefix>` | Used by `search` --interactive | shipping |
| 14 | "Find direct booking site" (host-direct discovery) | getaway.direct, hichee | `cheapest <listing-url>` — host extraction + web-search backend | **CLI-native**, scriptable, agent-native, JSON output, free fallback (DDG) | shipping ⭐ killer |
| 15 | Cross-platform listing match | (no existing tool) | `match <listing-url>` — Airbnb→VRBO or VRBO→Airbnb same property | Geocode + amenity + photo signal joined | shipping ⭐ |
| 16 | Reverse-image listing search | hichee.com magic button | `find-twin <photo-url>` — image search via Parallel/Tavily | First CLI to do it; falls back gracefully | shipping (degrades when image-search backend missing) |
| 17 | Doctor (env, auth, reachability) | standard | `doctor` | Probes Airbnb, VRBO, search backend; reports Akamai status | shipping |
| 18 | Auth login (browser cookie import) | standard | `auth login --chrome` | Imports cookies for authenticated GraphQL | shipping |
| 19 | Auth status + refresh | standard | `auth status`, `auth refresh` | Detects `_abck` expiry | shipping |
| 20 | Local SQL query | standard pp-cli surface | `sql "SELECT..."` | Power-user introspection | shipping |
| 21 | Local FTS search | standard pp-cli surface | `search "<term>"` | Joins listings/hosts/wishlists | shipping |
| 22 | Sync (populate local store) | standard | `sync` | Pulls wishlists + watchlist + saved listings | shipping |
| 23 | Export (CSV/JSON dump) | standard | `export listings`, `export hosts` | For spreadsheet workflows | shipping |
| 24 | MCP server | standard | `airbnb-vrbo-pp-cli mcp serve` | Cobra-tree mirror, all read-only commands annotated | shipping |

## Transcendence features (only possible with our approach)

These are the differentiators — none of these exist as a CLI today.

| # | Feature | Command | Why Only We Can Do This | Score | Status |
|---|---|---|---|---|---|
| T1 | **Triple-source price arbitrage** | `cheapest <listing-url>` | Composes host extraction (from Airbnb/VRBO HTML) + web search (Parallel/DDG) + price scrape (direct site). Browser-only competitors exist; no CLI does this. The headline command. | 10/10 | shipping ⭐⭐⭐ |
| T2 | **Fee-breakdown comparison** | `compare <listing-url>` | Side-by-side: OTA total (fees in), direct total (no fees), savings $/percent. Requires local store join across listing+host+price. Only works because we already have all three. | 9/10 | shipping ⭐⭐ |
| T3 | **Cross-platform same-property match** | `match <listing-url>` | Find same property on the other platform via geocode + amenity + photo signal. The "is this listing on both?" detection is a key arb signal. | 8/10 | shipping ⭐⭐ |
| T4 | **Price-drop watchlist** | `watch add/list/check` | Local sync runs on cron, `watch check` exits 7 if any saved listing dropped under threshold. Cron-friendly. No competitor has this for vacation rentals. | 8/10 | shipping ⭐⭐ |
| T5 | **Host portfolio analysis** | `host portfolio <name>` | Aggregate every listing under one host (or PMC) across both platforms. 10+ listings = direct site likely exists. Drives `cheapest` heuristic. | 7/10 | shipping ⭐ |
| T6 | **"Plan a trip" composite** | `plan <city> --dates --guests --budget` | Search Airbnb + VRBO in parallel, run `cheapest` on top N, return ranked-by-savings list with direct URLs. One command, one trip planned. | 9/10 | shipping ⭐⭐ |
| T7 | **Wishlist diff over time** | `wishlist diff [--since <date>]` | Track price changes on saved listings. Local store joins wishlist + price history. Only works because we sync both. | 7/10 | shipping ⭐ |
| T8 | **Saved-listing sync** | `wishlist sync` | Pulls Airbnb wishlists into local store. Cross-device parity. Companion to T4/T7. | 6/10 | shipping |
| T9 | **Reverse image search for direct site** | `find-twin <listing-url-or-photo>` | hichee's magic button as a CLI. Uses Parallel image search or Tavily; degrades to text search on photo alt-text if no image backend. | 7/10 | shipping (degrades) |
| T10 | **Listing fingerprint hash** | `fingerprint <listing-url>` | Stable hash from photos + amenities + host + city. Used internally by T3 (match) but also exposed for power users to dedupe across exports. | 5/10 | shipping |

## Pluggable web-search backend

The `cheapest` and `find-twin` killer commands need web search. We support 4 backends with a single interface:

| Backend | Auth | Free tier | Quality | Image search |
|---|---|---|---|---|
| Parallel.ai | `PARALLEL_API_KEY` | None ($5/1k tasks) | Highest | Yes (Task API) |
| DuckDuckGo HTML | None | Unlimited (rate-limited) | Decent | No |
| Brave Search API | `BRAVE_SEARCH_API_KEY` | 2000 queries/month | High | Yes (separate endpoint) |
| Tavily | `TAVILY_API_KEY` | 1000 credits/month | High | Yes |

`--search-backend parallel\|ddg\|brave\|tavily` flag overrides the default. Default logic: highest-quality available given env vars; fall back to DDG.

## Stubs

None. Every shipping feature in this manifest gets full implementation in Phase 3. If implementation proves infeasible mid-build, we return to this gate per the skill rules — no quiet downgrades.

## Source-priority economics

- Free path (Airbnb search/get, VRBO search/get, `cheapest` via DDG, `compare`, `match`, `watch`, `host portfolio`, `plan`) requires **zero API keys**.
- Paid quality bumps: Parallel/Brave/Tavily for `cheapest` arbitration, image search for `find-twin`.
- Authenticated path (Airbnb wishlists, calendar): user runs `auth login --chrome` once.

## Reachability + auth summary

| Source | Mode (probe) | Auth Mode | Bot protection | Plan |
|---|---|---|---|---|
| Airbnb (SSR HTML) | standard_http | none | DataDome (avoided by SSR path) | Standard HTTP, openbnb pattern |
| Airbnb (GraphQL) | standard_http | cookie (auth login --chrome) | DataDome + Arkose | Optional, gated behind `auth login` |
| VRBO (GraphQL + SSR) | browser_http | none for read; Akamai cookies via warmup | Akamai bot manager + JA3 | **Surf-Chrome TLS impersonation** + GET / warmup → wait 1.5s → search/detail |
| Web-search backends | standard_http | varies | none | Per-backend client |

## Implementation notes (read by Phase 3)

- **Airbnb client:** SSR HTML scrape with `cheerio` Go equivalent (use `github.com/PuerkitoBio/goquery`). Walk `niobeClientData[0][1]` apollo cache. Robots.txt env-bypass: `AIRBNB_VRBO_IGNORE_ROBOTS_TXT=true` (default obeys).
- **VRBO client:** Surf with Chrome impersonation; warmup `GET https://www.vrbo.com/` → wait 1500ms → store `_abck`/`ak_bmsc` → POST graphql with cookies + headers (Origin, Referer, Sec-Fetch-*). Adaptive limiter targeting 1 req per 2-3s.
- **Per-source rate limiting:** Both Airbnb and VRBO clients MUST use `cliutil.AdaptiveLimiter` and surface `*cliutil.RateLimitError` (per AGENTS.md `source_client_check`).
- **Search-backend interface:** `internal/searchbackend/` package; `Backend` interface with `Search(ctx, query) ([]Result, error)` and `ImageSearch(ctx, photoURL) ([]Result, error)`. Pluggable; auto-select based on env.
- **Host extraction:** `internal/hostextract/` — order: PMC name (Airbnb listing brand badge / VRBO `propertyManagement.name`) > host display name > description-text bio > image-search fingerprint.
- **`cliutil.FanoutRun`:** Use for `plan` and `cheapest` (multi-source aggregation with per-source error collection).
- **Side-effect convention:** No commands open browser tabs by default. `--launch` flag opens direct site URL in browser; respects `PRINTING_PRESS_VERIFY=1` short-circuit.
- **MCP annotations:** All read-only commands get `mcp:read-only`. Mutation commands (`watch add`, `wishlist sync`) are open-world but not destructive. `auth login` stays exposed (interactive setup is acceptable).

## Spec-source declaration

This is a **synthetic spec** (`kind: synthetic` in the YAML). Multiple sources, hand-built commands intentionally go beyond the spec, dogfood path-validity-skipped, scorecard tier-2 excluded.
