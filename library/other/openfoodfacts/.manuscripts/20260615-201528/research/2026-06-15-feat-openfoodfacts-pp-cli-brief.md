# Open Food Facts CLI Brief

> Built as the FatSecret-equivalent the user requested: a free, no-auth, open-data
> food + nutrition database CLI. FatSecret's consumer surface (food search, barcode
> scan, calorie/macro diary) rebuilt on Open Food Facts open data, with a local
> SQLite diary layered on top.

## API Identity
- **Domain:** Food / nutrition. Product database keyed by barcode (GTIN). 3M+ foods, branded + generic, 56 countries.
- **Base:** `https://world.openfoodfacts.org` (prod), `https://world.openfoodfacts.net` (staging, basic auth off/off).
- **Auth:** NONE for reads. Writes need session cookie / user+password. We build the read CLI; write commands are out of scope or stub.
- **Users:** People who want nutrition facts without an account or a paid API key. Open-data equivalent of FatSecret/MyFitnessPal food DB.
- **Data profile:** Product (barcode) -> nutriments (per-100g + per-serving), categories, brands, ingredients, allergens, additives (E-numbers), Nutri-Score, NOVA group, Eco/Green-Score, images, knowledge panels.

## Reachability Risk
- **High (rate-limit, not block).** OFF enforces ~15 req/min/IP product reads, 10 req/min/IP search. HTTP 503 on global overload, per-IP throttling otherwise. Crawlers get banned; OFF points bulk users to the data dump.
- **Mandatory User-Agent.** Format `AppName/Version (ContactEmail)`. Missing UA risks bot-block. Every official SDK requires it.
- **Mitigations the CLI MUST ship:** (1) default a well-formed configurable User-Agent, warn if unset; (2) client-side rate limiter + backoff on 429/503; (3) local SQLite cache + offline search so repeat queries don't re-hit the API; (4) staging-server flag for testing.
- Evidence: OFF issue #8818 (rate-limit policy), FoodYou app #232 (had to add a limiter), openfoodfacts-dart #941 (tests hit limits).

## Users (concrete personas)
1. **The label-checker at the supermarket.** Standing in an aisle, types/scans a barcode, wants kcal + Nutri-Score + additives + allergens in one glance before buying. Today: opens the FatSecret/Yuka app, taps through ad-laden screens. Can't pipe the result anywhere.
2. **The macro-tracking home cook.** Logs what they eat across the day, wants running kcal/protein/fat/carbs totals vs a daily goal. Today: MyFitnessPal (paywalled features, account required) or a spreadsheet. Can't do it offline or scriptably.
3. **The data-minded dieter comparing products.** "Is the store-brand cereal better than the name-brand?" Wants two products side-by-side on Nutri-Score/NOVA/sugar. Today: two browser tabs, eyeballing. No tool answers "find a healthier alternative in the same category."
4. **The agent / automation author.** Wants clean JSON nutrition data for an LLM meal-planner or a nutrition pipeline, without OAuth and without burning rate limit. Today: hits the raw API, gets throttled, parses verbose payloads by hand.

## Top Workflows (named rituals)
1. **Scan-and-judge:** `lookup <barcode>` -> kcal/100g, Nutri-Score, NOVA, additives, allergens, compact. The daily supermarket ritual.
2. **Search-the-shelf:** `search "greek yogurt" --no-palm-oil --max-nova 3` -> ranked candidates with scores. Find a product without a barcode in hand.
3. **Log-and-total (diary):** `diary add <barcode> --servings 1.5` then `diary today` -> running macro totals vs goal. The MyFitnessPal core loop, local + free.
4. **Compare-before-buy:** `compare <code1> <code2>` -> per-100g + score table. The name-brand-vs-store-brand decision.
5. **Sync-and-search-offline:** `sync` a category/favorites to SQLite, then `search`/`sql` offline — directly dodges the rate limit.

## Table Stakes (must match every competitor)
- Barcode lookup (v2 + v3), sparse `--fields`, multi-barcode batch
- Text search with tag filters (category/brand/label/country/store/additive/allergen) + nutrient-range filters + sort + pagination
- Per-100g + per-serving nutriment breakdown (kcal, protein, fat, sat-fat, carbs, sugar, fiber, salt, sodium)
- Nutri-Score, NOVA group, Eco/Green-Score, additives (E-numbers), allergens/traces, vegan/vegetarian/palm-oil flags
- Suggest/autocomplete (taxonomy_suggestions), taxonomy fetch
- Knowledge panels, attribute groups, facets browsing
- Barcode normalize (pad 8/13) + GTIN check-digit validation
- Flavor switch (off/obf/opff/opf), country/locale (cc/lc), env switch (prod/staging), API version (v2/v3)
- Image URLs (front/ingredients/nutrition/packaging), OCR text
- JSON / table / CSV output

## Data Layer
- **Primary entities:** product (PK = barcode/code), nutriments (per-product), categories, brands, labels, ingredients, allergens, additives, scores (nutri/nova/eco).
- **Local-only entities (ours):** diary_entry (date, barcode, servings, computed macros), goal (daily kcal/macro targets), favorites/pantry.
- **Sync cursor:** `last_modified_t` on products; sync by category/tag or by explicit barcode list.
- **FTS/search:** FTS5 over product_name, brands, categories, ingredients_text -> offline search that bypasses rate limit.

## Source Priority
- Single source (Open Food Facts). No combo ordering.

## Product Thesis
- **Name:** `openfoodfacts-pp-cli` (binary), display "Open Food Facts".
- **Why it should exist:** No general-purpose Go CLI for OFF exists. Existing tools are barcode-only toys, ML pipelines (labelr/Robotoff), or MCP servers. None ship: offline search, a local calorie/macro diary, product compare, healthier-alternative lookup, AND built-in rate-limit/User-Agent safety. This is FatSecret's free open-data twin you can pipe to `jq`.

## Build Priorities
1. **P0 data layer + sync + FTS search + SQL** — products/nutriments/scores tables; offline search that dodges rate limits.
2. **P1 absorb** — every read feature: lookup (v2/v3, fields, batch), search (filters/nutrient-range/sort/page), suggest, taxonomy, knowledge panels, attribute groups, facets, scores extraction, additives/allergens/flags, barcode normalize/validate, image URLs, flavor/locale/env/version switches. Rate limiter + UA baked into the client.
3. **P2 transcend** — local diary (add/today/since/goal), compare, healthier-alternative swap, meal/recipe nutrition aggregation, allergen-profile alert (see novel-features subagent output).
4. **P3 polish** — flag descriptions, tests for diary math + barcode validation + score parsing.
