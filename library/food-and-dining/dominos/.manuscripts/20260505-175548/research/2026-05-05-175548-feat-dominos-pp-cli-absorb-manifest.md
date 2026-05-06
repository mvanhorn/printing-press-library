# Domino's Pizza CLI Absorb Manifest (reprint — printing-press 3.9.1+pr639)

> Reprint of dominos-pp-cli. Prior absorb (31 features from 10 sources) carries forward verbatim — those features were already implemented in the prior CLI and the public-library spec captures the endpoint surface. This manifest's substantive change is the **transcendence layer**, which the novel-features subagent re-grounded against four named personas (Maya, Devon, Priya, Ace) for this run.

## Sources Cataloged
1. **apizza** (Go CLI) — harrybrwn/apizza — menu, cart, order, config
2. **node-dominos-pizza-api** (npm) — RIAEvangelist — stores, menu, order, tracking, payment, international
3. **pizzapi** (npm) — RIAEvangelist fork — stores, menu, order, payment
4. **pizzapi** (PyPI) — ggrammar — customer, address, store, menu, order, payment
5. **dominos** (PyPI) — tomasbasham — store, menu, basket, checkout (UK)
6. **mcpizza** (MCP) — GrahamMcBain — find_store, menu, add_to_order, customer, calculate_total
7. **pizzamcp** (MCP) — GrahamMcBain — order_pizza unified tool with 8 actions
8. **dominos-canada** (npm) — Canadian endpoint support
9. **ez-pizza-api** (npm) — simplified ordering wrapper
10. **Dominos GraphQL BFF** (sniff) — 24 operations including loyalty, deals, campaigns, upsells

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Find nearest store by address | apizza `config`, node-dominos NearbyStores | `stores find --address "421 N 63rd St, Seattle"` | Offline cache of visited stores, --json, distance sorting |
| 2 | Get store profile/details | node-dominos Store, dawg store.go | `stores get 7094` | SQLite-cached store data, hours check, capability flags |
| 3 | Store hours and availability | node-dominos store.info, sniff Store query | `stores hours 7094` | Formatted hours by day, open-now indicator, wait time estimate |
| 4 | Browse full menu by category | apizza `menu`, mcpizza get_menu | `menu browse --store 7094` | Offline FTS search, category filtering, --json |
| 5 | Search menu items by name | apizza `menu <category>`, pizzapi search | `menu search "pepperoni"` | FTS5 offline search across all items, toppings, descriptions |
| 6 | View item details with toppings | apizza `menu <code>`, node-dominos Item | `menu item S_PIZPH` | Full topping options with left/right/full, nutrition info |
| 7 | List toppings and codes | apizza `menu --toppings` | `menu toppings` | Searchable topping list with availability per item |
| 8 | Create new order/cart | apizza `cart new`, mcpizza add_to_order, sniff CreateCart | `cart new --store 7094 --service delivery --address "..."` | Named carts, --dry-run preview, SQLite persistence |
| 9 | Add item to cart | apizza `cart --add`, mcpizza add_to_order, sniff QuickAddProductMenu | `cart add S_PIZPH --size large --qty 2` | Topping syntax, quick-add by name, undo support |
| 10 | Remove item from cart | apizza `cart --remove`, pizzamcp remove_item | `cart remove <item-id>` | Undo buffer, item index reference |
| 11 | View cart contents | mcpizza view_order, sniff CartById | `cart show` | Formatted table with pricing, --json, item breakdown |
| 12 | Customize toppings | apizza topping syntax `P:full:2`, node-dominos Item options | `cart customize <item> --topping "P:left:1.5"` | Intuitive syntax: `--topping pepperoni:left:extra` |
| 13 | Validate order | node-dominos validate, dawg ValidateOrder, sniff CheckDraftDeal | `order validate` | Detailed validation errors, --json, fix suggestions |
| 14 | Price order | node-dominos price, dawg Price, sniff SummaryCharges | `order price` | Price breakdown: subtotal, tax, delivery fee, total |
| 15 | Place order | apizza `order`, node-dominos place, pizzamcp place_order | `order place --cvv 123` | --dry-run mandatory preview, confirmation prompt, receipt |
| 16 | Track order by phone | node-dominos Tracking, tracker endpoint | `track --phone 2065551234` | Real-time polling with status updates, progress bar |
| 17 | Customer setup | mcpizza set_customer_info, pizzapi Customer | `config set --name "Matt" --phone "..." --email "..."` | Stored in config, reused across orders |
| 18 | Address management | node-dominos Address, sniff Customer query | `address list` / `address add "421 N 63rd St"` | Multiple saved addresses, SQLite-backed |
| 19 | Payment management | node-dominos Payment, pizzapi PaymentObject | `payment add --card "..." --expiry "..." --zip "..."` | Encrypted local storage, multiple cards |
| 20 | Apply coupons | node-dominos Order coupons, sniff CheckDraftDeal | `cart coupon add <code>` | Auto-apply best deal, coupon stacking check |
| 21 | International support | node-dominos useInternational, dominos-canada | `config set --country canada` | US, Canada, custom endpoints |
| 22 | Config management | apizza `config set/get/--edit` | `config set/get/edit` | TOML config, env var override, per-profile support |
| 23 | List deals and coupons | sniff DealsList query | `deals list --store 7094` | All deals with pricing, eligibility, expiry dates |
| 24 | Loyalty points balance | sniff LoyaltyPoints query | `rewards points` | Current balance, pending points, account status |
| 25 | Available loyalty rewards | sniff LoyaltyRewards query | `rewards list` | Rewards by tier (20/40/60 pts), unlock status |
| 26 | Member-exclusive deals | sniff LoyaltyDeals query | `rewards deals` | Personal deals with expiry dates, --add to cart |
| 27 | Previous order suggestions | sniff PreviousOrderPizzaModal query | `orders recent` | Past orders for quick reorder |
| 28 | Upsell suggestions | sniff UpsellForOrder query | `cart suggest` | Contextual add-on suggestions based on cart contents |
| 29 | Account login/auth | dawg auth.go, power/login endpoint | `auth login` | OAuth token management, session persistence; PR #639 typed AuthEnvVar |
| 30 | Cart ETA | sniff CartEtaMinutes query | `cart eta` | Estimated wait time including prep and delivery |
| 31 | Menu categories list | sniff Category query | `menu categories --store 7094` | Category names, images, new-item indicators |

## Transcendence (only possible with our local data layer)

| # | Feature | Command | Why Only We Can Do This | Score | Persona |
|---|---------|---------|------------------------|-------|---------|
| 1 | Named order templates | `template save <name>` / `template order <name>` | SQLite `templates` table stores serialized cart (store, address, items, toppings, payment ref); `template order` rehydrates and runs validate→price→place | 9/10 | Maya, Ace |
| 2 | Smart reorder with substitution | `reorder --last --substitute-unavailable --dry-run` | Joins local `orders` (auth-synced history) with current `menu_items` FTS5 index; ranked substitution by name+category similarity for items the menu rotated out | 8/10 | Maya |
| 3 | Deal optimizer (with stacking) | `deals best --cart <name> [--stack]` | For each deal in DealsList (and loyalty rewards if logged in), call price-order with the deal applied; rank by total. `--stack` enumerates 2-3-deal combinations | 8/10 | Devon, Ace |
| 4 | Cross-store price comparison | `compare-prices --address ... --items ...` | For each store from store-locator, build identical cart and call price-order; rank by total (incl. delivery fee). Local fan-out only possible with synced multi-store menu | 7/10 | Devon, Priya |
| 5 | Multi-store wait-time scoreboard | `stores wait --address ...` | For each store in radius, fetch CartEtaMinutes (GraphQL BFF op) with a small dummy cart; sort by ETA. CartEtaMinutes is reverse-engineered and absent from every wrapper | 7/10 | Priya, Maya |
| 6 | Live delivery tracker with polling | `track --phone ... --watch --interval 30s` | Poll GetTrackerData every interval; emit one JSON line per status transition (placed→prep→bake→qc→out→delivered); exit 0 on delivered. node-dominos has single-shot tracker only | 8/10 | Maya, Ace |
| 7 | One-shot agent order | `order quick --template <name> --eta-watch --json` | Composes template-load → validate → price → place (gated on `--confirm`) → track --watch; emits final `{order_id, eta_min, total, tracker_phone}` JSON | 7/10 | Ace, Maya |
| 8 | Cart-shaped deal eligibility | `deals eligible --cart <name>` | Pulls DealsList; for each deal, locally checks deal predicates (qty / category / min-spend) against cart contents; tags reasons-for-fail per deal | 6/10 | Devon |
| 9 | Spending analytics | `analytics --period 90d --group-by item` | Aggregates synced order history into spend totals, frequency, favorite items, average order value | 6/10 (user-vouched) | (the user) — added back at Phase Gate 1.5 over the subagent's drop verdict; the user finds it "novel and neat" and the buildability is straightforward (single SELECT against the synced `orders` table) |

## Reprint Verdicts (prior → current)

| Prior feature | Verdict | Justification |
|---|---|---|
| Cross-store price comparison (`compare-prices`) | **Keep** | Devon + Priya both reach for it; scores 7/10 |
| Named order templates (`template save`) | **Keep** | Maya's defining ritual; scores 9/10 |
| Deal optimizer (`deals best`) | **Keep** + `--stack` flag | Devon's frustration verbatim; absorbs deal-stacking as a flag |
| Menu diff (`menu diff`) | **Drop** | No persona tracks menu changes weekly |
| Spending analytics (`analytics`) | **Keep (user-vouched at Phase Gate 1.5)** | Subagent dropped (no weekly persona); user overrode at Phase Gate 1.5 — finds it novel and neat. Restored to transcendence as #9. |
| Live delivery tracker (`tracking --watch`) | **Reframe** rename to `track --watch` | Persona fit is strong; rename to match brief recipes |
| Smart reorder with substitution (`reorder`) | **Keep** | Maya's frustration; scores 8/10 |
| Nutrition calculator (`nutrition`) | **Drop** | No persona; thin local sum with no compounding value |
| Bulk order builder (`order-bulk`) | **Drop** | Priya's core unmet need is wait-time visibility (#5), not multi-store CSV submit |
| Store health score (`stores health`) | **Reframe** to `stores wait` | Composite "health" was hand-wavy; useful ingredient is CartEtaMinutes wait time |

**New addition this run:** `order quick` (#7) — added because the user explicitly named "agent-shaped output" in their User Vision and the Ace persona's frustration is the exact gap.

**User override at Phase Gate 1.5:** `analytics` (#9) — restored over the subagent's drop verdict. The user found it novel and neat enough to keep; build cost is low (single SELECT against the synced `orders` table).

## Stubs

None. All 8 transcendence features are shipping-scope. Per the absorb-scoring rules, every approved feature ships as a working command, not a placeholder.

## Feature Counts
- Absorbed: 31 features from 10 tools (carry forward, generator-emitted)
- Transcendence: 9 novel features (8 from subagent cut + 1 user-vouched restore at Phase Gate 1.5)
- Total: 40 features
