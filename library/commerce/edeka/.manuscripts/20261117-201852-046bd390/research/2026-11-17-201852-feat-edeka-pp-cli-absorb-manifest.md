# EDEKA absorb manifest

Tools found that touch the edeka.shop (maexware/OXID) shops: none (web search 2026-11-17: no CLI, MCP server, SDK, npm or PyPI package). The absorbed rows therefore come from the shop's own web UI and from the sibling REWE CLI the OneDone agent already uses (same agent workflow, same command vocabulary).

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Market search by postal code or town | edeka.shop market finder | edeka-pp-cli markets search | JSON, pickup/delivery flags, delivery postal codes, no login |
| 2 | Select a market | edeka.shop "Markt ändern" | edeka-pp-cli markets select | Stored in config; every command works on that market |
| 3 | Current market | shop header | edeka-pp-cli markets current | Shows market host and login state |
| 4 | Weekly offers | /Angebote/ | edeka-pp-cli offers | Category filter, paging, JSON, recorded for price history |
| 5 | Product search with prices | search page | edeka-pp-cli products search | Price in cents, unit price, offer and weight flags |
| 6 | Product detail | product page | edeka-pp-cli products show | schema.org fields (sku, price, availability, description) |
| 7 | Categories | navigation tree | edeka-pp-cli categories | Flat list with paths for agents |
| 8 | Search suggestions | elastic suggester | (generated endpoint) suggest products | Raw suggester JSON |
| 9 | Basket show | basket page | edeka-pp-cli basket show | Items, quantities, totals as JSON |
| 10 | Basket add by product id | changeBasket | edeka-pp-cli basket add | Adds on top of the current quantity; --dry-run |
| 11 | Basket set quantity | changeBasket (absolute) | edeka-pp-cli basket set | --dry-run |
| 12 | Basket remove | changeBasket am=0 | edeka-pp-cli basket remove | --dry-run |
| 13 | Basket clear | changeBasket per item | edeka-pp-cli basket clear | --dry-run, --confirm not needed (reversible) |
| 14 | Slots list (pickup) | calendar widget | edeka-pp-cli slots list | Embedded JSON parsed into days and free slots |
| 15 | Slot reserve | mx_pickup_controller setCookie | edeka-pp-cli slots reserve | --dry-run; reservation cookie persisted |
| 16 | Payment options | payment step | edeka-pp-cli payment options | Ids and labels |
| 17 | Order history | account_order | edeka-pp-cli order list | JSON |
| 18 | Cookie session import | browser login | edeka-pp-cli auth import | CDP (loopback only) or cookie file |
| 19 | Session status | account page | edeka-pp-cli auth session | Logged-in check, remember-me expiry |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Basket by text | basket add "hafermilch" | hand-code | 8/10 | Resolves free text to the best-matching article (search + ranking) and adds it in one call - what a voice note needs | Use this command to add an item by product id or by free text. Do NOT use it to add a whole list; use 'list add' instead. |
| 2 | Shopping list to basket | list add milch butter "2x kaffee" | hand-code | 7/10 | Runs best-match resolution for many lines with quantities and reports what matched and what did not | Use this command to add several free-text lines at once. Do NOT use it for one item; use 'basket add'. |
| 3 | Earliest pickup slot | slots earliest | hand-code | 8/10 | Parses the calendar's embedded JSON and returns the first free slot - the question the agent always asks | none |
| 4 | Checkout with confirmation gate | order place | hand-code | 8/10 | Walks slot -> address -> payment -> review over HTTP, shows the preview, and only executes with --confirm | Use this command to preview or place the order. Do NOT use it to list past orders; use 'order list'. |
| 5 | Offer check for staples | deals check milch kaffee | hand-code | 7/10 | One call answers "which of these items are on offer this week" from offer-badged search hits | Use this command for "are these items on offer". Do NOT use it to browse all offers; use 'offers'. |
| 6 | Price history | price history 5915407005MXST | hand-code | 7/10 | Every search and offers call records prices in local SQLite; the history shows min/max and offer prices over time | none |
| 7 | Compare markets | compare markets milch --market a --market b | hand-code | 6/10 | Same search on several independent markets (public layer), side by side | Use this command to compare one product across markets. Do NOT use it to search one market; use 'products search'. |

Killed (from the brainstorm, see novel-features-brainstorm.md): wishlist store (deals check covers it without a store), deals since (needs ~40 pages of 2.7 MB per snapshot), basket audit (one 2 MB product page per item), order reorder (order history empty on the test account: not verifiable), slots wait, weights list, delivery check, payback estimate, basket fit, plan meals.

### Stubs
- None. Order cancel is not offered by the shop UI (no cancel control in the order history template; not verifiable without an order) - not built, documented.
- Recipes: the shop has none - not built.

### Approval
Autonomous run: the build owner's task lists the command scope (markets, offers, search, categories, basket, slots, order place with --confirm, order list/show, auth import/session, price history if cheap). This manifest matches that scope; treated as approved.
