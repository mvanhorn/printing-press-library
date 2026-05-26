# coffee-goat Absorb Manifest — Session 2

> Extends `research/prior-absorb-manifest.md`. Session 1's 13 absorbed-feature rows and 13 transcendence rows stay intact. This document documents the 5 new absorbed rows and 17 new transcendence rows added when the user broadened scope in Session 2.

## Absorbed (match or beat everything that exists)

### Session 1 absorbed (carried over)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Per-roaster bean browse | each roaster's site / Beanconqueror | Cross-roaster `roaster_products` synced corpus | Browse all 24 at once |
| 2 | Origin / process / varietal filter | each roaster's site | SQL on synced corpus | Filter across the global shelf |
| 3 | Price display in user currency | each roaster | `/products.json` variants + currency | One unit across continents |
| 4 | Roast date / freshness indicator | each roaster (variable) | `published_at` + parsed `roast_date` | Normalized across sources |
| 5 | Coffee Review score lookup | coffeereview.com | `reviews` table via WP REST sync | Offline, joinable |
| 6 | Brew log (30+ params) | Beanconqueror, Brewlog | `brews` table | Agent-native; cross-CLI portable |
| 7 | Method profiles | Beanconqueror | `method_profiles` table | Local, versioned |
| 8 | Water profiles | Beanconqueror | `water_profiles` table | Local, versioned |
| 9 | SCA cupping form | Beanconqueror / paper | `cupping_sessions` table | Structured, queryable |
| 10 | Ratings & notes | Beanconqueror | ratings JSON on brews | Composable in SQL |
| 11 | Cellar / inventory | Beanconqueror | `beans` table | Joins cleanly with brews + reviews |
| 12 | Cross-roaster sync orchestration | none (the absorbed gap) | `sync.go` with `cliutil.AdaptiveLimiter` | Per-roaster rate limiting |
| 13 | LLM bag-scan (opt-in) | n/a | `scan` command, requires `OPENROUTER_API_KEY` | Optional convenience |

### Session 2 additions

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 14 | YouTube creator review lookup | YouTube (Hoffmann/Hedrick) | `youtube_reviews` table populated via `youtube-pp-cli videos-transcript` subprocess + RSS feed for discovery (no-auth) | Cross-creator unified table |
| 15 | World Coffee Championship winners | wcc.coffee (current) / Wikipedia (historical) | `champion_recipes(year, competition, rank, competitor, country, …)` | Joinable with roaster_products |
| 16 | Champion bean / recipe metadata | Sprudge editorial cross-ref | Same table, partial-fill tolerant | 60–70% coverage acknowledged |
| 17 | Sprudge city coffee guides | sprudge.com/guides | `cafe_guides` table (city × source_url) | Curated specialty layer |
| 18 | Cafes near location | OSM Overpass | `cafes(name, address, lat, lng, source, source_url, tags)` populated on demand | Breadth, no-auth |

## Transcendence (only possible with our approach)

### Session 1 transcendence (13 rows; carried over from prior approval)

| # | Feature | Command | Why Only We Can Do This | Score | Personas |
|---|---|---|---|---|---|
| 1 | Cross-roaster FTS search | `search` | Local FTS5 over 24 synced roasters; no single storefront covers others | 10 | P1, P3, P4 |
| 2 | Bayesian dial-in | `dial-in` | Joins personal `brews` with global `roaster_products` clusters | 10 | P1, P2 |
| 3 | Cellar + freshness | `shelf` | Per-method peak freshness curve overlaid on user roast dates | 10 | P1, P2, P4 |
| 4 | Restock watch | `watch` | Sync-anchored diff over cross-roaster corpus | 10 | P1, P3 |
| 5 | Multi-bean compare | `compare` | Cross-roaster + Coffee Review delta table | 9 | P1, P3 |
| 6 | Closest twin | `twin` | Structured-attribute + descriptor similarity across 24 roasters | 10 | P1, P3, P4 |
| 7 | Blind cupping calibration | `blind-cup` | Spearman of user ratings vs Coffee Review scores | 9 | P1 |
| 8 | FX landed cost | `fx` | ECB FX + curated shipping table (`// pp:novel-static-reference`) | 9 | P1 |
| 9 | Producer tracking | `producer` | Cross-roaster producer index over years | 10 | P3, P1 |
| 10 | Refill plan | `refill-plan` | Consumption × twin × shelf depletion | 10 | P1, P2, P4 |
| 11 | Roaster style fingerprint | `roaster-style` | Cosine over per-roaster origin/process/price aggregates | 9 | P3 |
| 12 | Rating drift diagnostic | `drift` | Fixed-effects OLS on rating vs day across beans | 9 | P2, P1 |
| 13 | What to drink next | `whats-next` | Joins shelf freshness × dial-in confidence × user palate | 9 | P1, P2, P4 |

### Session 2 transcendence (19 new rows — includes 2 user adds at the gate)

| # | Feature | Command | Why Only We Can Do This | Score | Personas |
|---|---|---|---|---|---|
| 14 | Creator review lookup | `creator-review <bean>` | Joins `youtube_reviews` (Hoffmann/Hedrick) with `roaster_products`; no incumbent indexes creators against bags | 9 | P1, P4 |
| 15 | God cup recommender | `god-cup` | 5-table join (shelf + brews + reviews + youtube_reviews + champion_recipes) producing one brew + one buy pick | 10 | P1, P4 |
| 16 | Championship replay | `champion-replay <year>` | Joins WBC champion recipe with current in-stock matching lots | 9 | P1, P3 |
| 17 | Champion lineage | `champion-lineage --producer X` | 4-table cross-entity join (champion_recipes ⋈ producers ⋈ roaster_products ⋈ youtube_reviews) | 8 | P3 |
| 18 | Creator radar | `creator-radar` | Cron-safe diff over newly-synced creator videos mentioning tracked roasters/watchlist | 8 | P1, P4 |
| 19 | Cafe near | `cafe-near "<location>"` | Overpass + Sprudge city-guide overlay in one shot | 8 | P4 |
| 20 | Cafe trip planner | `cafe-trip "<city>"` | Sprudge guide curation + Overpass density + nearby HQ roasters | 7 | P1, P3 |
| 21 | Palate map | `palate-map` | Per-user descriptor signature learned from 8+ rated brews | 8 | P1, P2 |
| 22 | Bag lifecycle | `bag-life <bean>` | Personal rating curve vs days-since-roast for one bag | 8 | P2 |
| 23 | Espresso school | `espresso-school <bean>` | Process-keyword density ranking over Hedrick transcripts | 7 | P2 |
| 24 | Review gap | `review-gap` | Anti-join: shelf bags with no review and no creator coverage | 7 | P1 |
| 25 | Review consensus | `review-consensus <bean>` | One-row card aggregating Coffee Review + Hoffmann + Hedrick + champion + user rating | 8 | P1, P4 |
| 26 | Producer discovery | `producer-discovery` | Producers at 3+ roasters but absent from editorial — early signal | 8 | P3 |
| 27 | Roaster benchmark | `roaster-bench <slug>` | Quarter-over-quarter KPI delta per roaster | 7 | P3 |
| 28 | Transcript search | `transcript-search "<q>"` | Local FTS5 over all Hoffmann+Hedrick transcripts | 7 | P1, P4 |
| 29 | Champion shop | `champion-shop` | Match every recent champion recipe to current in-stock lots | 8 | P3, P1 |
| 30 | Personal roast window | `roast-window` | Learns user's actual per-method peak window from their brews | 7 | P1, P2 |
| 31 | Friend-pick (palate sharing) | `friend-pick <friend>` | Imported friend palate profile + cross-roaster corpus → bag pick for them, not you | 8 | P1, social |
| 32 | SCA Flavor Wheel mapping | `flavor-wheel` | Maps your brew ratings onto the official SCA Coffee Tasters' Flavor Wheel hierarchy; complements palate-map by descriptor instead of origin/process | 8 | P1, P2 |

## Phase Gate 1.5 trim — final approved scope (21 transcendence features)

User approved the trim from 32 to 21 features. Cut list with rationale:

| # | Cut feature | Reason | Closest survivor |
|---|---|---|---|
| 1 | `creator-radar` | Covered by `watch` + sync cadence | `watch` |
| 2 | `espresso-school` | Covered by `transcript-search --creator hedrick` | `transcript-search` |
| 3 | `review-gap` | Niche analytical; god-cup surfaces gaps implicitly | `god-cup`, `palate-map` |
| 4 | `review-consensus` | `god-cup` already aggregates every signal source | `god-cup` |
| 5 | `champion-lineage` | Overlaps with `producer` + `champion-replay` together | `producer`, `champion-replay` |
| 6 | `champion-shop` | Same join shape as `champion-replay` | `champion-replay` |
| 7 | `cafe-trip` | `cafe-near` covers the headline cafe workflow | `cafe-near` |
| 8 | `producer-discovery` | Devon-persona-specific; not in user's stated headline workflows | (Devon use case folds into `producer` queries) |
| 9 | `roaster-bench` | Devon-persona-specific competitive intel | (deferred) |
| 10 | `roaster-style` | Devon-persona-specific fingerprinting | (deferred) |
| 11 | `roast-window` | Overlaps with `shelf` per-method windows + `drift` | `shelf`, `drift` |

**Final approved transcendence list (21):** god-cup, search, watch, twin, compare, producer, fx, dial-in, shelf, whats-next, drift, refill-plan, blind-cup, palate-map, bag-life, friend-pick, flavor-wheel, creator-review, transcript-search, champion-replay, cafe-near.

**Locked groups for README/SKILL rendering:**
- The God cup (1): god-cup
- Cross-source corpus (6): search, watch, twin, compare, producer, fx
- Personal-history analytics (10): dial-in, shelf, whats-next, drift, refill-plan, blind-cup, palate-map, bag-life, friend-pick, flavor-wheel
- Editorial signal (2): creator-review, transcript-search
- Championship (1): champion-replay
- Discovery (1): cafe-near

**Persona update from gate round 2:** added Persona 5 — Mike, espresso enthusiast with Decent DE1. Served by existing espresso-focused features (`god-cup --method espresso`, `drift`, `bag-life`, `champion-replay`). Decent DE1 shot-file integration remains v0.2 per Session 1.

**Persona 1 method detail:** V60, Origami Air, Sibarist SD-1 (filter), Oxo Rapid Brewer (immersion). Seeded into `method_profiles` in Phase 3.

**Phase 3 build priority addition:** frictionless `beans add` UX — accept a roaster URL, fuzzy product slug, or interactive selection. UX polish on the existing absorbed `beans` resource (not a transcendence row).

**0 stubs — all 21 features ship with full implementation.**

**Persona update:** added Persona 5 — Mike, espresso enthusiast with Decent DE1. Served by existing espresso-focused features (`god-cup --method espresso`, `espresso-school`, `drift`, `bag-life`, `champion-replay`, `roast-window`). Decent DE1 shot-file integration remains v0.2 per Session 1.

**Persona 1 method detail:** V60, Origami Air, Sibarist SD-1 (filter), Oxo Rapid Brewer (immersion). Seeded into `method_profiles` in Phase 3.

**Phase 3 build priority addition:** frictionless `beans add` UX — accept a roaster URL, fuzzy product slug, or interactive selection. Not a new transcendence row; UX polish on the existing absorbed `beans` resource.

## Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `shelf-sprudge` | Brief explicitly states cafe→bean linkage is not viable | `cafe-trip` |
| `brew-twin` | Redundant with `twin` + `whats-next` together | `twin`, `whats-next` |

## Scope notes (review before approving)

- **30 transcendence features is a large CLI.** Each gets a Cobra command file + (for non-trivial features) a helper package + tests. Reasonable estimate: ~3,000-4,500 lines of hand-written Go for the new transcendence features alone, on top of the ~1,500 lines for Session 1's transcendence features already designed but not built.
- **No stubs.** Every row above ships as a working feature. If implementation becomes infeasible during Phase 3, the agent must return to Phase 1.5 with a revised manifest (per skill rule).
- **Hardware integrations (La Marzocco Home cloud, Decent DE1+) remain deferred** per Session 1 decision.
- **Cafe → bean linkage is impossible** with no-auth sources. `cafe-near` and `cafe-trip` return cafe locations only; no claim of "this cafe serves this bean."
- **WBC recipe data is sparse.** `champion-replay` and `champion-shop` must tolerate partial-fill rows (60-70% recipe completeness for top finalists since ~2018; older years much sparser).
- **YouTube transcript bean-mention extraction is noisy.** First-pass regex over the 24-roaster slug set; precision over recall to avoid false attributions. If precision is unacceptable, fall back to LLM extraction only with explicit user opt-in (matches the existing `scan` opt-in pattern via `OPENROUTER_API_KEY`).
- **youtube-pp-cli must be installed.** `doctor` will detect and emit the install command. Adding it as a hard runtime prerequisite is correct given the user explicitly named it.
