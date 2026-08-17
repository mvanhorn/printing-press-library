# NutritionValue.org Browser-Sniff Discovery Report

**Run:** 20260702-105700
**Backend:** browser-use 0.12.5 (CLI mode, no LLM key)
**Primary sniff goal:** "Look up a food's nutrition facts and compare two foods."
**Reachability mode:** `standard_http` (probe-reachability confidence 0.95; stdlib and surf-chrome both 200). No browser challenge on data pages. No clearance cookie needed. Printed CLI ships standard HTTP + HTML extraction.

## Headline finding: no first-party API, fully replayable server-rendered HTML

Performance-API capture on the search page showed **34 resources, zero first-party JSON/XHR/GraphQL endpoints**. The only non-static requests are third-party Google ad/funding + reCAPTCHA (`fundingchoicesmessages.google.com`, `pagead2.googlesyndication.com`, `google.com/recaptcha/api2`). All nutrition data is delivered as server-rendered HTML via `.php` and `.html` page loads.

Implication: the NV surface is authored as an internal spec with `response_format: html` endpoints (aggregator pattern), not sniffed JSON. Browser-sniff **confirmed** the hand-mapped structure and surfaced one thing hand-mapping missed (stateless compare URL, below).

## Replayable endpoints (all GET, browser-UA, standard HTTP)

| Purpose | URL pattern | Notes |
|---|---|---|
| Search | `/search.php?food_query=<term>` | Returns HTML list of foods; each result links to a detail slug + carries a numeric id and `s`-prefixed id for branded items |
| Food detail | `/<URL-encoded food name>_nutritional_value.html` | e.g. `/Cheese%2C_cheddar_nutritional_value.html`, `/Bananas%2C_raw_nutritional_value.html`. 18 KB, 14 tables, full nutrition facts + daily-value %, serving-size `<select>` |
| Compare (stateless!) | `/comparefoods.php?foods=<id>*<qty>+<unit>,<id2>*<qty>+<unit>` | **Browser-sniff discovery**: adding via `?action=add&id=X&unit=Y` 302-redirects to a canonical `?foods=173944*100+g` URL. Multiple foods join with commas. Verified two-food compare (banana 173944 vs cheddar 173414) returns 200 + both foods in one table. No session/cart needed. |
| Nutrient ranking (highest) | `/foods_by_<Nutrient>_content.html` | e.g. `/foods_by_Protein_content.html` (42 rows), `/foods_by_Vitamin%20C_content.html` |
| Nutrient ranking (lowest) | `/foods_by_<Nutrient>_content_lowest.html` | Same, ascending |
| Meal calculator | `/nutritioncalculator.php?action=add&ids=<id>` | Session/cart-based; returns 200. Lower-value for a stateless CLI; the local store + USDA math covers meal aggregation better. |

## Food ID shapes
- Plain numeric id (e.g. `173414`, `173944`) = generic/SR foods (these map closely to USDA FDC records).
- `s`-prefixed id (e.g. `s77515`) = user-submitted / branded foods unique to NV.

## Data on a food detail page
Calories, protein, total fat (+ saturated/mono/poly/trans), carbohydrate, fiber, sugars, cholesterol, sodium, vitamins (A, C, D, B-complex, K), minerals (calcium, iron, potassium, magnesium, zinc, etc.), daily-value %, multiple serving-size options via `<select>`. No JSON-LD; data lives in clean HTML tables (extract via `response_format: html`).

## Recommended printed-CLI transport
- **NV:** standard `net/http` with a Chrome User-Agent + Referer, HTML table extraction. No browser, no cookie, no login for the core read surface.
- Extraction targets: search result rows (name + slug + id), detail-page nutrient tables, ranking-page rows.

## ToS note
The homepage HTML carries a comment prohibiting scripted access and threatening IP blacklisting. The data pages nonetheless serve plainly to a browser UA. User explicitly approved building against NV ("YOLO mode"). The printed CLI must stay polite: conservative default rate limiting, honest User-Agent, no aggressive crawling. Document the ToS caveat in the README.
