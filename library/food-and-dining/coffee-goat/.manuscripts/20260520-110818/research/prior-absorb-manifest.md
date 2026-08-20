# coffee-goat Absorb Manifest

## Tools surveyed (Phase 1.5a)

| Tool | URL | Role | Coverage |
|---|---|---|---|
| Beanconqueror | https://github.com/graphefruit/Beanconqueror | Mobile-first brew log + bean inventory | Strongest incumbent for brew logging; mobile-only; no cross-roaster discovery |
| Brewlog | https://github.com/jnsgruk/brewlog | Self-hosted Rust CLI/web + LLM bag-scan | Direct CLI competitor; single-user manual entry; no cross-roaster sync |
| Artisan | https://github.com/artisan-roaster-scope/artisan | Roasting software for actual roasters | Wrong segment (roasters not drinkers); not absorbed |
| Doppio Coffee MCP | https://mcpmarket.com/es/server/coffee-1 | Single-roastery ordering MCP | Limited to one Slovak roastery; not absorbed |
| 24 individual roaster Shopify storefronts | various | Per-roaster catalog browse | Each shows its own beans; none aggregate |

## Absorbed (match-or-beat every existing feature)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---|---|---|---|---|
| A1 | Roaster + country + URL registry | Brewlog (`roaster add`) | `coffee-goat roasters list/show/add` | 24 roasters preloaded across continents; auto-sync each | shipping |
| A2 | Per-roaster bean catalog | All 24 roaster sites | `coffee-goat products list --roaster <name>` | One CLI, all 24, FTS5 offline | shipping |
| A3 | Cross-roaster origin/process/varietal filter | None (gap) | `coffee-goat products list --origin --process --varietal` | First tool to filter across roasters | shipping |
| A4 | Bean inventory (cellar) with running total | Beanconqueror, Brewlog | `coffee-goat beans add/list/use/archive` | Plus freshness window flags | shipping |
| A5 | Brew log with 30+ params | Beanconqueror | `coffee-goat brews log --method --grind --dose --yield --time --temp --rating` | Terminal-native, agent-readable, scriptable | shipping |
| A6 | Brewing method profiles (V60/Aeropress/Espresso/...) | Beanconqueror | `coffee-goat methods list/show` | Defaults + user-defined | shipping |
| A7 | Water profile tracking | Beanconqueror | `coffee-goat water add/use` | Joins with brews | shipping |
| A8 | SCA cupping form (aromatics/acidity/body/finish) | Beanconqueror | `coffee-goat cup score --sca` | Stored per bean | shipping |
| A9 | Per-bean ratings + notes | Beanconqueror, Brewlog | `coffee-goat beans rate <id> --score --notes` | FTS5-searchable notes | shipping |
| A10 | Coffee Review score lookup | coffeereview.com browse | `coffee-goat reviews show <bean>` | Auto-joined to local roaster_products | shipping |
| A11 | Cafe check-in | Brewlog | `coffee-goat cafe checkin <name>` | Optional, low-priority | stub-acceptable |
| A12 | LLM-powered bag-scan (photo → roaster/coffee) | Brewlog (OpenRouter) | `coffee-goat beans scan <photo>` | Opt-in, gated on `OPENROUTER_API_KEY` env var | shipping (gated) |
| A13 | Sync from upstream (Shopify storefronts) | Implicit in each roaster site | `coffee-goat sync [--roaster <name>] [--limit N]` | Aggregates all 24, single command | shipping |

13 absorbed features total. All match or beat the best incumbent.

## Transcendence (only possible with our cross-source corpus + local SQLite)

### LOCKED — User explicitly approved (12) + 1 subagent-proposed addition awaiting Phase Gate 1.5 approval (13)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---|---|---|---|---|
| T1 | Cross-roaster FTS search | `coffee-goat search "ethiopia natural" --in-stock --price-lt 25` | 10/10 | FTS5 MATCH on `roaster_products_fts` joined to `roaster_products` for stock/price filters across all 24 roasters | User briefing (prior-keep); brief Top Workflow #1; no incumbent searches all 24 at once |
| T2 | Bayesian recipe recommendation | `coffee-goat dial-in <bean>` | 10/10 | Aggregates dose/yield/ratio/time/temp from `brews`⋈`beans` clustered by origin/process/varietal/altitude similarity to the input bean; weighted mean ± 1σ by rating | User briefing (prior-keep); brief Top Workflow #2; Beanconqueror logs brews but has no recipe-recommendation engine |
| T3 | Cellar with freshness windows | `coffee-goat shelf --by freshness` | 10/10 | Selects `beans` with current_mass_g>0, joins `roaster_products`+`reviews`, computes `days_off_roast` against per-method peak windows (espresso 8–21d, filter 5–28d) | User briefing (prior-keep); brief Top Workflow #3; persona P1+P2 daily ritual; no incumbent computes per-method peak windows |
| T4 | Persisted query diff feed | `coffee-goat watch "diego bermudez gesha"` | 10/10 | On `sync`, re-runs each `watchlist` query against `roaster_products` rows updated since the watchlist's `last_sync_anchor`; emits only diffs; cron-safe silent-when-empty | User briefing (prior-keep); persona P1 weekly + P3 daily; agent-shape (P4); no incumbent has cross-roaster restock alerts |
| T5 | Multi-bean compare | `coffee-goat compare <a> <b> [...]` | 9/10 | Selects matching `roaster_products` rows (optional `reviews` join), pivots origin/process/varietal/altitude/price/score into a deltas table | User briefing (prior-keep); brief Top Workflow #4 (paired with twin); cross-roaster compare impossible on any single roaster's site |
| T6 | Closest-current-twin | `coffee-goat twin <bean>` | 10/10 | Weighted similarity vs. every in-stock `roaster_products` row: exact-match on origin/varietal/process + Gower distance on altitude + Jaccard on tags+descriptors; top-N | User briefing (prior-keep); brief Top Workflow #4 flagship; structurally impossible on per-roaster sites |
| T7 | Blind cupping + Coffee Review correlation | `coffee-goat blind-cup --beans 4` | 9/10 | Inserts N hashed cup IDs from `--from-shelf` into a `cuppings` table; accepts SCA scores; reveals on close; computes Spearman ρ vs. `reviews.score` | User briefing (prior-keep); SCA cupping is genre-canonical; calibration cross-source join no incumbent does |
| T8 | FX-normalized $/oz + landed cost | `coffee-goat fx <bean> --ship-to JP` | 9/10 | Daily ECB rate fetch (free, no-auth) cached locally; joins to `roaster_products`'s price/currency/weight to compute USD/oz; layers curated `roaster_shipping` static reference (`// pp:novel-static-reference`) for ship-to surcharge | User briefing (prior-reframe — curated shipping table flagged); P1 explicit pain (24 roasters across continents); no incumbent normalizes cross-currency $/oz |
| T9 | Producer tracking across roasters | `coffee-goat producer "Diego Bermudez"` | 10/10 | Selects from derived `producers` index joined to `roaster_products`, ordered by year/lot; every roaster that has ever carried that producer | User briefing (prior-keep); brief Top Workflow #5; persona P3 primary use case; impossible on a single roaster's site |
| T10 | Refill plan + replacement picks | `coffee-goat refill-plan` | 10/10 | Fits per-bean dose_g/day from `brews`, projects days_remaining = current_mass_g/rate; runs `twin`-style similarity for replacements; ranks by similarity × $/oz × rating | User briefing (prior-keep); P1+P2 weekly; uniquely combines local brew-log analytics with cross-roaster catalog |
| T11 | Roaster style fingerprint + "roasters like X" | `coffee-goat roaster-style "Sey" --similar-to` | 9/10 | Groups `roaster_products` by roaster; per-roaster vectors (origin distribution, process distribution, median $/oz, filter:espresso ratio); cosine distance for `--similar-to` | User briefing (prior-keep); P3 primary; "roasters like Sey" unanswerable elsewhere |
| T12 | Brew-rating drift diagnostic | `coffee-goat drift` | 9/10 | Fixed-effects OLS on `brews.rating ~ days_since(brewed_at) + bean_id` over rolling window; flags negative slope p<0.05; partitions variance by method | User briefing (prior-keep); headline differentiator vs. Beanconqueror; P1+P2 explicit unmet need |
| T13 | "What should I drink next" recommender ⚠ NEW | `coffee-goat whats-next --method v60 --mood fruity` | 10/10 | Selects shelf beans (current_mass_g>0), scores each by (rating affinity = mean rating of similar beans in `brews`) × (descriptor match against `--mood` tag in `reviews.descriptors_json` ∪ `roaster_products.tags`) × (freshness factor) | Subagent-proposed (source a/c); persona P4 signature daily question; cross-source join no incumbent does; agent-shape ideal — **awaiting user approval at Phase Gate 1.5** |

### Killed candidates

| Feature | Kill reason | Closest-surviving |
|---|---|---|
| `harvest <country>` (subagent re-proposed) | User explicitly cut "seasons" in scoping; same concept | n/a |
| `stock-pulse` | Sync-history audit table not in data model; overlap with watch + roaster-style | `watch`, `roaster-style` |
| `vintage <bean> --year` | Pure refactor of producer; redundant standalone | `producer --years N` flag |

## Scope realism — agent self-assessment

**This is a large CLI by Printing Press standards.** Honest scope budget for one session:

| Component | LOC est | Risk |
|---|---|---|
| Internal YAML spec (no upstream OpenAPI) | ~600 | Low — known generator surface |
| Generator scaffold output | ~6000 | Low — printing-press generate handles |
| Shopify adapter (`internal/source/shopify/`) | ~250 | Low — uniform `products.json` schema |
| WooCommerce adapter (Mame) | ~150 | Low — documented Store API |
| Square Online adapter (Loquat) | ~200 | Med — per-site HTML scrape |
| Snipcart adapter (DAK) | ~200 | Med — sitemap walk + data-item-* attrs |
| Body-HTML NLP extractor (origin/producer/process/varietal/altitude) | ~300 | Med — per-roaster variance |
| Coffee Review WP REST adapter | ~150 | Low |
| Sprudge RSS + Chrome cookie | ~120 | Med — Cloudflare cookie freshness |
| 24 roaster registry | ~100 | Low |
| `sync` orchestration | ~200 | Low |
| 13 novel commands (avg ~150 LOC + tests) | ~2400 | Med-Hi — drift/twin/dial-in are non-trivial |
| Tests for non-trivial logic | ~800 | Med |
| Phases 4-5.6 (shipcheck, polish, promote) | n/a | Med — driven by binary |

**Total ~11,500 LOC of generated + hand-authored Go + tests.** Realistic for one session: foundation + 6-8 novel features fully built; remaining 5-7 stubbed with honest "(stub)" messaging per Phase 1.5 stub-marking rule. Bonus: with the press machine doing 6000 LOC of scaffolding, hand-authored is ~5500 LOC, which is achievable end-to-end if scope is bounded.
