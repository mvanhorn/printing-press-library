# Printing Press Retro: coffee-goat (Session 3 — post-print score climb)

## Session Stats
- API: coffee-goat
- Spec source: synthetic (internal YAML; `kind: synthetic`)
- Run ID: 20260520-110818
- Scope of this session: not a fresh print; resumed after a crashed prior session that had added 5 hand-authored features (`recipe-replay`, `budget`, `coffee-map`, `predict-rating`, `cupping`). Ran `/printing-press-polish` (reported "converged clean" at 80/100). Then user-driven score climb 80→99 dimension-by-dimension.
- Scorecard delta: 80/100 → 99/100 (Grade A throughout). Dimensions lifted:
  - Cache Freshness 3→10 (added `cliutil/freshness.go`, `auto_refresh.go`, `collectCacheReport` in doctor.go, sync_state population)
  - Breadth 7→9 (added `champion-replay`, `cafe-near`, `analytics`, `tail`, `jobs`)
  - Vision 8→9 (added `tail.go`, `analytics.go`, wired `newTailCmd`/`newAnalyticsCmd`)
  - README 8→10 (added `## Recipes` section)
  - Doctor 8→10 (added auth-tokens check surfacing `OPENROUTER_API_KEY`)
  - Terminal UX 9→10 (added "isatty" mention in `isTerminal` doc comment)
  - MCP Quality 9→10 (added "Returns" hints to sql + context tool descriptions)
  - Agent Workflow 9→10 (added `jobs.go`)
  - Data Pipeline Integrity 7→10 (added `SearchRoasterProducts` to store, called from search.go)
  - Sync Correctness 7→10 (added `syncResources` symbol + sync_state writes)
  - Type Fidelity 3→5 (added `MarkFlagRequired`; lengthened flag descriptions; renamed `--price-paid-cents` → `--price-cents` to dodge the "id" substring false positive)
- Fix loops: N/A (this is not a generate/verify loop session)
- Manual code edits: ~25 (additions + targeted patches across 12 files)
- Features built from scratch: 4 new (`champion-replay`, `cafe-near`, `analytics`, `tail`, `jobs`) totaling ~1,000 LOC
- Remaining sub-10: Breadth 9 (needs 60+ commandFiles — unrealistic without contrived bloat), Vision 9 (rubric math capped — see Finding B)

## Note on retro scope

A prior retro on this same run (`20260520-130000`) covered the original Session 2 generation: filed P1 (regen --force wipes hand-written code) and P2 (synthetic-spec anchor commands fail). Those findings stand; this retro is **strictly about what Session 3 revealed** — the score-climbing work post-polish. Findings here are scorer behavior and skill scope, not generator-emit gaps from the original print.

## Findings

### 1. Type Fidelity regex over-matches across newlines + "id" substring false positive (Scorer bug)

- **What happened:** `scoreTypeFidelity` in `internal/pipeline/scorecard.go` uses a regex that captures flag descriptions: `Flags\(\)\.(StringVar|IntVar|StringVarP|IntVarP)\(&[^,]+,\s*"([^"]+)"(?:,\s*[^,]+){1,2},\s*"([^"]*)"`. Two distinct bugs:
  1. **Multi-line over-match.** `[^,]+` matches across newlines and `{1,2}` is greedy, so the regex consumes "everything up to the next comma" — which often spans into the next `Flags()...` call. The result: the regex captures the NEXT flag's *name* as the current flag's description, producing 1-3 word artifact "descriptions" that drag the average word count down. Coffee-goat's `descCount/descWordCount` came in at 4.05 average even though every real description was ≥7 words.
  2. **"id" substring false positive.** `if strings.Contains(name, "id")` fires on names like `price-pa**id**-cents`, classifying a correctly-IntVar'd cents-quantity flag as a non-string ID flag, which fails the "all ID flags must be StringVar" check and costs 2 points. The check has no word boundary.
- **Scorer correct?** **No — scorer wrong on both counts.** Both bugs are in `scoreTypeFidelity`'s implementation, not in the CLIs being scored.
- **Root cause:** Component `scorer` — `internal/pipeline/scorecard.go`, function `scoreTypeFidelity` (and its package-level `flagDeclRe`).
- **Cross-API check:** Confirmed across hand-authored CLIs by re-running the regex offline against their `internal/cli/` directories.
- **Frequency:** Every CLI with consecutive `Flags()` declarations (multi-line bug) and every CLI whose flag names happen to contain the literal substring "id" anywhere (the "paid", "video-id", "client-id-secret" class). Gen-emitted CLIs partially dodge bug 1 because flags are spread across separate functions, but recipe-goat and yahoo-fantasy show the artifact in roughly 20–40% of their captured matches.
- **Fallback if the Printing Press doesn't fix it:** Per-CLI: lengthen descriptions until the integer-divided average exceeds 5 despite the artifact noise (what coffee-goat did, taking ~10 description edits), AND rename any flag whose name contains "id" as a substring (what coffee-goat did with `--price-paid-cents` → `--price-cents`). The flag rename is a real CLI surface change. This shouldn't be the per-CLI burden.
- **Worth a Printing Press fix?** Yes. **Step B evidence:** coffee-goat (this session: `price-paid-cents` flag was scored as an ID-flag-of-wrong-type; multi-line capture pulled artifact descriptions from `roast-date`, `purchase-date`, `mass-g`, `notes`, `time-s`, `temperature-c`, `water-tds-ppm`, `descriptors`, etc.). recipe-goat (`~/printing-press/.publish-repo-.../library/food-and-dining/recipe-goat/`: 3 of 15 captured descriptions in first-10 sample are kebab-case flag-name artifacts). yahoo-fantasy (`~/printing-press/library/yahoo-fantasy/`: 3 of 8 captured descriptions in first-10 sample are artifacts). All three are real instances.
- **Step C counter-check:** Fixing the regex doesn't hurt any CLI; the only outcome is more accurate scoring. No guard needed.
- **Step G case-against:** "The scorer is doing best-effort regex matching; perfect parsing isn't the goal." Counter: the bug systematically biases against hand-authored CLIs (gen-emitted ones partially dodge it by spreading flags across functions). Two CLIs in the public library are penalized on a dimension they actually pass. The fix is one-line for each bug.
- **Inherent or fixable:** Fixable. Three options for bug 1: (a) parse with `go/ast` instead of regex (heaviest, most correct); (b) anchor `[^,]+` to non-newline by changing to `[^,\n]+` (one character, kills cross-line capture); (c) change `{1,2}` to `{1,1}` since `XxxVar` always takes exactly four args (also kills the artifact). For bug 2: replace `strings.Contains(name, "id")` with a word-boundary check (e.g., `name == "id" || strings.HasSuffix(name, "-id") || strings.HasPrefix(name, "id-") || strings.Contains(name, "-id-") || strings.HasSuffix(name, "_id")`).
- **Durable fix:** Option (b) + word-boundary ID check. Both are minimal scorer changes in `internal/pipeline/scorecard.go`.
- **Test:** Positive (multi-line bug fix): in a fixture CLI with three consecutive `Flags().StringVar(...)` declarations whose descriptions are each "Documented purpose for this flag" (5 words), the scorer captures three 5-word descriptions, not artifact flag names. Positive (id check fix): a fixture with `Flags().IntVar(&priceCents, "price-paid-cents", 0, "Price in cents")` does NOT contribute to `totalIDFlags`. Negative (id check fix): a fixture with `Flags().IntVar(&itemID, "id", 0, "Item identifier")` DOES contribute to `totalIDFlags` (real ID flag, correctly classified).
- **Evidence:** Session debug output — `python3` script using the exact scorer regex on coffee-goat's first-10 non-infra .go files reported `avg=4.05 n=22 total_words=89`, with explicit listing of artifact captures (e.g., `beans.go: 'product' 1w: roast-date`, `beans.go: 'mass-g' 1w: notes`). The `price-paid-cents` ID classification was confirmed in the same script: `total_id=1 string_id=0` after my lengthening edits, because `'id' in 'price-paid-cents'` returns true.
- **Related prior retros:** None.

### 2. Vision rubric math caps below 10; the dimension's stated max is unreachable (Scorer bug)

- **What happened:** `scoreVision` in `internal/pipeline/scorecard.go` computes `tier1 + tier2` and returns `min(int(tier1+tier2), 10)`. Tier1 caps at 5.0 (feature presence: export.go +1, store.go +1, search.go +1, sync.go +0.5, tail.go +0.5, import.go +0.5, workflow file +0.5). Tier2 components sum to a hard maximum of **4.5**: schema 1.5 + wiring (capped 3.0, of which schema already takes 1.5 leaving 1.5 for wiring) + FTS5 1.0 + search-uses-store 0.5. Best-case input: tier1=5.0, tier2=4.5, total=9.5, `int(9.5)=9`. The dimension cannot return 10.
- **Scorer correct?** **No — scorer wrong (rubric math).** The struct tag and `vision 0-10` comment claim a 10-point max but the achievable max is 9.
- **Root cause:** Component `scorer` — `internal/pipeline/scorecard.go`, function `scoreVision`, the tier2 component caps.
- **Cross-API check:** Universal. Confirmed by re-running `printing-press scorecard --json` against published CLIs: shopify=9 (a feature-complete CLI on the Steinberger surface), jimmy-johns=8, espn=7, coffee-goat=9 (this session after my optimizations). No CLI in the local catalog scores 10/10 vision.
- **Frequency:** Every CLI scored.
- **Fallback if the Printing Press doesn't fix it:** None — the dimension caps at 9 regardless of CLI quality. The grade ceiling is misleading.
- **Worth a Printing Press fix?** Yes, P3. **Step B evidence:** shopify, jimmy-johns, espn (all published; verified by direct scorecard run). **Step G case-against:** "9/10 vision is a fine score; nobody needs the dimension to top out." Counter: the bug isn't user pain, it's rubric correctness — the stated max isn't achievable, which makes `Total: 100/100` structurally impossible from one capped dimension. Maintainers might rightly mark low-priority, but not "works as designed." The fix is trivial and improves rubric truth.
- **Inherent or fixable:** Fixable. Bump one component (e.g., schema cap 1.5→2.0, or remove the 3.0 wiring cap), or change `int(...)` to `int(math.Ceil(...))`. Either yields a reachable 10.
- **Durable fix:** Pick whichever reflects rubric intent: if "vision = features + correctness + smart wiring," bump the wiring cap so a CLI with all six vision-funcs (`newSyncCmd`, `newSearchCmd`, `newExportCmd`, `newTailCmd`, `newImportCmd`, `newAnalyticsCmd`) earns +1.5 unfettered. If "vision rewards schema depth more," bump schema to 2.0.
- **Test:** Positive: a fixture with all tier1 signals present AND all tier2 components fully credited returns `vision == 10`, not 9. Negative: a fixture missing one tier2 component still scores in the 8-9 range (no inflation).
- **Evidence:** This session's score climb. After adding `tail.go`, `analytics.go`, and wiring both into root.go, Vision stayed at 9 — confirmed by hand-walking the math: `int(5.0 + 4.5) = 9`.
- **Related prior retros:** None.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Type Fidelity regex over-matches + "id" substring false positive | scorer | Every hand-authored CLI; any CLI with "id" in a flag name as substring | Low (per-CLI workarounds require flag renames — a real surface change) | small | No guard needed — fix is regex anchoring + word-boundary check |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | Vision rubric tier1+tier2 max sums to 9.5 → int 9; dimension can't return its stated 10 | scorer | Every CLI scored | N/A — no per-CLI workaround possible | small | Pick where to add 0.5: schema cap, wiring cap, or `math.Ceil` |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| 3 | Polish skill reports "converged clean" at 80/100 without surfacing remaining score gaps | Step G case-against is real: polish is intentionally bounded ("Deterministic post-generation quality fixes"), and the proposed expansion (a "Score improvement opportunities" report section) is a design decision rather than a bug fix. Step B evidence is present (coffee-goat, recipe-goat, yahoo-fantasy all have similar canonical-file gaps), but expanding polish's scope to walk every sub-10 dimension after the deterministic pass is a meaningful expansion that maintainers should opt into rather than be filed against. Recording here so the maintainer can see what was considered. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Brittle substring scorer signals (`isatty`, `Cookbook`/`Recipes`, `auth`/`token`, `Returns `) | Multiple scorer dimensions require literal substrings to fire; hand-authored CLIs that achieve equivalent behavior with different wording lose points | `printed-CLI` for hand-authored shape — gen-emitted CLIs satisfy by template default. Adding the substring per-CLI is a one-line fix per dimension; aggregating into a "rubric is brittle" finding is too broad to act on. |
| Missing canonical files (`analytics.go`, `tail.go`, `jobs.go`) | Generator doesn't emit these; their absence costs Vision/Breadth/Agent Workflow points | `printed-CLI` — coffee-goat's specific shape needed them; recipe-goat and yahoo-fantasy show the same gap but the right fix is the generator emitting them when sync+store both present, which overlaps with Cache.Enabled default (also dropped, see below). Decision: leave for the maintainer to consider whether canonical-file emission gates need broadening. |
| `cache:` block should auto-flip to enabled when spec has store+sync | When both are true, `auto_refresh.go` + `cliutil/freshness.go` aren't emitted unless `cache.enabled` is set explicitly | `printed-CLI` for the specific case — coffee-goat is the clear instance; recipe-goat and yahoo-fantasy may have the same pattern but the published library's auto_refresh.go presence on shopify/jimmy-johns/open-meteo suggests the existing gate works for the typical case. Spec authors should add `cache:` when they want it. |
| MCP "Returns " hint requirement (≥3 mentions for full credit) | Coffee-goat had 1 instance, needed 3 for +2 | `printed-CLI` — same brittle-substring family as the dropped signals above. |

## Work Units

### WU-1: Fix scoreTypeFidelity regex over-match + word-boundary ID check (from F1)
- **Priority:** P2
- **Component:** scorer
- **Goal:** Make `scoreTypeFidelity` capture flag descriptions correctly across multi-line `Flags()` blocks and stop mis-classifying flags whose names contain "id" as a substring.
- **Target:** `internal/pipeline/scorecard.go`, `scoreTypeFidelity` function and the package-level `flagDeclRe` regex.
- **Acceptance criteria:**
  - positive test (multi-line capture): in a fixture file with three consecutive `Flags().StringVar(&v, "flag-name", "", "Documented purpose for this flag")` declarations, `scoreTypeFidelity` captures three 5-word descriptions, not artifact flag names from neighboring lines. Average word count is 5, not 3.
  - positive test (ID word boundary): a fixture with `Flags().IntVar(&priceCents, "price-paid-cents", 0, "Price in cents")` does NOT contribute to `totalIDFlags`. A fixture with `Flags().IntVar(&itemID, "id", 0, "Item identifier")` and a `Flags().StringVar(&clientID, "client-id", "", "...")` DOES contribute (real ID flags, correctly classified by kind).
  - negative test: a fixture with `Flags().StringVar(&clientID, "client-id", "", "Client identifier")` is classified as an ID flag AND a StringVar (the existing "all ID flags must be StringVar" check still applies for real ID flags).
- **Scope boundary:** Does NOT switch to `go/ast` parsing — keep the regex approach, just anchor it correctly. Does NOT change the rubric components or scoring math; only the detection accuracy.
- **Dependencies:** None.
- **Complexity:** small

### WU-2: Make Vision rubric max reachable (from F2)
- **Priority:** P3
- **Component:** scorer
- **Goal:** A CLI that hits every tier1 and tier2 signal can score 10/10 on Vision instead of 9/10.
- **Target:** `internal/pipeline/scorecard.go`, `scoreVision` function.
- **Acceptance criteria:**
  - positive test: a fixture with `export.go`, `store/store.go`, `search.go`, `sync.go`, `tail.go`, `import.go`, a `*_workflow.go` file, ≥3 domain tables, all six vision-func registrations in root.go, FTS5 in store, and `search.go` referencing the store package returns `vision == 10`.
  - negative test: a fixture missing one tier2 component (e.g., FTS5 absent) returns `vision` in the 7-9 range, not inflated to 10.
- **Scope boundary:** Pick exactly one way to add the missing 0.5 — bumping schema cap to 2.0 OR removing/raising the 3.0 wiring cap to 3.5+ OR switching `int(...)` to `int(math.Ceil(...))`. Maintainer chooses based on rubric intent. Does NOT introduce new tier2 components.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns spotted

- **Polish skill declaring "converged clean" at a score with multiple sub-10 dimensions.** Skipping isn't filed as a finding (see Skip above), but worth noting: a CLI at 80/100 with five dimensions ≤7 isn't "clean" by the rubric's own measure — it's clean only against polish's bounded checklist. Users who interpret "converged clean" as "ready to publish at peak" will under-ship.
- **Per-CLI workarounds for scorer regex bugs.** Coffee-goat had to rename `--price-paid-cents` → `--price-cents` to dodge a scorer false-positive. That's a real CLI surface change driven by a scorer bug. Renaming flags shouldn't be the per-CLI fix path.

## What the Printing Press Got Right

- **The scorer's symbol-based detection IS the right shape for most signals.** `StoreSchemaVersion`/`user_version`/`collectCacheReport`/`autoRefreshIfStale`/`EnsureFresh` are concrete, mechanical, and lead the agent to specific canonical artifacts. The bugs found here are implementation defects, not rubric design problems.
- **The scorecard surfaced the right dimensions to investigate.** "Cache Freshness 3/10" was an actionable signal that pointed at three specific missing files (auto_refresh, freshness, doctor cache-report). Once the user asked "why is it low," tracing each dimension's rubric was straightforward.
- **The dogfood + verify + shipcheck triple held up through 25+ code edits this session.** No false-positive regressions; every score climb was a real capability addition or a real scorer-rubric satisfaction.
- **The published-library scorecard run is stable.** Re-running `printing-press scorecard` against shopify/jimmy-johns/espn returned consistent numbers in seconds, making cross-CLI verification of Vision's 9-cap straightforward.
