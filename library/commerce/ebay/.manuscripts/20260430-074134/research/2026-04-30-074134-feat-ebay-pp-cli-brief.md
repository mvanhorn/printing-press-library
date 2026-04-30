# eBay CLI Brief

## API Identity
- Domain: world's largest C2C / B2C marketplace; 1.7B active listings, ~135M active buyers.
- Users: buyers (search, watch, bid, snipe, comp), sellers (list, fulfill, message). This CLI targets **buyer power users** first; seller features are absorbed as table stakes but not the headline.
- Data profile: items (active + sold), search results, watchlists, saved searches, bids, best offers, categories/taxonomy, deals.

## Reachability Risk
- Low for public Browse / Deal / Commerce APIs (open with App OAuth; no rate-limit hostility at typical CLI volume; 5,000 calls/day cap matters at scale only).
- Medium for sold-comps via `/sch/i.html?LH_Sold=1`. CAPTCHA can fire on aggressive scraping; community consensus is keep <20 req/min/IP. Surf with a Chrome TLS fingerprint plus the user's Chrome cookies works in 2026.
- Low for browser-session endpoints (watchlist, bid, best-offer) when reusing the user's logged-in cookies + CSRF tokens.

## Top Workflows
1. **Comp before bid** — given a free-text item title, return average / median / min / max sold price over last 90 days, with smart matching, condition normalization, and outlier trim. Decide whether to bid.
2. **Snipe a target auction** — schedule a max-bid that fires through the user's web session at T-N seconds, hidden from other bidders until placed.
3. **Watchlist intelligence** — flat eBay watchlist becomes folders + tags + price-threshold alerts + ending-soon alerts.
4. **Saved-search-to-feed** — power user defines structured queries; new matches stream in, deduplicated, with sold-comp context attached.
5. **Best-offer hunter** — across a saved search of fixed-price-with-best-offer items, auto-suggest (or auto-submit) offers at N% of asking, track which offers get accepted, learn the seller's floor.

## User Vision
> "I want to know what a [specific item, e.g. Cooper Flagg gold /50 Topps Chrome] has sold for in the last 90 days. Today I have to go to sold items, search, verify match, eyeball average. I want one command with smart matching that gives me the average price so I know what I should pay if I'm going to bid."
>
> "True sniper bidding — agent holds a secret max client-side, places the bid manually in the final seconds. eBay's proxy bidder never sees my max until T-25s, and other bidders never see it until it's too late to react."
>
> Domain-agnostic: works for cards, watches, electronics, vintage, anything.

## Auth Strategy
**Browser-session-primary, App-OAuth-optional.**
- Primary: Chrome cookie capture (`auth login --chrome`). Unlocks search/item HTML, sold-comps via /sch/i.html, watchlist read/write, bid placement, best-offer submission, saved searches, My eBay buying history.
- Optional accelerator: `EBAY_APP_ID` + `EBAY_CERT_ID` for client-credentials OAuth. Unlocks Browse API JSON (richer item fields, faster, no CAPTCHA risk), Deal API, Commerce/Taxonomy.
- Scopes are independent: every command works with browser session alone; App-OAuth-only is also a valid path for the read-only public surface.

## Data Layer
- **Primary entities (SQLite)**: `items` (item id, title, price, currency, condition, seller, listing type, end_time, watch_count, bid_count, image_url), `sold_items` (same shape + sold_price + sold_date + best_offer_accepted), `watches` (item_id, folder, tags, price_threshold, end_alert_minutes, added_at), `saved_searches` (id, query_json, last_run, last_seen_max_id), `bids` (id, item_id, max_amount, lead_seconds, status, fired_at, group_id), `bid_groups` (id, name, type [single-win|multi-win|contingency], item_ids).
- **Sync cursor**: per saved search → last_seen_listing_time; per watchlist → last_synced; per sold-comp query → cache_until.
- **FTS5**: items.title, items.subtitle, sold_items.title.

## Codebase Intelligence
- Source: hendt/ebay-api SDK (TypeScript) + ebaysdk-python (XML) + ruippeixotog/ebay-snipe-server (Scala)
- Auth: OAuth2 client-credentials Bearer for Buy/Browse; legacy Auth'N'Auth or modern user-OAuth for Trading; **session cookies + CSRF tokens** for web flows (login.ebay.com sets `cid`, `nonsession`, `dp1`, `s` cookies; bid forms include `_csrf` hidden inputs).
- Data model: Browse API returns `itemSummaries[]` with `itemId`, `legacyItemId`, `price.value`, `price.currency`, `condition`, `seller.username`, `seller.feedbackPercentage`, `itemEndDate`, `bidCount`, `currentBidPrice`, `image.imageUrl`. Sold-page DOM uses `.s-item__price`, `.s-item__title`, `.s-item__caption--row.s-item__caption--signal.POSITIVE` for sold date.
- Rate limiting: 5,000 calls/day default, per-app; 429 with `Retry-After` header. Web scraping caps at ~20 req/min/IP before CAPTCHA.
- Architecture key insight: `/sch/i.html?LH_Sold=1&LH_Complete=1` is the only path to 90-day sold history for non-partner devs; the modern Marketplace Insights API is partners-only and has been since 2020.

## Source Priority
- Single-source CLI (eBay). No multi-source ordering needed.

## Product Thesis
- **Name**: `ebay-pp-cli` (binary), `eBay` (display)
- **Why it should exist**: There is no Go CLI for eBay; the existing JS/Python wrappers are libraries, not CLIs; every existing CLI is either active-search-only or seller-side. Buyers — the larger and more underserved population — have zero programmatic access to the three things they need most: (1) sold comps, (2) sniper bidding, (3) watchlist intelligence. The two killer features (sold-comp + sniper) are explicitly blocked from public APIs (Marketplace Insights and Offer API are partners-only), so every CLI sidesteps them. We don't.
- **Tagline candidate**: "Every eBay buyer feature a power user wants, plus a local sold-comp brain and a sniper that doesn't tell eBay your max."

## Build Priorities
1. **P0 (foundation)**: SQLite store for items / sold_items / watches / saved_searches / bids / bid_groups; FTS5 over titles; Chrome-cookie auth import (`auth login --chrome`); HTML scraper for /sch/i.html with LH_Sold variants; Browse API client gated on App-OAuth.
2. **P1 (absorb)**: every Browse API endpoint, search-by-image, Deal API, Trading API GetMyeBayBuying for watchlist / bids / best-offers / won / lost, Browse + sold search with all filters, watchlist add/remove via web session, place-bid via web session, best-offer submission via web session, saved-search CRUD, item detail get, taxonomy lookup.
3. **P2 (transcend, the differentiators)**:
   - `comp <query> [--days 90] [--condition raw|graded] [--outlier-trim]` — sold-comp intelligence with smart matching, condition normalization, trendline.
   - `snipe <item-id> --max <amount> [--lead 25s] [--group <name>]` — true sniper, max held client-side, fires at T-N seconds via web session.
   - `bid-group create <name> --type single-win|multi-win=N|contingency` — coordinated multi-item snipes.
   - `watch tag/folder/threshold` — watchlist with tags, folders, price-threshold alerts, end-of-auction alerts.
   - `feed <saved-search>` — new-listings stream with sold-comp context attached.
   - `offer-hunter <saved-search> --at-percent 80` — auto-submit offers across a saved search.
   - `comp image <path>` — sold-comp lookup by image (combines searchByImage with sold-page scrape).
   - `bestoffer-history <item-id>` — Best Offer accepted price recovery (130point-class).
4. **P3 (polish)**: scheduled jobs for snipe-fire, watchlist sync, saved-search refresh; agent-native output for every command; rate-limit-aware Surf transport with Chrome TLS fingerprint.
