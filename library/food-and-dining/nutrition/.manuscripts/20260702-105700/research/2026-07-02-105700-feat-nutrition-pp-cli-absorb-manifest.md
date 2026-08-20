# nutrition-pp-cli Absorb Manifest

Combo (aggregator pattern): **USDA FoodData Central** (OpenAPI, seeds generation) + **NutritionValue.org** (hand-authored HTML source). Peers, no primary. Unified `foods` store keyed by (source, source_id). Shared IDs (NV id == USDA fdcId) make cross-referencing trivial.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword food search (dataType/brandOwner/category filters, sort) | cyanheads usda_search_foods; USDA /foods/search | (generated endpoint) foods search | Offline FTS fallback, --json/--select, DEMO_KEY fallback |
| 2 | UPC/GTIN barcode lookup | NV search accepts UPC; USDA branded gtinUpc | (behavior in nutrition-pp-cli food search) query accepts UPC / --data-type Branded | Works across both sources |
| 3 | Ingredient search / requireAllWords | USDA /foods/search | (behavior in nutrition-pp-cli food search) --require-all-words | — |
| 4 | Get food by id (abridged/full, nutrient subset) | USDA /food/{fdcId}; cyanheads usda_get_food | (generated endpoint) food get | --format abridged/full, --nutrients subset, portion scaling |
| 5 | Batch get (<=20, partial-failure tolerant) | USDA /foods; cyanheads usda_get_foods | (generated endpoint) foods batch | fetch_failures envelope |
| 6 | Paginated list / browse | USDA /foods/list | (generated endpoint) foods list | --limit, dataType/sort |
| 7 | Portion scaling (g/oz/lb/kg/serving/household) | cyanheads usda_get_food; usda-fdc | (behavior in nutrition-pp-cli food get) --grams / --serving | per-100g -> any measure via foodPortions |
| 8 | Static nutrient reference table (name->id) | cyanheads usda_list_nutrients; AiAgentKarl | nutrition-pp-cli nutrients | zero API calls; resolves "vitamin C" -> 1162 |
| 9 | JSON + --select + --compact output | every MCP | (behavior, framework flags) | agent-native |
| 10 | CSV export | Arvind595; NV per-food CSV button | (behavior, framework --csv) | — |
| 11 | Local cache / offline FTS search / SQL | caltui; daveremy; framework | (behavior in nutrition-pp-cli search / sql / sync) | offline, composable |
| 12 | Non-interactive result selection (--first/--index) | Arvind595 future work | (behavior in nutrition-pp-cli food search) --first | scriptable, no TTY |
| 13 | Multi-env key config + DEMO_KEY fallback | caltui 3-tier; asachs01 | (behavior in nutrition-pp-cli doctor) FDC_API_KEY or USDA_API_KEY, DEMO_KEY fallback | runs rate-limited keyless |
| 14 | Food categories / dataType browse | NV categories pages; USDA dataType | (behavior in nutrition-pp-cli food list) --data-type / --category | — |
| 15 | Sources registry (aggregator) | aggregator pattern | nutrition-pp-cli sources list / sources sync | shows USDA + NV, auth needs |

Every absorbed row ships with `--json`, typed exit codes, local caching, and 429 handling.

## Transcendence (only possible with our approach)

7 survivors from the novel-features subagent (customer model: Marcus/powerlifter, Dana/low-carb dieter, Priya/agent builder). All `hand-code`.

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Cross-source enrichment overlay | enrich 173414 | 9/10 | hand-code | USDA /v1/food/{fdcId} + HTML extraction of the NV food page at the same shared id → one merged record: authoritative nutrients + NV-only derived fields (omega-6/omega-3, net carbs, percentile, calories-by-source, amino %DV) | Brief Shared-ID insight + User Vision (NV first-class); NV has only 2 scrapers, none exposing these fields | Use this command to get NutritionValue.org derived analytics (omega ratio, net carbs, percentile, amino-acid %DV) merged onto one food. Do NOT use it for raw nutrient arrays or portion scaling; use 'food get' instead. |
| 2 | NV nutrient-ranking ingestion | rank potassium --order lowest --category vegetables | 8/10 | hand-code | NV precomputed foods_by_<Nutrient>_content.html pages (~60 nutrients, highest/lowest, category filters) via HTML extraction → scriptable top/bottom-N table | Brief workflow 3; NV ranking pages confirmed, no NV tooling exposes them | Use this command for top or bottom foods by a single nutrient. Do NOT use it for compound thresholds like high protein under N kcal; use 'find' instead. |
| 3 | Common-basis compare incl. protein density | compare 173414 171287 175167 --basis 100kcal | 8/10 | hand-code | USDA /v1/foods batch + local per-basis math (100g, serving via foodPortions, per-100kcal) → side-by-side macro table | Brief workflow 2 names per-100kcal protein density; no absorbed competitor offers a compare basis | Use this command to compare 2-5 foods on a common basis (100g, serving, or 100kcal). Do NOT use it for a single food's cross-source detail; use 'enrich' instead. |
| 4 | Compound nutrient filters | find --min protein=20 --max-kcal 165 | 7/10 | hand-code | USDA /v1/foods/search nutrient arrays filtered locally, unioned with synced SQLite foods → foods passing all thresholds | Brief workflow 3 (compound filters); P2 build priority; no absorbed tool supports multi-nutrient thresholds | Use this command for multi-condition nutrient thresholds. Do NOT use it for a simple top-N by one nutrient; use 'rank' instead. |
| 5 | Daily intake log with targets | log add 173414 --grams 150 / log progress | 8/10 | hand-code | Local SQLite (entries + targets) joined with cached food records → today/summary/progress-vs-target reports (local-data command, reimplementation carve-out) | Brief workflow 5 verbatim; caltui proves terminal-tracker demand | Use this command for the persistent daily diary and target tracking. Do NOT use it for a one-off total across several foods; use 'meal' instead. |
| 6 | Stateless meal/recipe aggregation | meal 173414:150g 171287:1cup | 7/10 | hand-code | USDA /v1/foods batch + foodPortions scaling → summed nutrition across N id:quantity pairs; mirrors NV stateless comparefoods pattern | Brief lists NV stateless compare URL; Priya's recipe-total gap; no absorbed tool aggregates across foods | Use this command for a one-shot nutrition total of several foods at given quantities. Do NOT use it to record what you ate; use 'log' instead. |
| 7 | Citation output | cite 173414 --style apa | 6/10 | hand-code | USDA /v1/food/{fdcId} metadata (description, dataType, fdcId, publication date) formatted locally into APA/MLA | Brief Users names agents needing cited numbers; Table Stakes union lists citation, absorb manifest doesn't cover it | none |

**Hand-code count: 7** (all transcendence rows). Absorbed rows are generator-emitted or framework behavior.

### Killed candidates (audit)
cross-source raw macro compare (zero-diff by shared IDs → use enrich); standalone percentile (→ enrich); omega-ratio ranking (→ rank); swap suggester (unverifiable → find); NV community foods/recipes browse (fragile HTML, no weekly ritual → rank); standalone %DV (table stakes in food get; amino %DV in enrich); nutrient-gap suggester (deficit in log progress; suggestion unverifiable → log).

## Source balance (peers)
- NV leads 2 headline novel commands (enrich #1 at 9/10, rank #2 at 8/10) and contributes the derived-analytics overlay. USDA seeds the generated baseline + backs compare/find/meal/cite/log. NV cannot have "more" commands because its value is an enrichment overlay on shared-ID data, not a separate data feed — this is the correct shape for peers, not a demotion.
