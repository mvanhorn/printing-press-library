# CookUnity Novel-Features Brainstorm (subagent audit trail)

## Customer model
- **Marcus (PRIMARY / vision holder)** — macro-tracking meal-prepper. 180g protein / 2300 kcal target. Today: opens web app every Sunday, clicks meal cards one at a time, copies calories/protein into a spreadsheet by hand; no "show me everything over 40g protein under 600 kcal" filter. Frustration: menu only visible logged-in/online, paginated across lazy clusters, no arithmetic to pick a macro-fitting set.
- **Priya** — constraint-driven household cook (gluten-free, tree-nut-allergic). Reads allergen lists dish by dish. Frustration: shallow filters that reset on reload, allergen exclusion not composable with macro/cuisine filters, no "how much of the menu is even safe for us" view.
- **Dana** — data-curious optimizer/automator. Wants what's new/removed this week, whether favorites returned, best protein-per-dollar, machine-readable output. Frustration: everything trapped in SDUI JSON behind a 24h token; no diff, no history, no leaderboard, no --json to pipe.

## Survivors (transcendence)
1. plan (10/10, hand-code) — macro/calorie/budget/diet-constrained meal-set selector over local SQLite. Serves the User Vision directly.
2. drift (9/10, hand-code) — week-over-week menu diff (added/removed/price-changed) across two synced snapshots.
3. value (8/10, hand-code) — best-value leaderboard (protein-per-dollar / per-calorie) computed from macros × price.
4. chefs (7/10, spec-emits via analytics) — chef roster analytics (dish count, avg rating, avg price, cuisines).
5. favorites (8/10, hand-code) — which favorited meals are on the current menu (local query on isFavorite; no GraphQL needed).
6. compare (6/10, hand-code) — side-by-side macro/price/chef/allergen comparison of 2+ meals by id.

## Killed
- nutrition (redundant with sql/analytics), restock (--watch scope creep), recommend (thin GraphQL wrapper), allergen-safe (a flag on meals list, not a command), coverage (redundant group-by), history (thin GraphQL wrapper; value captured by favorites).
