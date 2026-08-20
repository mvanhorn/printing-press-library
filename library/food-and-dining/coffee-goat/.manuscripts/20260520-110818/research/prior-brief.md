# coffee-goat CLI Brief

## API Identity
- Domain: Specialty / third-wave coffee discovery, brewing, and palate calibration.
- Users: Coffee enthusiasts who buy beans from multiple roasters across continents, log their brews, and care about freshness, origin, producer, process, and cupping scores.
- Data profile: Public roaster storefront catalogs (Shopify, WooCommerce, Square Online, Snipcart) + Coffee Review's score corpus + Sprudge editorial. Personal brew/cellar log lives only in the local SQLite.

## Reachability Risk
- **Low for Tier-1 (21 Shopify storefronts):** all 21 returned 200 on `/products.json` during scoping probes (after resolving Leaves → `leaves-coffee-roasters.myshopify.com` and Friedhats → `www.friedhats.com`). Shopify storefronts expose this endpoint by convention and rate-limit gracefully.
- **Low for Tier-1b WooCommerce (Mame):** `/wp-json/wc/store/v1/products` returned 200, public endpoint by default.
- **Medium for Tier-2:**
  - Loquat (Square Online): HTML scrape required, no public JSON endpoint.
  - DAK (Snipcart): static HTML + `data-item-*` Snipcart attributes; needs sitemap walk.
- **Medium for editorial:**
  - Coffee Review: WP REST + `/feed/` work without auth.
  - Sprudge: Cloudflare-fronted; HEAD returned 403. Needs Chrome-cookie clearance fallback like `recipe-goat`/`allrecipes`.

## Top Workflows
1. **Browse the global current third-wave shelf** — "show me every Ethiopian natural in stock right now, ranked by $/oz."
2. **Log a brew session** — capture grind/dose/yield/time/temperature/rating; feed into dial-in recommendations.
3. **Track a bean cellar** — what's on the shelf, freshness window, what to drink first.
4. **Find a bean's twin** — "I loved Sey's Gesha last month, find the closest current alternative across all 24 roasters."
5. **Track a producer** — "Diego Bermudez, every roaster, every lot, every year."

## Table Stakes (absorbed from incumbents)
- Per-roaster bean browse (every roaster's own site does this — we do all 24 at once).
- Origin / process / varietal filtering.
- Price display in user's currency.
- Roast date / freshness indicator (where exposed).
- Coffee Review score lookup (currently a separate browse on coffeereview.com).
- A brew log (Beanconqueror, MyCoffeeNote, paper notebooks — fragmented landscape).

## Data Layer

**Primary entities:**
- `roaster_products` — synced from 24 sources, normalized to a unified Product shape: roaster, name, origin (country/region), producer/farm, process, varietal, altitude, roast_level, tags, price_cents, currency, weight_g, url, image_url, in_stock, published_at, updated_at, body_text (cleaned).
- `roasters` — roaster identity + transport metadata (Shopify slug, WooCommerce base, custom sniff config).
- `beans` — user's personal cellar: source roaster_product_id (FK), purchase_date, roast_date (when known), price_paid_cents, current_mass_g, notes.
- `brews` — user's brew log: bean_id (FK), method (espresso/v60/aeropress/...), grind, dose_g, yield_g, time_s, temperature_c, water_tds_ppm, rating (1-10), notes, brewed_at.
- `reviews` — Coffee Review join: score, descriptors_json, reviewer, published_at; matched to `roaster_products` via fuzzy (roaster, bean name).
- `watchlist` — saved queries: query_text, filters_json, last_sync_anchor.
- `producers` — derived index over `roaster_products` for cross-roaster producer tracking.

**Sync cursor:** per-roaster `last_synced_at`; for Shopify, the `updated_at` column on products supports incremental diffs.

**FTS:** SQLite FTS5 over `roaster_products(body_text, name, origin, producer, varietal)`. Powers `search`.

## Codebase Intelligence

No upstream OpenAPI spec exists for any source — this is a sniffed/wrapper-only CLI. Source code intelligence comes from convention knowledge:

- **Shopify storefronts** — `/products.json?limit=N` returns `{products: [{id, title, vendor, product_type, tags, variants[{price, sku, available, grams}], body_html, images[], options[], published_at, updated_at, handle}, ...]}`. Stable across 21/21 probed storefronts. Pagination via `page=N` (deprecated but still works) or `since_id=N` (preferred).
- **WooCommerce Store API** — `/wp-json/wc/store/v1/products?per_page=N&page=N` returns block-store schema. Field set differs from Shopify but covers same semantics.
- **Square Online** (Loquat) — no JSON, HTML pages under `/menu/<id>` or sitemap walk.
- **Snipcart** (DAK) — static site with `data-item-id`, `data-item-name`, `data-item-price`, `data-item-url`, `data-item-description` attributes on product pages. Sitemap discovers product URLs.
- **Coffee Review WP REST** — `/wp-json/wp/v2/posts` returns score-bearing review posts; score appears in `content.rendered` HTML (regex-extractable). RSS at `/feed/` is the lighter path.
- **Sprudge RSS** — `/feed` Cloudflare-blocked anonymous; Chrome-cookie clearance unblocks.

**Auth:** none for tier-1, tier-1b. Cookie/clearance only for Sprudge fallback. Optional hardware (La Marzocco Home cloud, Decent DE1+ local) deferred to v0.2.

## User Vision

User locked the scope across 6 turns of scoping conversation. Key commitments:
- 24 roasters, named individually (Tier-1: 21 Shopify; Tier-1b: 1 Woo; Tier-2: 2 sniffs).
- 12 novel features, each with detailed spec.
- GOAT-shaped: cross-source corpus, local SQLite, personal-history analytics.
- Anti-reimplementation strictly enforced.
- Hardware integrations defer to v0.2.

## Source Priority

Single conceptual primary: the unified `roaster_products` corpus aggregated across all 24 roasters as peer sources. Editorial sources (Coffee Review, Sprudge) are read-side enrichment. No per-source priority ordering required — every roaster is a peer feeding the same table.

- **Economics:** all sources free. No paid API keys.
- **Inversion risk:** none — there is no spec/no-spec asymmetry across sources.

## Product Thesis

- **Name:** coffee-goat
- **Display name:** Coffee GOAT
- **Headline:** "The third-wave coffee terminal — 24 elite roasters, your brew log, and a CLI that diagnoses your grinder from your rating drift."
- **Why it should exist:** No tool aggregates the global specialty-coffee shelf and joins it with a personal brew log + cross-roaster producer tracking + palate calibration vs. published reviews. Each roaster's own site covers its own beans. Beanconqueror logs brews but knows nothing of beans you haven't bought. Coffee Review scores but doesn't tell you what's in stock. coffee-goat is the first CLI to join all three corpuses with cross-source analytics.

## Build Priorities

1. **Data foundation:** sync from all Tier-1 (21 Shopify) + Tier-1b (1 Woo). SQLite store with unified `roaster_products` + `roasters` + FTS5. Body-HTML NLP for origin/producer/process/varietal/altitude extraction.
2. **Personal data:** `beans`, `brews`, `watchlist` tables + CLI verbs to add/edit/remove. This is the second corpus.
3. **Tier-2 sources:** Loquat (Square Online HTML), DAK (Snipcag HTML). One adapter each.
4. **Editorial:** Coffee Review WP REST sync (`reviews` table), Sprudge RSS with Chrome cookie fallback.
5. **Novel features (12):** `search`, `dial-in`, `shelf`, `watch`, `compare`, `twin`, `blind-cup`, `fx`, `producer`, `refill-plan`, `roaster-style`, `drift`.
6. **Polish:** dial-in's Bayesian recipe model, twin's similarity decomposition, drift's regression diagnostic.

## Constraints / Risks

- **Body-HTML NLP variance:** every roaster phrases origin/process differently. Need a per-roaster light-extractor + a global fallback regex set + tags-first preference.
- **Coffee Review fuzzy match:** roaster names don't always match cleanly between corpora (e.g., "Sey" vs "Sey Coffee" vs "Sey Coffee Roasters"). Match must be tolerant.
- **Sprudge cookie freshness:** Cloudflare clearance cookies rotate; documented in `recipe-goat` pattern.
- **Vendor mix in Black & White:** filter `product_type: "Coffee"` (skip Hugo Tea).
- **Internal spec authoring:** no upstream OpenAPI exists. The Printing Press internal YAML spec format is the right shape; need to hand-author one.

## Reachability Probes (already executed during scope conversation)

| Source | Endpoint | Status | Notes |
|---|---|---|---|
| Glitch (JP) | `glitchcoffeeroasters.myshopify.com/products.json` | 200 | Direct Shopify slug |
| Leaves (JP) | `leaves-coffee-roasters.myshopify.com/products.json` | 200 | Resolved from Nuxt frontend |
| Prodigal | `prodigalcoffee.com/products.json` | 200 | |
| Black & White | `blackwhiteroasters.com/products.json` | 200 | Filter product_type:Coffee |
| Onyx | `onyxcoffeelab.com/products.json` | 200 | |
| Sey | `seycoffee.com/products.json` | 200 | |
| Tim Wendelboe | `www.timwendelboe.no/products.json` | 200 | |
| Hydrangea | `hydrangea.coffee/products.json` | 200 | |
| Passenger | `passengercoffee.com/products.json` | 200 | |
| George Howell | `georgehowellcoffee.com/products.json` | 200 | |
| Heart | `www.heartroasters.com/products.json` | 200 | |
| Saint Frank | `saintfrankcoffee.com/products.json` | 200 | |
| Verve | `www.vervecoffee.com/products.json` | 200 | |
| Proud Mary | `proudmarycoffee.com/products.json` | 200 | |
| Square Mile (UK) | `shop.squaremilecoffee.com/products.json` | 200 | |
| April (DK) | `aprilcoffeeroasters.com/products.json` | 200 | |
| Coffee Collective (DK) | `coffeecollective.dk/products.json` | 200 | |
| Manhattan (NL) | `manhattancoffeeroasters.com/products.json` | 307→200 | normal redirect |
| Friedhats (NL) | `www.friedhats.com/products.json` | 200 | Required `www.` |
| The Barn (DE) | `www.thebarn.de/products.json` | 200 | |
| La Cabra (DK) | `lacabra.dk/products.json` | 200 | |
| Mame (CH) | `mame.coffee/wp-json/wc/store/v1/products` | 200 | WooCommerce |
| Loquat (LA) | `www.loquatcoffee.com/` | 200 | HTML (Square Online) |
| DAK (NL) | `dakcoffeeroasters.com/` | 200 | HTML + Snipcart |

24/24 reachable, no challenges, no auth.
