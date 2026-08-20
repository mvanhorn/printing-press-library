# OfferUp CLI — Absorb Manifest

**Run:** 20260531-200239 · **Binary:** `offerup-pp-cli` · **First print**

## Ecosystem scanned
| Tool | Type | What it offers |
|---|---|---|
| pyOfferUp (oscar0812) | Python scraper | get_listings(query,city,state), get_listings_by_lat_lon |
| planetzero/offerup + npm `offerup` 2.0 | Python/JS unofficial API | getFullListByQuery, getItem, getUserProfile, (auth: getMyProfile, offerPrice) |
| everettperiman/OfferupUnofficalAPI | Python | search params: radius, price_min/max, delivery, lat/lon, limit, sort |
| everettcaldwell/unofficial-offerup-api | Python GraphQL | GraphQL operations + types |
| Apify actors (lulzasaur, crowdpull, igolaizola, …) | SaaS scrapers | keyword+ZIP search, full details, condition/date filters, "no login" |
| npm `offerup-api` | npm (deprecated 8yr) | legacy client |
| gs-scraper (jgdigitaljedi) | multi-source | Craigslist/LetGo/OfferUp + eBay price compare |

**No OfferUp CLI, MCP server, or Claude skill exists.** This is the first agent-native, persistent OfferUp tool.

## Absorbed (match or beat everything that exists)
All headline reads are **unauthenticated** (`no_auth: true`), per user directive. Data via plain-HTTP SSR `__NEXT_DATA__` extraction.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword search w/ location | pyOfferUp / Apify | `offerup-pp-cli listings search` | offline SQLite store, FTS, --json/--select, dedup, no login, no paid key |
| 2 | Lat/lon + ZIP scoping | pyOfferUp get_listings_by_lat_lon | `(behavior in offerup-pp-cli listings search) --zip / --lat / --lon` | sets `ou.location` cookie; ZIP shorthand |
| 3 | Filters: radius, price min/max, condition, delivery, sort | everettperiman params | `(behavior in offerup-pp-cli listings search) --radius / --price-min / --price-max / --condition / --delivery / --sort / --limit` | enum-validated flags, typed exit codes |
| 4 | Item full detail (desc, photos, seller) | planetzero getItem / Apify | `offerup-pp-cli listings get` | full apollo extraction (description, photos, distance, shipping), cached to store |
| 5 | Seller/user profile + reputation | planetzero getUserProfile | `(behavior in offerup-pp-cli listings get) seller profile + reputation badges (BUSINESS/dealer/TruYou) returned on item detail` | OfferUp has no standalone seller page; seller rides on item detail, and `offerup-pp-cli seller-scan` aggregates a seller's inventory + reputation from the store |
| 6 | Category-scoped browse | Apify / explore pages | `(behavior in offerup-pp-cli listings search) --category <cid> scopes results to an OfferUp category` | category browse delivered as a search filter; a standalone taxonomy-list command is deferred |
| 7 | Pagination | scrapers limit/cursor | `(behavior in offerup-pp-cli listings search) --limit / --max-pages` | bounded via nextPageCursor |
| 8 | Persist listings to local store | (framework) | `offerup-pp-cli sync` | NEW for OfferUp: nobody else persists |
| 9 | Offline FTS / SQL over results | (framework) | `offerup-pp-cli search`, `offerup-pp-cli sql` | query stored listings offline, composable |

### Auth-gated (out of v1 scope per "prefer unauthenticated" directive)
Saved items, messages, my-listings, posting, making offers — require a logged-in session cookie. **Deferred.** Not stubbed in the shipped surface; can be added in a future opt-in pass. Listed here so the gate review is explicit that they are intentionally excluded.

## Transcendence (only possible with our approach — local SQLite + agent-native)
All six are `hand-code` (the agent's post-generate scope commitment). Source: novel-features subagent, all scored ≥5/10.

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| 1 | Local going-rate stats | `offerup-pp-cli price-check` | hand-code | median/p25/p75/min/max + firm-vs-negotiable ratio over stored listings for a query+area; no OfferUp page returns this |
| 2 | Below-median deal flagging | `offerup-pp-cli deals` | hand-code | joins each listing to the computed local median, emits those ≥N% under; `--markdowns` mode surfaces seller-declared originalPrice>price cuts |
| 3 | New-listing diff per query | `offerup-pp-cli new-since` | hand-code | time-windowed diff on stored first-seen/posted_at across sync runs; OfferUp only reshuffles its feed |
| 4 | Cross-sync price-drop detection | `offerup-pp-cli price-drops` | hand-code | joins `price_snapshots` for the same listingId across syncs, emits price cuts; no page gives per-listing history |
| 5 | Offline seller inventory + reputation | `offerup-pp-cli seller-scan` | hand-code | joins `sellers`↔`listings`: full inventory + listing count + median asking + badges in one shot |
| 6 | One-call daily deal report | `offerup-pp-cli digest` | hand-code | single composite read (new-since + price-drops + deals) → one agent-friendly payload / MCP call |

**Hand-code count: 6** (all transcendence). **Spec-emits: 0.** Absorbed reads (search/item/seller/categories) are hand-built SSR extractors backed by real OfferUp HTTP calls (`// pp:client-call`); sync/search/sql/analytics are framework-provided.
