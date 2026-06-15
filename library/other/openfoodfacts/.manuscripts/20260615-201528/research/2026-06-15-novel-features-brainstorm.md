# Open Food Facts CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Aisle Annie — label-checker at the supermarket.** Today: Yuka/FatSecret app, ad-laden taps, result trapped in app. Weekly: scans 5-15 barcodes mid-shop for buy/skip on kcal/Nutri-Score/NOVA/additives/allergens. Frustration: many taps per verdict, no allergen alarm tuned to her allergens, can't pipe data.

**Macro Marco — macro-tracking home cook.** Today: MyFitnessPal (account+paywall) or spreadsheet. Weekly: logs all food daily, watches running kcal/protein/fat/carbs vs goal. Frustration: diary paywalled+account-locked, no scriptable offline free running-total.

**Comparison Carla — data-minded dieter.** Today: two browser tabs eyeballing sugar/Nutri-Score/NOVA. Weekly: store-brand vs name-brand decisions, hunts healthier same-category items. Frustration: no side-by-side tool, nothing answers "healthier alternative in this category".

**Pipeline Pete — agent/automation author.** Today: raw OFF API for LLM meal-planner, gets throttled, hand-parses verbose payloads. Weekly: batch nutrition pulls + recipe aggregation into JSON. Frustration: rate-limit bans, no clean macro-totaled JSON, no recipe-level aggregation.

## Candidates (pre-cut)

14 candidates generated. Survivors + kills below. (Full candidate reasoning preserved from subagent run.)

Candidates: local macro diary; daily goal tracking; week/range diary report; side-by-side compare; healthier-alternative swap; recipe/meal aggregation; allergen-profile alert; scan-and-judge verdict; pantry/favorites; offline nutriment ranking; additive explainer; nutrient-budget search; diary export; barcode-batch macro report.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | How It Works |
|---|---------|---------|-------|---------|--------------|
| 1 | Local macro diary + goal | `diary add <barcode> --servings N` / `diary today` / `diary goal` | 9/10 | Macro Marco | Inserts into local `diary_entry`, joins `goal`, sums per-serving nutriments. No API reimplementation. |
| 2 | Range diary report | `diary since <date>` | 7/10 | Macro Marco | Aggregates diary rows by date in SQLite, per-day + avg macros. |
| 3 | Side-by-side compare | `compare <code1> <code2> [...]` | 8/10 | Comparison Carla | N product lookups, client-side per-100g + Nutri-Score/NOVA/Eco/sugar table. |
| 4 | Healthier-alternative swap | `swap <barcode> [--max-nova 3]` | 8/10 | Comparison Carla | Reads category, search w/ category tag + Nutri-Score sort, ranks better-scoring items. |
| 5 | Recipe/meal aggregation | `recipe <code...> --servings ...` | 7/10 | Pete + Marco | Batch lookups, sums per-serving nutriments into recipe + per-serving block, agent JSON. |
| 6 | Allergen-profile alert | `allergens set <list>` + `lookup --check-allergens` | 8/10 | Aisle Annie | Set-intersects product allergens+traces vs stored profile, flags HIT, non-zero exit. |
| 7 | Diet-budget search | `search --remaining-from-diary` | 6/10 | Marco + Pete | Reads goal minus logged macros, injects remaining headroom as nutrient-range filters. |
| 8 | Offline nutriment ranking | `rank --category <c> --sort <nutrient>` | 6/10 | Carla + Pete | Pure local SQLite query over synced store, ordered by any nutriment. Offline, dodges rate limit. |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `judge` compact verdict | Thin wrapper over absorbed `lookup` once allergen logic moves to #6 | #6 allergen alert |
| `pantry` favorites | Overlaps sync/favorites + diary product set | #1 diary |
| `additives` explainer | Already absorbed; taxonomy enrichment thin, drifts to static reimpl | #3 compare |
| `diary export` | A `--format json` flag, not a feature | #2 range report |
| `report` batch macro | Duplicates absorbed batch + #5 recipe aggregation | #5 recipe |
| Daily goal as separate feature | Storage half of #1, double-counting | #1 diary+goal |
