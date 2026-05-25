# coffee-goat Session 2 Handoff — Phase 3 follow-up scope

> Companion to the Session 1 handoff at `~/printing-press/manuscripts/coffee-goat/20260520-102217/HANDOFF.md`. This document describes what Session 2 built (the vertical slice) and what remains for a future Phase 3 session.

## What Session 2 built (vertical slice)

Foundation:
- `internal/roasters/registry.go` — 24-roaster static registry with transport metadata + tests
- `internal/store/coffee_schema.go` — extended schema: roasters, roaster_products (+ FTS5), reviews, youtube_reviews (+ FTS5), beans, brews, watchlist, palate_profiles, sync_state + tests
- `internal/refdata/sca_wheel.go` — SCA Coffee Tasters' Flavor Wheel taxonomy (curated, `// pp:novel-static-reference`)
- `internal/extract/extract.go` — body_html NLP extractor (origin/producer/process/varietal/altitude) + tests
- `internal/cli/doctor.go` — rewritten to ping representative roaster + check youtube-pp-cli on PATH

3 source adapters:
- `internal/source/shopify/shopify.go` — 21 of 24 roasters; pagination, rate-limited, Black&White filter
- `internal/source/coffeereview/coffeereview.go` — WP REST + /feed/ fallback
- `internal/source/youtube/youtube.go` — RSS feed + youtube-pp-cli subprocess for Hoffmann + Hedrick

sync.go rewrite:
- Multi-source orchestration with per-source AdaptiveLimiter
- Graceful degradation when youtube-pp-cli is missing
- `--source`, `--roaster`, `--full` flags
- JSON summary output

7 transcendence commands:
- `search` — Cross-roaster FTS5 over roaster_products_fts
- `watch` — Persisted query diff (save/list/run subcommands)
- `twin` — Cosine similarity over normalized attribute vectors
- `creator-review` — Hoffmann/Hedrick clip lookup with mention extraction
- `flavor-wheel` — SCA wheel mapping from brew descriptors × ratings
- `friend-pick` — Palate-profile export/import + recommend bag for friend
- `god-cup` — 5-signal weighted recommender (brew_pick + buy_pick)

Each command has a behavioral acceptance test verifying output content (not just exit code).

## What's NOT done — Phase 3 follow-up scope

### 7 remaining source adapters

1. **`internal/source/woocommerce/`** — Mame (CH). Endpoint: `https://mame.coffee/wp-json/wc/store/v1/products?per_page=N&page=N`. Block-store schema differs from Shopify but covers same semantics. ~150 LOC.

2. **`internal/source/squareonline/`** — Loquat (LA). Pure HTML scrape; product URLs under `/menu/<id>`. Need sitemap walk. ~200 LOC.

3. **`internal/source/snipcart/`** — DAK (NL). Static HTML site with `data-item-id`, `data-item-name`, `data-item-price`, `data-item-url`, `data-item-description` attributes. Sitemap walker. ~200 LOC.

4. **`internal/source/sprudge/`** — Sprudge editorial RSS + city guides. Cloudflare-protected; requires Chrome cookie import (`auth sprudge --chrome`). RSS at `/feed`; city guides at `/guides`. ~150 LOC.

5. **`internal/source/wikipedia/`** — Wikipedia REST API for WBC + sibling competition winners. Endpoint: `https://en.wikipedia.org/api/rest_v1/page/summary/<title>` + full HTML for table parsing. ~100 LOC.

6. **`internal/source/overpass/`** — OSM Overpass API for cafes. CRITICAL: must set a custom User-Agent (curl default UA returns 406). Endpoint: `https://overpass-api.de/api/interpreter`. Form-encoded `data=...` POST body. ~120 LOC.

7. **`internal/source/wbc/`** — wcc.coffee HTML scrape + Sprudge cross-reference for champion recipes. Note: wcc.coffee has NO usable past-rankings page (Squarespace soft-404 on `/wbc-past-rankings`). Use Wikipedia as primary; Sprudge for bean/recipe metadata. Tolerates partial-fill rows. ~250 LOC.

### 14 remaining transcendence commands

Personal-history analytics (7):
- `dial-in <bean>` — Bayesian recipe from history of similar beans (joins brews × roaster_products by attribute clusters)
- `shelf` — Cellar view with per-method freshness windows (espresso 8–21d, filter 5–28d; use refdata/freshness.go)
- `whats-next` — One bag for tomorrow morning (shelf × dial-in confidence × palate weight)
- `drift` — Fixed-effects OLS regression: `rating ~ days + bean_fe + method_fe`. ~150 LOC including regression helper.
- `refill-plan` — Consumption rate × twin similarity × shelf depletion → 3 replacement picks
- `blind-cup` — Interactive cupping session w/ hashed IDs; reveal at end + Spearman ρ vs Coffee Review
- `palate-map` — Origin/process/varietal weight signature learned from rated brews
- `bag-life <bean>` — Daily rating curve vs days-since-roast for one bag; peak/decline/dead label

Cross-source corpus (1):
- `compare <bean> <bean> [...]` — Multi-bean delta table; joins reviews for scores

Cross-source corpus (1):
- `producer <name>` — Track a producer across roasters across years; GROUP BY producer + year filter

Cross-source corpus (1):
- `fx <product>` — ECB FX cache (daily) × curated `roaster_shipping` static reference (`internal/refdata/shipping.go`, `// pp:novel-static-reference`) for `--ship-to` landed cost

Editorial signal (1):
- `transcript-search "<query>"` — Local FTS5 over `youtube_reviews_fts` with `--creator <name>` filter

Championship (1):
- `champion-replay <year> [--competition wbc|brewers-cup|...]` — depends on internal/source/wbc/ + internal/source/wikipedia/ + champion_recipes table population. Tolerates partial-fill rows.

Discovery (1):
- `cafe-near "<location>"` — depends on internal/source/overpass/ + internal/source/sprudge/ (for city-guide overlay)

### Phase 3 build priority items (not transcendence rows)

- **`beans add --url <roaster-url>`** — UX polish on the existing absorbed `beans` resource. Accept a roaster URL, fuzzy product slug, or interactive selection. Add to `internal/cli/beans_add.go`.
- **`method_profiles` seed** — Pre-populate with V60, Origami Air, Sibarist SD-1 (filter), Oxo Rapid Brewer (immersion), espresso. Plus the Decent DE1 placeholder for v0.2.
- **`scan` command** — LLM-assisted bag-bag photo identification (already an opt-in stub from Session 1; keep as-is, requires `OPENROUTER_API_KEY`).

### Deferred to v0.2

- Decent DE1 shot-file integration (Persona 5 — Mike's use case is currently served by espresso-focused features without DE1 data).
- La Marzocco Home cloud integration.
- Hardware-aware dial-in adjustments.

## Resume instructions for next session

1. Start a fresh `/printing-press coffee-goat` session.
2. The skill's Phase 0 "Resolve and Reuse" step will discover this manuscript at `$PRESS_MANUSCRIPTS/coffee-goat/20260520-110818/`. The brief, absorb manifest, novel-features brainstorm, source probes, and research.json are all current.
3. The skill will ask whether to re-validate prior research; accept "reuse as-is" — this manuscript is fresh.
4. Choose **"Generate a fresh CLI"** only if you want to refresh the templates from the current binary; otherwise resume directly from the working directory at `$PRESS_LIBRARY/coffee-goat/` (after Session 2 promotes it).
5. Proceed directly to **Phase 3 follow-up**: build the 7 remaining source adapters, then the 14 remaining commands.

## Risks for next session

1. **WooCommerce schema differences** — Mame's Store API returns block-editor blocks for product description, not raw HTML. Adapter must walk the block tree.
2. **Loquat (Square Online) and DAK (Snipcart) HTML drift** — these are the riskiest scrapers; site redesigns will break extraction. Per-roaster light-extractors with golden HTML fixtures.
3. **WBC recipe data sparsity** — `champion-replay` returns empty recipe fields for years before ~2018. README troubleshoot already documents this.
4. **OSM Overpass rate limits** — 25-second daily query budget per IP. Cache aggressively; serve repeat queries from `cafes` table.
5. **YouTube mention extraction precision** — regex-only first pass is noisy. If acceptance tests show poor precision, the LLM extraction fallback (via `OPENROUTER_API_KEY`) is the documented escape hatch.
6. **drift command** — Needs a proper linear-regression helper. `gonum.org/v1/gonum/stat` is fine; or hand-write fixed-effects OLS in `internal/stats/`.
7. **palate-export/palate-import format stability** — Commit to a versioned JSON format from day 1 (`{schema_version: 1, ...}`) so friend-pick stays compatible across CLI versions.
