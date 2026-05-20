# Booking.com CLI Absorb Manifest

## Source tools surveyed
- ZoranPandovski/BookingScraper (Python, 96★) — generic search-page scraper
- HexNio/booking_scraper (Python, 36★) — CLI scraper (city, country, --people, --datein, --dateout, -d detail, -l limit)
- dtrungtin/actor-booking-scraper (JS, 17★) — Apify actor for bulk extraction
- avkaz/Booking.com-python-api-spider (Python) — uses the hidden /dml/graphql endpoint with daily price tracking concept
- sudoknight/booking-reviews-scraper (Python, 28★) — CLI for reviews extraction
- gilbertekalea/booking.com_crawler (Python) — basic CSV scraper
- rukshar69/hotel-scraper (Python) — Playwright-based, scrapes booking.com + agoda
- azaelcodes/bookingcomclient (PHP) — partner-API-style client
- esakrissa/hotels_mcp_server (Python, 20★) — MCP server but RapidAPI-routed (not real booking.com)
- EmilyThaHuman/booking-mcp-server (TypeScript) — MCP with ChatGPT widgets, accommodation search
- Apify "Booking.com GraphQL Hotel Scraper" — exploits /dml/graphql, demonstrates CSRF pattern
- Scrapfly 2026 "How to Scrape Booking.com" + The Web Scraping Club "scraping Booking using internal APIs" — reference articles

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Hotel search by destination + dates + guests | ZoranPandovski, HexNio | `booking-com-pp-cli search --query Paris --checkin 2026-06-20 --checkout 2026-06-23 --adults 2` | SSR HTML extraction via data-testid; SQLite price-snapshot on every run; agent-native JSON; --select dotted-path field filter |
| 2 | Hotel detail page (amenities, address, photos) | avkaz, ZoranPandovski | `booking-com-pp-cli hotels get fr auliviaopera` | JSON-LD Hotel schema parse; structured types; cached in SQLite |
| 3 | Hotel reviews paginated with filters | sudoknight | `booking-com-pp-cli reviews list --slug auliviaopera --country fr --page 2 --language en --score-band wonderful` | Local cache of reviews; FTS5 over review text; CSV export |
| 4 | Destination autocomplete (text → dest_id, dest_type) | (no scraper does it cleanly) | `booking-com-pp-cli destinations autocomplete --query Paris` | Cached destination IDs in SQLite for reuse |
| 5 | Search filters: price range, stars, rating, free-cancellation, breakfast, distance | dtrungtin, Apify GraphQL Hotel Scraper | Composable `--min-price --max-price --stars 4 --score 8 --free-cancellation --breakfast` plus `--nflt` passthrough | Filters expressed as flags, not opaque nflt strings |
| 6 | Sort order | dtrungtin | `--order price\|distance\|review_score_and_price` | Same |
| 7 | Currency selection | All | `--currency USD` | Same |
| 8 | Language selection | All | `--lang en-us` | Same |
| 9 | Pagination | All | `--limit N` auto-paginates; or `--offset 25` for direct | Auto-paginate to user's --limit |
| 10 | CSV / JSON export | gilbertekalea, dtrungtin | Every command: `--json --csv` | Universal flag pattern |
| 11 | Map markers (lat/lon + price per pin) | Apify GraphQL Hotel Scraper | `booking-com-pp-cli map markers --query Paris --checkin --checkout` | /dml/graphql MapMarkersDesktop with auto-managed CSRF token |
| 12 | Authenticated trips list | (no scraper supports it) | `booking-com-pp-cli trips list --state upcoming` | Cookie auth via `auth login --chrome`; reads SSR HTML |
| 13 | Authenticated wishlist read | (no scraper supports it) | `booking-com-pp-cli wishlist get` | Auto-resolves wl_id; lists saved properties with last-seen prices |
| 14 | Authenticated Genius rewards / credit balance | (no scraper supports it) | `booking-com-pp-cli rewards get` | Parses rewards_and_wallet.html |
| 15 | Authenticated profile (name, tier, currency) | (no scraper supports it) | `booking-com-pp-cli profile get` | Parses mysettings.html |
| 16 | Local SQLite store + offline `sql` + offline FTS | avkaz mentions daily tracking, none ship | `booking-com-pp-cli sync` + `sql` + offline `search` | Real store-first; FTS5 over property descriptions + reviews |
| 17 | Doctor / health check | (none) | `booking-com-pp-cli doctor` | Checks cookie freshness, WAF clearance, CSRF validity |
| 18 | MCP server exposing every command | esakrissa (RapidAPI-routed), EmilyThaHuman | Every Cobra command auto-mirrored as MCP tool | stdio + http MCP transport; readOnlyHint annotations on read commands |
| 19 | Flights search (multi-airport, dates, cabin) | (no community CLI ships this against booking.com) | `booking-com-pp-cli flights search JFK CDG --depart 2026-06-20 --return 2026-06-27 --adults 1 --cabin economy` | SSR HTML extraction from flights.booking.com/flights/<ORIG>-<DEST>/; carrier + price + duration + layovers |
| 20 | Attractions search by city + dates | (no community CLI ships this) | `booking-com-pp-cli attractions search fr paris --from 2026-06-20 --to 2026-06-23` | SSR HTML extraction from /attractions/searchresults/<country>/<city>.html; price, rating, slug for detail |
| 21 | Attraction detail (description, inclusions, policy) | (no community CLI ships this) | `booking-com-pp-cli attractions get fr prjyfitkuhfz-one-day-admission-to-disneylandr-paris` | Full SSR detail page parse |
| 22 | Cars landing (suppliers + city pickup pages) | (no community CLI; Booking cars is JS-only) | `booking-com-pp-cli cars list` | Enumerates Hertz/Sixt/Avis/Budget supplier paths + city pickup paths; honest about no-deep-link-search limitation |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Cheapest-dates for one hotel | `prices cheapest --slug <s> --country <c> --window <s>..<e> --nights N` | 10/10 | hand-code | Iterates candidate checkins in --window, calls hotels/<country>/<slug>.html for each, writes (slug, checkin, checkout, currency, price, observed_at) into `price_history`, returns top-K cheapest rows | Brief Top Workflow #2; absent from every community scraper |
| 2 | Cheapest-dates for a destination | `prices cheapest-destination --query <city> --window <s>..<e> --nights N --max-price P` | 9/10 | hand-code | For each candidate checkin, calls /searchresults.html, stores each (property, checkin, price) tuple in `price_history`, joins to `properties`, returns cheapest under --max-price | Brief Top Workflow #2; no scraper aggregates across dates |
| 3 | Price-drop watch | `watch add` / `watch run` / `watch list` | 9/10 | hand-code | `watch add` stores (slug, country, checkin, checkout); `watch run` re-hits and appends to `price_history`; reports rows where latest >= --min-pct below trailing median | Brief Data Layer + Build Priorities; community scrapers lack persistence |
| 4 | Wishlist price-drop digest | `wishlist drops --since 7d --min-pct 5` | 9/10 | hand-code | Joins authenticated `wishlist_items` to `price_history`, surfaces items where latest >= --min-pct below previous observation | Brief Build Priorities explicit; no public scraper supports wishlist auth |
| 5 | Compare two hotels | `compare <slug1> <slug2> --checkin <d> --checkout <d>` | 8/10 | hand-code | Fetches hotel JSON-LD + reviewlist.html for both, joins to side-by-side struct (price, score, amenities Δ, distance, free-cancellation, breakfast, recent-review counts) | Brief Build Priorities; community scrapers never paired |
| 6 | Free-cancellation deadline alarm | `trips deadlines --within 7d` | 8/10 | hand-code | Walks authenticated mytrips.html, opens each trip-detail, parses free-cancellation-until timestamp, filters within --within of today, ranks by urgency | Service-specific Booking pattern; no scraper hits authenticated trips |
| 7 | Multi-leg itinerary planner | `trip plan --leg <city>:<in>:<out> --leg ... --budget <N> --filters <...>` | 8/10 | hand-code | Runs absorbed `search` per leg, picks cheapest property whose summed totals fit --budget; bounded combinatorial fallback (top-10 per leg) | Brief Top Workflow #1 + Persona Priya; no scraper runs multi-leg optimization |
| 8 | Genius-tier price-impact estimator | `genius impact --query <city> --checkin <d> --checkout <d>` | 7/10 | hand-code | Runs absorbed search twice (authenticated vs unauth), diffs price-per-property, returns Genius savings delta | Brief lists Genius as auth-only; the diff is the new value |
| 9 | Mobile-only rate detector | `deals mobile-rates --query <city> --checkin <d> --checkout <d>` | 6/10 | hand-code | Calls /searchresults.html with desktop UA and mobile UA, diffs to surface mobile-only discounts; both prices written to `price_history` | Scrapfly 2026 + avkaz reference mobile-rate diff without exploiting it |
| 10 | Destination price-band by month | `destinations price-band --query <city> --year <Y> --nights N` | 7/10 | hand-code | Local SQL aggregation over `price_history`; per-month median + min + max nightly rate plus property-count | Brief Data Layer + Top Workflow #2; seasonal waterfall is Booking content pattern |
| 11 | Trips export for expense systems | `trips export --state past --format csv --since <date>` | 6/10 | hand-code | Walks past trips, opens each detail, extracts (confirmation, property, checkin, checkout, currency, total, address), CSV with deterministic columns | Persona Marcus's weekly ritual |
| 12 | Offline FTS over synced properties | `search "<free text>"` | 6/10 | hand-code | FTS5 query over properties.name + description + amenities (after `sync`), BM25 ranking, no network call | Brief Build Priorities explicit |
| 13 | Review stats rollup | `reviews stats --slug <s> --country <c> --by score-band,language,traveler-type` | 5/10 | hand-code | Local SQL group-by over synced reviews; counts + median score per (score-band, language, traveler-type); mechanical, no NLP | Top Workflow #7; sudoknight lists never aggregates |

**Hand-code commitment:** 13 transcendence features, all tagged `hand-code`. Each requires ~50-150 LoC of Cobra command + parsing + SQL logic in `internal/cli/<command>.go`, plus `root.go` wiring (the latter survives regen via `regen-merge`).

**Stub disclosure:** None of the 13 transcendence features will ship as stubs. All 18 absorbed features ship fully implemented (the spec emits typed-tool endpoint commands for resources, and a thin `--limit/--csv/--json` flag-augmentation is generator-emitted).
