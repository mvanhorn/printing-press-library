# foodpanda CLI — Absorb Manifest

Landscape searched: Claude plugins, MCP registries (lobehub/mcpmarket/fastmcp), GitHub CLIs,
npm, PyPI, Apify actor store, commercial scraping vendors.

**There is no foodpanda MCP server, no foodpanda CLI, and no maintained open-source client.**
The entire competitive field is (a) paid hosted scraper actors and (b) abandoned Python scripts.
Nothing is local-first, nothing is agent-native, nothing has an offline store.

## Source tools

| Tool | Type | Notes |
|---|---|---|
| `scrapesage/foodpanda-scraper` (Apify) | paid hosted | restaurants, menus, prices; SG/BD/PK/HK/MY |
| `fatihtahta/food-panda-scraper` (Apify) | paid hosted, $5/1k | "all in one" |
| `nowi5/apify-foodpanda-restaurants` | **deprecated** | by URL or location; nested menus |
| `vedingal/WebScrapingUsingPython_FoodPanda…` | GitHub script | HTML table menu scrape |
| `iCHAIT/oloviz` | GitHub script | personal order-history scrape |
| foodspark / fooddatascrape / iwebscraping / 3idatascraping | commercial services | restaurant + menu + review datasets |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Restaurant search near a location | Apify scrapesage | `foodpanda-pp-cli vendors list` | Offline store, `--json`, typed exits, no per-run fee |
| 2 | Text search for restaurants/dishes | Apify actors | `foodpanda-pp-cli search` | Uses the **real** `/search?query=` endpoint, not the ignored `q=` |
| 3 | Restaurant detail (address, geo, images) | Apify scrapesage | `(generated endpoint) vendors get` | Typed, cached, agent-native |
| 4 | Full nested menu + prices | Apify scrapesage | `foodpanda-pp-cli menu` | Persisted to SQLite, FTS-indexed, diffable |
| 5 | Menu categories + product variations | `nowi5` actor | `(behavior in foodpanda-pp-cli menu) --flat / --category` | Flattened or nested output, CSV-able |
| 6 | Ratings + review counts | Apify actors | `(behavior in foodpanda-pp-cli vendors list) rating fields` | Sortable/filterable across whole city |
| 7 | Reviews text + author + date | commercial scrapers | `foodpanda-pp-cli reviews` | Dedicated reviews API w/ **per-topic** scores + vendor replies |
| 8 | Cuisine filtering | Apify actors | `(behavior in foodpanda-pp-cli vendors list) --cuisine` | Facet list from live `aggregations` |
| 9 | Sort by rating / distance / delivery time | foodpanda web | `(behavior in foodpanda-pp-cli vendors list) --sort` | All three verified to genuinely reorder |
| 10 | Delivery fee / min-order / ETA | Apify actors | `(behavior in foodpanda-pp-cli vendors list) fee fields` | Whole-area comparison, not one vendor at a time |
| 11 | Opening hours | commercial scrapers | `(behavior in foodpanda-pp-cli vendors get) --hours` | From JSON-LD `openingHoursSpecification` |
| 12 | Deals / discounts | Apify actors | `foodpanda-pp-cli deals` | Cross-vendor deal board |
| 13 | Multi-country coverage | Apify scrapesage | `(behavior in foodpanda-pp-cli --country) pk/bd/sg/my/hk` | Verified live on 5 markets |
| 14 | Pagination over large listings | Apify actors | `(behavior in foodpanda-pp-cli vendors list) --limit/--max-scan-pages` | Bounded scan + `scanned_vendors` in JSON |
| 15 | Darkstore / q-commerce listing | — | `(behavior in foodpanda-pp-cli vendors list) --vertical darkstores` | Honest: catalog is a separate surface, disclosed |
| 16 | Saved addresses | `oloviz` (order scrape) | `foodpanda-pp-cli addresses` | Verified auth endpoint, feeds `home` resolution |
| 17 | Sync to local database | — (nobody does this) | `foodpanda-pp-cli sync` | The foundation everything below stands on |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|-------------------------|------------------|
| 1 | Delivery-fee board from your home address | `home board` | 10 | hand-code | Requires joining your authenticated saved address to a full-area vendor sweep. The web UI shows fees one card at a time and never lets you rank the whole city by what delivery actually costs you. | Use this to answer "what does delivery cost me from home, everywhere". Do NOT use it for a single known restaurant; use `vendors get`. |
| 2 | Cross-restaurant dish price comparison | `dish` | 10 | hand-code | Requires FTS over every product of every synced vendor. foodpanda has no cross-vendor dish search — the API only returns one vendor's menu per call. | Use this to find the cheapest/best-rated source of a specific dish nearby. Do NOT use it for restaurant-name lookup; use `search`. |
| 3 | Menu + price drift over time | `menu-diff` | 9 | hand-code | Requires historical menu snapshots in SQLite. The API is stateless and exposes no price history whatsoever. | Use this to detect price rises, item removals, or new deals between two syncs. |
| 4 | Commercial posture / ad-spend signals | `posture` | 9 | hand-code | Requires reading `ncr_pricing_model`, `is_promoted`, `premium_position`, `vendor_points`, `delivery_provider` across every vendor at once and ranking them. No foodpanda surface aggregates this. | Use this for competitive analysis of which vendors buy CPC ads and premium placement. Does NOT report merchant commission rates — those are not in any consumer surface. |
| 5 | Who actually delivers to a point | `coverage` | 8 | hand-code | Requires per-vendor `areaServed.geoRadius` from JSON-LD joined against a target coordinate. Nothing on the site answers "does this place reach my office". | Use this to test delivery reach for an arbitrary address. |
| 6 | Fee / MOV / service-fee comparison board | `fees` | 8 | hand-code | Requires a local join across `minimum_delivery_fee`, `minimum_order_amount`, `service_fee_percentage_amount`, `vat_percentage_amount` for a whole area. | Use this to compare the true cost structure of ordering, not just headline delivery fee. |
| 7 | Review topic breakdown | `reviews digest` | 7 | hand-code | Requires aggregating per-topic scores (`overall` vs `restaurant_food` vs rider) across paginated reviews. The site shows one blended star. | Use this to separate "food is bad" from "delivery is bad". |
| 8 | Cross-market comparison | `market-compare` | 7 | hand-code | Requires fanning the same query across pk/bd/sg/my/hk and normalizing currency/labels in one local table. | Use this for regional benchmarking across foodpanda markets. |
| 9 | Search honesty / match confidence | `(behavior in foodpanda-pp-cli search) --explain` | 6 | hand-code | foodpanda's search is fuzzy and never returns empty (`zzzqqqnonsense` → 18 hits). We score and label weak matches so agents don't treat noise as signal. | Use this when you need to know whether a search actually matched or just fell back. |

**9 transcendence rows, all hand-code.** Generated endpoint commands cover the typed API surface.

## Explicitly out of scope

- **Cart / checkout / ordering** — user chose authenticated reads only. No money is spent.
- **Merchant commission rates** — not present in any consumer surface (1,881 key paths searched;
  GraphQL safelisted). See discovery report. `posture` ships the observable proxies instead.
- **Web order history** — no such route exists (`/orders` and `/account/orders` both 404).
  Only `/api/v5/orders/reorder` carries reorder data.
- **pandamart grocery catalog** — `vertical=darkstores` lists the store but returns `menus: 0`;
  the product catalog is a separate q-commerce surface not reachable from `/api/v5/vendors`.
