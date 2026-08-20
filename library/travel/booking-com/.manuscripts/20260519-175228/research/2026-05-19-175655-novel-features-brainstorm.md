# Novel-features Brainstorm Audit Trail: booking-com

Returned by general-purpose subagent on 2026-05-20.

## Customer model

**Persona 1: Lena, the "cheapest-dates" leisure traveler**

- **Today (without this CLI):** Lena has 8-10 Booking.com tabs open during planning sprees, each one with slightly different checkin/checkout dates for the same destination. She manually clicks through 4-7 candidate dates ("June 18 weekend? June 25? Shoulder of July 4?"), screenshots the totals, and pastes them into a Notes app. When she sees a hotel she likes, she stars it on the wishlist and re-checks the wishlist page every few days hoping the price moved.
- **Weekly ritual:** Wednesday and Saturday evenings — open Booking.com, re-run her three saved destination searches against three to five candidate date pairs, eyeball whether anything dropped versus what she screenshotted last week, decide whether to book.
- **Frustration:** She cannot answer "did this hotel get cheaper than the last time I looked, and by how much?" without scrolling back through her own screenshots. There is no calendar view of price-by-date for a single hotel.

**Persona 2: David, the wishlist deal-hunter**

- **Today (without this CLI):** David curates a wishlist of 30-40 properties across cities he wants to visit "if the price is right." He opens www.booking.com/mywishlist.html?wl_id=X once or twice a week, manually eyeballs whether the displayed nightly rate looks lower than what he remembers, and re-clicks each property when something looks promising. He has no log of past prices.
- **Weekly ritual:** Sunday morning — open wishlist, scroll, flag anything that "feels cheaper," click into 3-5 properties for the real total, give up on the rest.
- **Frustration:** The wishlist page tells him today's price but not whether it dropped. He has no idea which of his 40 properties saw the biggest swing since he last looked.

**Persona 3: Priya, the multi-city itinerary planner**

- **Today (without this CLI):** Priya is planning a 10-day Italy trip across Rome, Florence, and Venice. She has three SearchResults tabs open, fiddles with checkin/checkout dates so the legs chain correctly (Rome 3 nights, Florence 3 nights, Venice 4 nights), filters each for "free cancellation + breakfast + 8+ score," and tries to keep all three nightly totals roughly proportional to her per-leg budget. When she switches Florence from 3 nights to 4 to extend a leg, she has to redo Venice from scratch.
- **Weekly ritual:** Friday evening planning session — open three searches, lock in three properties, screenshot, iterate next weekend with date shifts.
- **Frustration:** No way to ask "given these three legs, what are the cheapest free-cancellation options for each, sorted so the per-leg totals fit my budget?" She does this math by hand.

**Persona 4: Marcus, the business traveler tracking his own bookings**

- **Today (without this CLI):** Marcus books his own travel through Booking.com (his employer reimburses). He has 6-9 trips in various states on mytrips.html — some upcoming, some past awaiting expense reports. He logs into Booking each Monday to grab confirmation numbers, totals, hotel addresses, and cancellation deadlines for the expense system and his calendar. When a free-cancellation deadline is creeping up on a trip he's not sure about, he has to manually click each trip to see the deadline.
- **Weekly ritual:** Monday morning — open mytrips.html, click through each upcoming trip, copy confirmation + total + address into expense tool and calendar.
- **Frustration:** No single view of "which of my upcoming trips have a free-cancellation deadline in the next 7 days?" He's been double-charged before because he missed a deadline by a day.

## Candidates (pre-cut)

(See Pass 2 table in earlier brainstorm output — 16 candidates spanning persona-driven, service-specific Booking patterns, cross-entity local queries. C15 killed inline as a wrapper. C5 + C11 killed in Pass 3 cut.)

## Survivors and kills

### Survivors (final list, 13 features, all hand-code)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Cheapest-dates for one hotel | `prices cheapest --slug <s> --country <c> --window <start>..<end> --nights N` | 10/10 | hand-code | Iterates candidate checkin dates in --window, calls hotel/<country>/<slug>.html for each, writes (slug, checkin, checkout, currency, price, observed_at) into local `price_history` SQLite, returns top-K cheapest rows | Brief Top Workflow #2 names it; absent from ZoranPandovski/BookingScraper, HexNio, dtrungtin, avkaz, sudoknight; Booking UI requires manual per-date clicks |
| 2 | Cheapest-dates for a destination | `prices cheapest-destination --query <city> --window <s>..<e> --nights N --max-price P` | 9/10 | hand-code | For each candidate checkin, calls /searchresults.html for the destination + filters, stores every (property, checkin, price) tuple in `price_history`, joins to `properties`, returns the cheapest (property, date) pairs under --max-price | Brief Top Workflow #2 + Build Priorities "Transcend"; no scraper aggregates across dates |
| 3 | Price-drop watch | `watch add` / `watch run` / `watch list` | 9/10 | hand-code | `watch add` writes (slug, country, checkin, checkout) into `watches`; `watch run` re-hits hotel/<slug>.html, appends to `price_history`, reports rows where latest price is >= --min-pct below trailing median | Brief Data Layer explicitly names `price_history`; Build Priorities lists price-drop-watch; community scrapers all lack persistence |
| 4 | Wishlist price-drop digest | `wishlist drops --since 7d --min-pct 5` | 9/10 | hand-code | Joins authenticated `wishlist_items` to `price_history` (populated by repeated `wishlist get` runs), surfaces items whose latest price is >= --min-pct lower than the previous observation in --since window | Brief Build Priorities calls out "wishlist drop alert" explicitly; no public scraper or RapidAPI MCP supports wishlist auth, let alone diff |
| 5 | Compare two hotels | `compare <slug1> <slug2> --checkin <d> --checkout <d>` | 8/10 | hand-code | Fetches hotel/<country>/<slug>.html JSON-LD for both, fetches reviewlist.html score bands for both, joins to side-by-side struct (price, score, amenities Δ, distance, free-cancellation, breakfast, count of recent reviews scored > 8 vs < 6) | Brief Build Priorities lists `compare` explicitly under Transcend; community scrapers all return single-hotel detail, never paired |
| 6 | Free-cancellation deadline alarm | `trips deadlines --within 7d` | 8/10 | hand-code | Walks authenticated mytrips.html, opens each trip-detail page, parses the free-cancellation-until timestamp, filters to those whose deadline is within --within of today, returns ranked by urgency | Booking's free-cancellation window is a service-specific identity feature; no scraper hits authenticated trips at all |
| 7 | Multi-leg itinerary planner | `trip plan --leg <city>:<in>:<out> --leg ... --budget <N> --filters <...>` | 8/10 | hand-code | Runs absorbed `search` per --leg, applies filters, picks cheapest property per leg whose summed nightly total fits --budget; falls back to next-cheapest combinations when greedy pick busts budget (bounded N=top-10 per leg, ~1000 combos worst case) | Brief Top Workflow #1 + Persona Priya's frustration; no scraper or MCP runs multi-leg optimization |
| 8 | Genius-tier price-impact estimator | `genius impact --query <city> --checkin <d> --checkout <d>` | 7/10 | hand-code | Runs absorbed `search` twice — once with authenticated cookie (Genius rates honored) and once without — diffs price-per-property, returns saved-by-Genius delta per property and total | Brief lists Genius as auth-only surface; diff against unauthenticated pricing is service-specific identity feature no scraper has access to |
| 9 | Mobile-only rate detector | `deals mobile-rates --query <city> --checkin <d> --checkout <d>` | 6/10 | hand-code | Calls /searchresults.html with desktop UA and again with Booking's mobile-app UA, diffs price-per-property to surface mobile-only discounts; writes both prices to `price_history` so subsequent diffs are cheap | Service-specific Booking pattern; Scrapfly 2026 blog and avkaz both reference mobile-rate differential without exploiting it |
| 10 | Destination price-band by month | `destinations price-band --query <city> --year <Y> --nights N` | 7/10 | hand-code | Local SQL aggregation over `price_history` for the destination's properties — groups by month, emits median + min + max nightly rate per month plus property-count contributing to each row | Brief Data Layer + Top Workflow #2; seasonal price waterfall is Booking content pattern no scraper exposes |
| 11 | Trips export for expense systems | `trips export --state past --format csv --since <date>` | 6/10 | hand-code | Walks authenticated past-trip list, opens each trip detail, extracts (confirmation, property, checkin, checkout, currency, total, address), emits CSV with deterministic column order | Persona Marcus's weekly ritual; no scraper supports authenticated trip detail, let alone structured export |
| 12 | Offline FTS over synced properties | `search "<free text>"` | 6/10 | hand-code | After `sync`, FTS5 query over properties.name + description + amenities, ranked by BM25; no network call | Brief Build Priorities lists "offline property search via FTS" explicitly; Top Workflow #1 implies free-text search is core |
| 13 | Review stats rollup | `reviews stats --slug <s> --country <c> --by score-band,language,traveler-type` | 5/10 | hand-code | Local SQL group-by over `reviews` synced for the property; emits counts + median score per (score-band, language, traveler-type) bucket; mechanical, no NLP | Top Workflow #7 + sudoknight/booking-reviews-scraper (28★) only lists, never aggregates; mechanical reframe of "summarize reviews" per rubric LLM rule |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C5: Price-history calendar | Pure rendering layer over the data the cheapest-dates sweep already produces; agent users pipe `prices cheapest --json` into whatever calendar surface they want — no transcendence beyond C1 + C16. | C1 `prices cheapest` |
| C11: Recent-search analytics | Sub-weekly use for personas Lena and David — they don't audit their own search log, they just re-run it. Smells like internal CLI logging dressed as a feature; failed rubric question 1 (weekly use). | C3 `watch run`, the actual repeated-search workflow rendered as state. |
| C15: Sustainability filter | Thin flag passthrough on the already-absorbed `search` command; failed rubric question 2 (wrapper vs leverage). Better expressed as a `--nflt` arg on absorbed search #5. | Absorbed #5 (search filters) |
