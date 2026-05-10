# OrderToGo CLI Absorb Manifest

## Ecosystem search

- WebSearch / WebFetch for: `"OrderToGo.com" Claude plugin`, `"OrderToGo.com" MCP server`, `"OrderToGo.com" CLI site:github.com`, `"OrderToGo.com" SDK site:npmjs.com`, `"OrderToGo.com" client library site:pypi.org`, `"OrderToGo" plugin claude-plugins-official`
- Result: **zero** existing CLIs, MCPs, plugins, SDKs, or wrappers for OrderToGo.com. The platform is small enough that no developer has built tooling for it. We are the first.
- Adjacent restaurant-ordering tooling (own Printing Press CLIs):
  - `dominos-pp-cli` — pizza ordering with deal optimization, delivery tracking, local SQLite history, "usual" reorder
  - `pagliacci-pp-cli` — Seattle pizza ordering, discount stacking, slice rotation, local order history
  - `instacart-pp-cli` — grocery ordering with PQL hash extraction technique
  - These three define the shape: cookie auth via Chrome import, sync to local SQLite, agent-native commands, dry-run-first ordering, `usual` pattern.

## Note on novel-features subagent

The skill mandates spawning a Task subagent in Step 1.5c.5 to brainstorm novel
features. This run intentionally skips that step because (a) the user supplied
an unusually concrete vision in their briefing, (b) the discovery captured a
complete API surface, and (c) the CLI is personal-scope (not public-library
candidate). The transcendence features below are derived from the user's
brief and the place-order safety constraints, not from a generic brainstorm.

## Absorbed (match or beat everything that exists on OrderToGo.com)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse restaurants by location | OrderToGo.com homepage `/m/api/restaurants/filter/<loc>` | `restaurants list [--location sto]` with --json | Offline cache, agent-native JSON, `--select` |
| 2 | Restaurant detail (hours, address, phone) | Web restaurant page | `restaurants show <slug>` | Local cache with `is_open` computed live from hours |
| 3 | Full menu fetch with options | `/m/api/restaurants/<slug>/menus/full` | `menu <slug> [--category Roll]` | FTS5 search, offline browse, `--json` for agents |
| 4 | View item detail with modifiers | Web menu detail modal | `menu <slug> item <id>` | Pure JSON, easy `--select` for option IDs |
| 5 | Add item to cart with options | Web "Add" button + options modal | `cart add <slug> <item-id> [--option <opt-id>] [--note "No Ginger"]` | Per-restaurant cart with same localStorage shape, `--dry-run` |
| 6 | Edit / remove cart items | Web cart panel | `cart show <slug>`, `cart remove <slug> <idx>`, `cart clear <slug>` | All structured |
| 7 | Set tip (10/15/20/Other%) | Web "Select Tip" row | `--tip 15%` or `--tip 0.90` flag on `order plan/place` | Auto-tip = avg of history if `--tip auto` |
| 8 | Set customer phone / name | Web checkout fields | Pulled from local config (set once via `auth login`) | Never re-prompts; agent-friendly |
| 9 | Place order (Braintree DropIn → `/m/api/orders/braintreeCheckout`) | Web "Place Order!" button | `order place --confirm --max 25` (chromedp headless drive) | Budget gate, idempotency key, verify-env short-circuit |
| 10 | Order history list | `POST /m/api/getmicmeshorders` | `orders list [--restaurant <slug>] [--since 30d]` | Local SQLite, SQL-queryable |
| 11 | Order detail | `POST /m/api/getmicmeshorders` `{orderid, restname}` | `orders show <id>` | Cached, supports `--json` |
| 12 | Order tracking (received → preparing → ready → picked up) | `/trackorder/<orderToken>` HTML | `order track <token> [--watch]` | Poll loop with typed exit codes (0=picked up, 7=still preparing) |
| 13 | "Order Again" / reorder | Web Order History detail "Order Again" button | `order plan --reuse-last [--restaurant <slug>]` | Pure-Go, no browser; agent-callable |
| 14 | Reward Points | Web "Reward Points" panel | `rewards [--restaurant <slug>]` | Across all restaurants in one call |
| 15 | Giftcards | Web "My Giftcards" panel | `giftcards` | Same |
| 16 | Coupons / promo codes | Web "My Coupons" panel + `/api/markPromotionUsed` | `coupons list`, `coupons apply <code>` | Surfaced before place |
| 17 | Restaurant open/closed awareness | Web closed-modal | `restaurants show --closed-check`, used internally before place | Hard refusal to place at closed restaurant |
| 18 | Multi-location chains | Web filtering | `restaurants list --chain "Mix Sushi"` returns all locations | Single call |
| 19 | Notifications badge | `GET /api/ordertogoV2/notification/getNotificationCountUserUnread` | `notifications` | Shown on `agent-context` |
| 20 | Cancel order (within window) | `POST /m/api/voidSelfOrder` | `order cancel <token> --confirm` | Confirmation gate |
| 21 | Group ordering | `POST /m/api/party_invite` | `order invite <emails...>` | Stretch — needed only if requested |
| 22 | Geocoding (delivery) | `POST /m/api/mapping/geocode` | not needed (pickup-only for this user) | Skipped |
| 23 | Auth via cookie session | Firebase phone-OTP on web | `auth login --chrome` imports cookies from Chrome | One-time setup; CLI never sees password / OTP |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | "What's my usual" detection | `usual [--restaurant <slug>]` | Requires clustering of all historical orders by item-set similarity over local SQLite history; the website only shows discrete past orders | 9 |
| 2 | Reuse-last with budget gate | `order plan --reuse-last --max 25` | Requires composing cart locally, calling `/m/api/orders` for tax, and gating against a user-provided cap before any payment can fire | 10 |
| 3 | Headless full-auto place | `order place --confirm --max 25 [--reuse-last]` | Needs chromedp to drive Braintree DropIn → saved-card → confirm; no other tool exists | 10 |
| 4 | Spending analytics | `spending [--since 90d] [--restaurant <slug>]` | Requires local sync of all orders + SQL; the web shows a list, not totals | 8 |
| 5 | Order cadence detection | `cadence [--restaurant <slug>]` | "You order roughly weekly on Wednesdays" — local time-series over orders | 7 |
| 6 | Auto-tip from your history | `--tip auto` on plan/place | Requires reading historical tips for that restaurant from local store and computing avg | 7 |
| 7 | Closed-check before place | `order plan` refuses if restaurant closed at requested pickup window | Cached hours + local clock; the web only checks at submit | 7 |
| 8 | Agent-context snapshot | `agent-context` | Single-call dump for agents: account, default restaurant, usual, last-order, budget, days-since-last; designed for "I'm an agent and need everything" | 9 |
| 9 | Order tracking watch | `order track <token> --watch` | Polling loop with typed exit codes; tells an agent "still preparing" vs "ready for pickup" without parsing HTML | 6 |
| 10 | Sync once, query forever | `sync --orders` then offline `orders list --since 7d --json` | Web requires live session; we cache | 6 |
| 11 | Receipt export | `orders show <id> --format receipt` | Plain text or markdown receipt, shareable | 5 |
| 12 | Multi-restaurant cart context | `cart show --all` shows every restaurant cart in one view | localStorage carts are per-restaurant; we surface them all in one place | 5 |
| 13 | Verify-env safe by construction | All side-effect commands honor `cliutil.IsVerifyEnv()` | Required by skill; ensures dogfood and CI don't accidentally place orders | 8 |
| 14 | Idempotency keys | Every place call includes `__requestid: <epoch>_<rand>` from observation | Matches the website's own pattern; prevents double-charge on retry | 7 |
| 15 | Fall-through to visible browser if headless fails | `order place --visible-on-fail` (default true) | If chromedp fails to find Braintree DropIn or the saved card, opens visible Chrome at the pre-filled checkout for one-tap completion | 8 |

## Stubs (not shipping in v1, declared explicitly)

| # | Feature | Status | Why deferred |
|---|---------|--------|--------------|
| 1 | Group/party ordering | (stub) | Requires mapping the `/m/api/party_invite` body shape; user has no need yet |
| 2 | Delivery support | (stub) | Account is pickup-only; geocode endpoint kept but not exercised |
| 3 | Promo code redemption | (stub) | Discoverable via web flow; not in user's flow |
| 4 | Reward points redemption (spend-side) | (stub) | Read works (`rewards`); spending requires investigating the redemption endpoint, low priority |

## Build order

1. **Auth + read path** (P0): `auth login --chrome`, `restaurants list/show`, `menu`, `orders list/show`, `last-order`, `order track`, `rewards`, `giftcards`, `coupons list`, `notifications`
2. **Local store + sync** (P0): SQLite schema, `sync --restaurants`, `sync --menu <slug>`, `sync --orders`, FTS5 index
3. **Cart + plan** (P1): `cart add/show/remove/clear`, `order plan`, `order plan --reuse-last`, all dry-run-friendly
4. **Place + safety** (P1): `order place --confirm --max <dollars>` with chromedp headless flow + visible-fall-through, `--confirm-over-budget`, idempotency, verify-env short-circuit
5. **Transcendence** (P2): `usual`, `spending`, `cadence`, `agent-context`, `--tip auto`, closed-check, multi-cart view
6. **Polish** (P3): receipt export, `track --watch`, comprehensive `--json` and `--select` everywhere
