# Amazon CLI Brief

## API Identity
- Domain: amazon.com (US storefront), buyer side
- Users: end shoppers reordering items they've previously purchased; nanoclaw agent acting on the user's behalf
- Data profile: order history (the only thing we care about), cart state, addresses, payment defaults

## Reachability Risk
- Medium. Amazon has no public buyer API. The CLI must replay HTTP requests against `www.amazon.com` using an authenticated browser session (cookies copied/imported from Chrome). Amazon aggressively rotates anti-bot tokens, but logged-in users browsing normally are not generally blocked. Risk mitigation: scope the CLI to the same endpoints a logged-in browser hits (order history page, "Buy Again" page, add-to-cart, checkout). No scraping at scale, no concurrent fan-out.

## Top Workflows
1. **Reorder by name** — the user says "add bath tissue" → CLI looks up her local history, finds the most-frequent matching past order, adds that exact ASIN to the cart on her chosen profile.
2. **Show cart** — quick read of what's currently in the cart on a given profile.
3. **Checkout** — place the order with default shipping address + payment, after an explicit "yes" confirmation.
4. **Switch profiles** — same CLI binary, multiple Amazon accounts (personal vs work).
5. **Backfill history** — initial dump from `/your-orders` so the local SQLite store has data to match against.

## Table Stakes
- `--profile <name>` on every command, mirroring the instacart pattern
- `--json` everywhere
- `--dry-run` for `add` and `checkout`
- Idempotent add (don't double-add if already in cart)
- SQLite local store at `~/.config/amazon-pp-cli/<profile>.db`

## Data Layer
- Primary entities: `orders` (order_id, placed_at, total), `order_items` (order_id, asin, title, quantity, price), `purchased_items` (asin, title, purchase_count, last_purchased_at) — derived view for fast lookup, `profiles` (name, label, session_cookie_path)
- Sync cursor: max(placed_at) per profile
- FTS: `purchased_items_fts` over title for fuzzy "add bath tissue" → ASIN matching

## Codebase Intelligence
- Reference: `~/projects/printing-press-library/library/commerce/instacart/` (printing-press v4 internal pattern)
- Same shape: cobra root → AppContext → SQLite store + auth session, GraphQL/HTTP client per service, `history import` for browser-side dumper output, `add` resolves history-first
- Amazon-specific: no GraphQL, all HTML scraping + JSON sub-payloads. `/gp/your-store/home`, `/your-orders/order-details?orderID=...`, `/gp/aw/d/<ASIN>`, `/gp/aws/cart/add.html`, `/gp/buy/spc/handlers/display.html` (checkout).

## User Vision
- Repurchase-only — refuse to add an item that has no history match. No search/discovery for new items.
- Full checkout supported. Order placement gated on explicit "yes" confirmation, no exceptions.
- Multi-account: `--profile personal` and `--profile work`.
- Resolution tiebreak: when multiple history matches, pick the **most-frequently-purchased** one (not most-recent).
- Reference CLI: instacart-pp-cli. Match structure 1:1.

## Source Priority
- Single source: amazon.com (buyer side). No public API, browser-sniff is the discovery path. User pre-approved this at Phase 0 ("the website itself").

## Product Thesis
- Name: `amazon-pp-cli`
- Why it should exist: every existing Amazon CLI/MCP I can find either uses the Product Advertising API (seller affiliate stuff, no cart/checkout) or wraps a Playwright sidecar (slow, brittle, requires keeping a browser running). This one talks directly to amazon.com using your Chrome session cookies — sub-second add-to-cart, no headless browser at runtime, repurchase-only (refuses to add anything that isn't in your history) so an agent can't go off-script and buy random things.

## Build Priorities
1. **P0 foundation**: profile system (config + multi-profile DB layout), auth session loader (Chrome cookie import via kooky + paste fallback), SQLite store with `orders` / `order_items` / `purchased_items` / `purchased_items_fts`, root cobra cmd
2. **P1 absorbed core**: `auth login` (kooky), `auth paste` (Cookie header paste), `auth import-file` (Chrome cookies JSON), `doctor`, `history import <path>` (JSONL from browser-side dumper), `history list`, `history search`, `history stats`, `cart show`, `add <query>` (history-first, refuses on no match, picks most-frequent), `checkout` (gated on `--yes` confirmation flag), `profiles list/add/use`
3. **P2 transcendence**: `reorder-last` (re-add every line from the most recent order), `add --dry-run` (preview the match without writing to cart), `history dedup` (collapse multiple ASINs that map to the same product), full nanoclaw skill integration
