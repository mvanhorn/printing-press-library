# Novel Features Brainstorm — nutrition-pp-cli (audit trail)

## Customer model

### Marcus, the cut-season powerlifter
Today: MyFitnessPal on phone + 3 NutritionValue.org tabs, eyeballing protein-per-calorie during a cut; does per-100kcal division in a spreadsheet because no tool exposes that basis.
Weekly ritual: Sunday meal prep — re-checks 5-10 staples, compares protein density, adjusts portions to hit 180g protein under 2,200 kcal; logs food during the week.
Frustration: No tool answers "most protein per 100 kcal" or "high-protein under 165 kcal/100g" in one step.

### Dana, the low-carb dieter
Today: Lives on NutritionValue.org for fields USDA tools don't show — net carbs, omega-6/omega-3 ratio, "lowest in carbohydrate" ranking pages, percentile bars. Bookmarks foods_by_Carbohydrate_content pages.
Weekly ritual: Grocery planning from ranking pages + net carbs on 10-15 candidate foods.
Frustration: The derived numbers she shops by exist only as server-rendered HTML; nothing scriptable exposes them and the USDA API never will.

### Priya, the agent builder
Today: Wiring nutrition into a Claude meal assistant; hard requirement is non-hallucinated numbers with provenance (every macro traces to a real fdcId). Hand-rolls USDA calls, normalizes 4 dataTypes, no clean recipe-total or citation.
Weekly ritual: Agent logs meals and answers "how am I doing vs targets today" — dozens of lookup+aggregate+compare calls/day needing --select-able JSON.
Frustration: Recipe/meal totals and daily-target progress need stateful aggregation the USDA API doesn't do.

## Survivors (7, all hand-code, all >=5/10)
See absorb manifest transcendence table.

## Killed candidates
| Feature | Kill reason | Closest sibling |
|---|---|---|
| Cross-source raw macro compare | NV raw numbers are mirrored USDA data → zero-diff by construction | enrich |
| Standalone percentile command | one field of enrich payload | enrich |
| Omega-ratio ranking command | = rank omega-ratio + enrich | rank |
| Swap suggester | unverifiable in dogfood, needs dense store, speculative demand | find |
| NV community foods + public recipes browse | most fragile HTML, no weekly persona ritual, crawl budget better spent | rank |
| Standalone %DV report | standard %DV is table stakes in food get; amino %DV ships in enrich | enrich |
| Nutrient-gap food suggester | deficit report already in log progress; suggestion half is unverifiable recommender | log |
