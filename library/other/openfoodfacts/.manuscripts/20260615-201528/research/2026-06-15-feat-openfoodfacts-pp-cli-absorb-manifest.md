# Open Food Facts CLI — Absorb Manifest

Sources catalogued: openfoodfacts-python (official SDK), openfoodfacts-nodejs (official, most complete feature map), JagjeevanAK/OpenFoodFacts-MCP, nagarjun226/food-tracker-mcp, openfoodfacts/labelr, Robotoff, angristan/openfoodfacts-api-c. Competitor table-stakes from USDA FoodData Central / Nutritionix / Edamam.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get product by barcode (v2) | python `product.get`, all tools | `lookup <code>` typed endpoint | `--json`/`--select`/`--compact`, local cache, typed exit codes |
| 2 | Get product by barcode (v3) | nodejs `getProductV3` | `lookup <code> --api-version 3` | Richer v3 shape, same flags |
| 3 | Sparse fieldsets | OFF `fields=` param | `--fields name,nutriments,nutriscore` | Cuts payload, saves agent context |
| 4 | Multi-barcode batch | (gap in ecosystem) | `lookup <c1> <c2> ...` | No tool does clean batch; we do |
| 5 | Text search | nodejs `search`, python `text_search` | `search "<query>"` | Offline FTS fallback |
| 6 | Tag filters (category/brand/label/country/store/additive/allergen) | OFF `_tags` params | `search --category --brand --label --no-additive ...` | Composable flags |
| 7 | Nutrient-range filters | OFF `nutrient_lt/gt` | `search --max-sugar 5 --min-protein 10` | Friendly flag names over wire keys |
| 8 | Sort results | OFF `sort_by` | `search --sort popularity\|nutriscore\|created` | — |
| 9 | Pagination | OFF `page`/`page_size` | `--page --limit` | — |
| 10 | Suggest / autocomplete | OFF taxonomy_suggestions | `suggest <prefix> --taxonomy categories` | — |
| 11 | Taxonomy fetch | nodejs `taxonomy/` | `taxonomy <type>` | Cached locally |
| 12 | Attribute groups | OFF `/attribute_groups` | `attribute-groups` | — |
| 13 | Knowledge panels | nodejs `knowledgepanels` | `lookup <code> --panels` | — |
| 14 | Facets browsing | nodejs `facets` | `facets <type>` (values + counts) | — |
| 15 | Nutri-Score | MCP analysis | parsed field in `lookup`/`search` | Explained grade A-E |
| 16 | NOVA group | MCP analysis | parsed field | 1-4 with label |
| 17 | Eco/Green-Score | OFF | parsed field | — |
| 18 | Additives (E-numbers) | OFF | parsed list | — |
| 19 | Allergens & traces | OFF | parsed list | — |
| 20 | Vegan/vegetarian/palm-oil flags | OFF | parsed flags | — |
| 21 | Nutriments per-100g + per-serving | USDA/Nutritionix | normalized table | Both bases, one command |
| 22 | Serving-size / quantity parsing | python | `--per serving\|100g` | — |
| 23 | Product image URLs | OFF | `lookup --images` | front/ingredients/nutrition/packaging |
| 24 | Image OCR text | python OCR | `lookup --ocr` | — |
| 25 | Barcode normalize (pad 8/13) | python `normalize_barcode` | `barcode normalize <code>` | — |
| 26 | Barcode GTIN check-digit | python `has_valid_check_digit` | `barcode validate <code>` | Local, offline, instant |
| 27 | Flavor switch (off/obf/opff/opf) | python `Flavor` | `--flavor` global flag | Beauty/petfood/products too |
| 28 | Country/locale (cc/lc) | python `Country` | `--country --lang` | — |
| 29 | Env switch (prod/staging) | python `Environment` | `--env prod\|staging` | Staging basic-auth handled |
| 30 | API version (v2/v3) | python `APIVersion` | `--api-version` | — |
| 31 | JSON / table / CSV output | wrappers | `--json --csv` + default table | — |
| 32 | Mandatory User-Agent | all SDKs | auto-default UA + `doctor` warns if unset | Reachability-critical, baked in |
| 33 | Client rate limiting + backoff | (CLI gap) | adaptive limiter on 429/503 | None of the toy CLIs do this |
| 34 | Local response caching | (CLI gap) | SQLite cache | Dodges rate limit |
| 35 | Sync to SQLite | (gap) | `sync` by category/tag/barcode list | Foundation for offline + diary |
| 36 | FTS offline search | (gap) | `search --local` / FTS5 | Bypasses rate limit entirely |
| 37 | SQL over local store | framework | `sql "<query>"` | Composable analytics |

No stubs in the absorbed set — all are read-only, no-auth, freely testable. Write/update product and image upload (auth-gated) are explicitly OUT of scope, not stubbed.

## Transcendence (only possible with our approach)

Full customer model, candidate list, and kill log: `2026-06-15-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Local macro diary + goal | `diary add <barcode> --servings N` / `diary today` / `diary goal` | 9/10 | Local `diary_entry`+`goal` SQLite join; OFF has no diary. MyFitnessPal's core loop, free + offline + scriptable. |
| 2 | Range diary report | `diary since <date>` | 7/10 | Time-windowed aggregation over local diary rows — no single API call produces it. |
| 3 | Side-by-side compare | `compare <code1> <code2> [...]` | 8/10 | Multi-product client-side score/nutriment table; no OFF tool does compare. |
| 4 | Healthier-alternative swap | `swap <barcode> [--max-nova 3]` | 8/10 | Category lookup + Nutri-Score-sorted search + mechanical rank; no tool answers "healthier in same category". |
| 5 | Recipe/meal aggregation | `recipe <code...> --servings ...` | 7/10 | Sums per-serving nutriments across N products into recipe + per-serving JSON. |
| 6 | Allergen-profile alert | `allergens set <list>` + `lookup --check-allergens` | 8/10 | Set-intersect product allergens/traces vs stored profile, non-zero exit on HIT. Scriptable allergen gate no app offers. |
| 7 | Diet-budget search | `search --remaining-from-diary` | 6/10 | Injects (goal − logged macros) as nutrient-range filters; cross-entity join of local diary state into live query. |
| 8 | Offline nutriment ranking | `rank --category <c> --sort <nutrient>` | 6/10 | Pure local SQLite query over synced store; runs offline, dodges the rate limit. |

No stubs in transcendence set. All buildable read-only/local; testable without credentials.
