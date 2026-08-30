# Agoda CLI — Absorb Manifest

Scope: **hotel booking only** (user directive). Flights, activities, transfers, cars
are excluded even where endpoints were discovered.

## Process note
Step 1.5c.5 normally spawns a novel-features subagent. This session carries an explicit
standing instruction not to invoke the Agent tool unless the user requests it, so the
customer-model -> candidate -> adversarial-cut brainstorm was performed inline by the
main agent instead. Recorded here so the deviation is not silent.

## Tools surveyed
| Tool | Kind | Notes |
|---|---|---|
| birariro/agoda-review-mcp | MCP (Java) | Reviews only. hotelId extraction BROKEN (relies on removed `script-initparam`). Mislabels sort enum. |
| seanbabalala/hotelrate-crawl | MCP (Python) | Multi-OTA compare incl. Agoda. Requires Playwright (resident browser). |
| jiaweing/agoda-agent | CLI + agents | LLM-driven listing analysis. Needs an LLM key. |
| ScraperHub/agoda-property-listing-scraper | Scraper | Requires paid Crawlbase for JS render + CAPTCHA. |
| TyW-98/data-collection-pipeline | Scraper | Selenium. Params: location, start_date, nights, hotel count. |
| egeland/agodaparser | Parser | Agoda affiliate CSV; lookup by hotel ID / URL snippet. |
| Apify agoda-scraper (parseforge/bovi/datawebot/shahidirfan) | Paid actor | De-facto search-card field schema. |
| Agoda Partner API (developer.agoda.com) | Official | 4 modules: content, availability search, reservation create, post-booking. Partnership-gated. |
| booking-com-pp-cli | Peer printed CLI | Closest analog; full OTA feature set to match. |
| hotel-goat-pp-cli | Peer printed CLI | Google Hotels metasearch; wishlist + deep links. |

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | Destination text -> id resolution | Apify / all scrapers | `agoda-pp-cli destinations resolve` | Real `GetUnifiedSuggestResult`; returns cityId + type + property count; cached locally |
| 2 | Hotel search by dest + dates + occupancy | booking-com `hotels list` | `agoda-pp-cli hotels search` | Native `citySearch` GraphQL; 49 properties/call; both price bases in one shot |
| 3 | Filters (price ceiling, stars, score) | booking-com / Apify | `(behavior in agoda-pp-cli hotels search)` `--max-price --min-stars --min-score --free-cancel --breakfast` | Filters applied on TRUE all-in price, not teaser |
| 4 | Sorting | Apify / site | `(behavior in agoda-pp-cli hotels search)` `--sort` maps to `sorting.sortField/sortOrder` | Adds `--sort true-price` which the site cannot do |
| 5 | Pagination | booking-com offset | `(behavior in agoda-pp-cli hotels search)` `--limit --offset` | Bounded scan caps; `scanned_properties` in envelope |
| 6 | Property detail | booking-com `hotels get` | `agoda-pp-cli hotels get` | `propertyDetailsSearch`; amenities, images, coords, highlights, local info |
| 7 | Room/rate availability | Agoda Partner API "availability" | `agoda-pp-cli hotels rooms` | `propertyAllotmentSearch` availability calendar |
| 8 | Reviews, paginated + sorted | agoda-review-mcp | `agoda-pp-cli reviews list` | Correct sort enum (1/2/3/7), real pagination, per-review metadata; works where the MCP is broken |
| 9 | Review score breakdown | booking-com `reviews stats` | `agoda-pp-cli reviews stats` | Local SQL group-by over synced reviews |
| 10 | Currency + language control | site cookies | `(behavior in agoda-pp-cli hotels search)` `--currency --language` | Sets BOTH required currency fields; competitors set one and silently get geo-default |
| 11 | Deep booking link | hotel-goat / all | `(behavior in agoda-pp-cli hotels search)` `--json` includes `booking_url` | Preserves dates/occupancy/currency params |
| 12 | Similar destinations | site | `agoda-pp-cli destinations similar` | `SimilarCitySearch` |
| 13 | Saved / wishlist properties | booking-com `wishlist` | `agoda-pp-cli saved list` | `/bff/trips/save-to-trip/retrieve` with Chrome cookie import |
| 14 | Loyalty/VIP status | booking-com `rewards` | `agoda-pp-cli account` | AgodaVIP tier, AgodaCash, PointsMAX eligibility from authenticated SSR params |
| 15 | Multi-OTA price variance framing | hotelrate-crawl | `(behavior in agoda-pp-cli hotels search)` fee/markup breakdown | Same insight without Playwright; single-OTA but exact, not scraped |
| 16 | Local persistence + offline query | booking-com | `(generated endpoint)` sync + sql | SQLite mirror of properties/rates/reviews |
| 17 | Agent-native output | booking-com | `(behavior in agoda-pp-cli hotels search)` `--json --agent --select --compact` | Dotted-path selection over deep GraphQL nesting |
| 18 | Health check | booking-com `doctor` | `(generated endpoint)` doctor | Verifies reachability, cookie import, currency wiring |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | True all-in price by default | `hotels search --true-price` (default on) | hand-code | Agoda returns `exclusive` AND `inclusive` in one response but the site renders `exclusive`. We surface the real number plus the delta. Measured 21-30.3% gaps. | Use this for any "what will I actually pay" question. Do NOT quote the advertised rate; use the inclusive figure. |
| 2 | Honest re-ranking | `hotels rank` | hand-code | Markup ratio varies per property (21% vs 30.3%), so the teaser-cheapest is frequently NOT the all-in-cheapest. Re-sorting by true price changes the recommendation. Requires both prices for the whole result set at once. | Use this to pick a hotel on price. Do NOT use `hotels search` ordering for price decisions; it inherits Agoda's teaser ranking. |
| 3 | Fee-gouge detection | `hotels fees` | hand-code | Computes each property's tax+fee ratio and flags outliers vs the destination median. Directly answers the resort-fee complaint. | Use this to spot properties whose headline price hides an unusually large fee load. |
| 4 | Cheapest-date sweep | `prices cheapest` | hand-code | Native `priceTrendSearch` returns a 60-day per-property price curve in ONE call. Competitors would issue one search per date. | Use for flexible-date travelers. Returns the price floor across a window, not a single-date quote. |
| 5 | AgodaVIP delta | `vip delta` | hand-code | Runs the same search authenticated and anonymous, diffs per property. Quantifies what the user's VIP tier is actually worth on this specific search. | Use when deciding whether to log in / chase a VIP tier. Reports the real unlocked discount, not marketing copy. |
| 6 | Price-drop watch | `watch run` | hand-code | Local rate time-series; surfaces only properties whose latest true price dropped N% below trailing median. Stateless scrapers structurally cannot do this. | Schedule nightly. Returns empty most days; returns gold when a watched property actually drops. |
| 7 | Shortlist compare | `compare` | hand-code | Parallel detail+price fetch for 2-3 finalists, emitting true-price delta, score, amenity diff, cancellation terms. | Use when narrowed to finalists instead of re-reading two detail pages. |
| 8 | Offline corpus search | `search` | hand-code | FTS5 over synced property name/description/amenities. Cross-destination questions with no network call. | Use after `sync` for "which of my synced properties has X" questions. |

Minimum-5 requirement satisfied (8 proposed).

## Stubs
None proposed. Every row above is intended as shipping scope.

## Risk register (surfaced at the gate, not hidden)
- **`vip delta`** depends on the authenticated cookie path materially changing pricing.
  The request lever (`isUserLoggedIn`, `memberId`) is verified present; the magnitude of
  the delta was NOT measured during discovery. If the delta is always zero for this
  account's tier, the command still returns an honest "no VIP delta on this search".
- **`saved list` / `account`** were modelled from one observed `/bff/trips/...` call plus
  authenticated SSR `window.params` keys; a populated-state response was never observed.
- **PointsMAX-vs-AgodaCash valuation** was considered and CUT from shipping scope: the
  `pricing.pointmax` field exists but its earn-rate semantics were not verified during
  discovery, and shipping a wrong points valuation is worse than shipping none.
- **Rate limiting** is the live risk (Akamai + community reports of custom WAF + rate
  limiting). Adaptive limiter + typed rate-limit errors are mandatory, not optional.
