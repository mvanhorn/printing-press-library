# Amazon CLI Absorb Manifest

## Sources researched
- **No upstream API to absorb.** Amazon Product Advertising API is affiliate/seller only; it cannot read the buyer's cart or order history and cannot place orders. The buyer-side surface lives entirely behind `www.amazon.com` HTML/XHR with session cookies.
- **Reference pattern**: `~/projects/printing-press-library/library/commerce/instacart/` — same authenticated-session-scraping recipe (Chrome cookies via kooky + JSONL history import + history-first add). All commands below are structurally lifted from instacart and adapted to Amazon's surface.
- **MCP servers**: a handful of "amazon mcp" repos exist on GitHub but they wrap Product Advertising API (search affiliate links) and contribute nothing to buyer-side workflows.
- **No competing buyer-side CLI worth absorbing**. The closest cousin is `amazon-buyer-bot` style Playwright wrappers that keep a browser open per call — explicitly the anti-pattern this CLI exists to replace.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|--------------|--------|
| 1 | Authenticate against amazon.com | Playwright/Selenium pattern | kooky Chrome cookie import + paste fallback + import-file | Sub-second startup, no browser running at runtime | shipping |
| 2 | View current cart | playwright `goto /cart` | direct `/gp/cart/view.html` HTTP, parse line items | <1s, no browser, JSON output | shipping |
| 3 | Add item by name to cart | Playwright search+click | history-first lookup against local SQLite + direct `/gp/aws/cart/add.html` | History-only (no search drift), most-frequent tiebreak, --dry-run preview | shipping |
| 4 | View order history | playwright `goto /your-orders` | JSONL import from browser-side dumper → SQLite | Offline, FTS searchable, persistent | shipping |
| 5 | Local history search | n/a | SQLite FTS5 over purchased_items | Offline, regex via SQL, scriptable | shipping |
| 6 | Diagnose setup | n/a | `doctor` — checks profile, cookies, DB, marker endpoint | Single command surfaces all failure modes | shipping |
| 7 | Multiple Amazon accounts | n/a | `--profile <name>` per-call + named profiles in config | the user has personal + work; switch via flag | shipping |
| 8 | Place an order | playwright checkout walkthrough | direct `/gp/buy/spc/handlers/display.html` HTTP flow, explicit `--yes` confirm gate | Refuses without `--yes`, never auto-confirms, JSON post-order receipt | shipping |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Repurchase the entire last order | `reorder-last` | hand-code | Requires local-history join across orders + order_items — Amazon's UI offers buy-again per-item but not as a single batched action |
| 2 | Dry-run a name → ASIN match before adding | `add --dry-run "<query>"` | hand-code | Local FTS resolution lets us preview the resolved ASIN, title, last-purchased-at, and purchase count without touching the live site |
| 3 | Most-frequent tiebreak vs most-recent | `add` resolver | hand-code | Local history-count column makes the tiebreak deterministic and explainable |
| 4 | Honest refusal when no history exists | `add` | hand-code | History-only repurchase is the safety rail — refuses to invent a new SKU. No competing tool does this; they all fall back to "first search result", which is exactly the agent-shopping failure mode this CLI prevents |
| 5 | Cross-profile history isolation | `--profile <name>` + per-profile DB | hand-code | Each profile's history lives in its own SQLite file, so the work account's office-supply history doesn't bleed into the personal account's repurchases |

## Out of scope (intentionally not built)
- New-item search / discovery (`search` against amazon.com catalog) — would defeat the repurchase-only safety rail
- Subscribe & Save management
- Prime Pantry / Fresh / Whole Foods integration
- Gift card / digital downloads / Kindle
- Returns / cancellation
- Saved-for-later list management

## Stubs
- None. Every shipping-scope row above is implemented in full.

## Shipping count
- 8 absorbed features + 5 transcendence features = 13 features
- Hand-code count: 5 transcendence rows (all hand-code per Buildability)
- Stub count: 0
