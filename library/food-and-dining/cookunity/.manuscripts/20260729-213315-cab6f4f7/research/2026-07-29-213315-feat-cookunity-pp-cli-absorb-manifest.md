# CookUnity CLI Absorb Manifest

There is **no competing CookUnity CLI, MCP server, or community wrapper** — the
only "prior art" is the CookUnity web app itself and adjacent meal/grocery/
nutrition CLI patterns (offline SQLite store, FTS search, macro filtering,
agent-native output). So "absorb" here means matching every capability the web
app has AND making it offline, composable, and agent-native — then transcending
with local computation the web app cannot do.

## Absorbed (match or beat the web app + adjacent-CLI patterns)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse the full weekly menu | CookUnity web app | cookunity-pp-cli meals list | Offline, one query not lazy-loaded cards, `--json`/`--select`/`--csv` |
| 2 | Meal detail (macros, chef, ingredients, allergens, heating) | CookUnity web meal page | cookunity-pp-cli meals get | Offline, full `MEAL.properties`, agent-native |
| 3 | Filter by diet / cuisine / protein / calories / allergen | CookUnity web filters | (behavior in cookunity-pp-cli meals list) `--diet --cuisine --min-protein --max-calories --exclude-allergen --max-price --chef` | Composable, persistent, SQL-backed, offline |
| 4 | Full-text meal search | CookUnity web search bar | cookunity-pp-cli search | FTS5 offline over name+description+chef+diet tags+cuisine |
| 5 | Sync the full menu into a local store | (nothing does this) | cookunity-pp-cli sync | **Priority 0.** Custom SDUI cluster-walk: clustered-results → 17 clusters → extract `MEAL.properties` → dedup by id → SQLite. Snapshots per delivery week. |
| 6 | Arbitrary SQL over the menu | (nothing does this) | cookunity-pp-cli sql | Offline analytics no web UI offers |
| 7 | Sort by price / calories / protein / rating | CookUnity web sort | (behavior in cookunity-pp-cli meals list) `--sort` | Offline, agent-native |
| 8 | Menu analytics / group-by | (nothing does this) | cookunity-pp-cli sql | Read-only SQL over the local meal store for arbitrary group-by/analytics (chef analytics also via `chefs`) |

## Transcendence (only possible with our local-first approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Macro-constrained meal planner | plan --protein-min 40 --calories-max 600 --count 8 --budget 120 --diet gluten-free | hand-code | Greedy/knapsack selection over local `meals` rows using nutrition+price+diet; the web app has no arithmetic and no way to pick a macro-fitting set | Use this to auto-build a set of meals hitting nutrition/budget targets. Do NOT use it to rank single meals by value ('value') or compare specific known meals ('compare'). |
| 2 | Week-over-week menu drift | drift --from <date> --to <date> | hand-code | Diffs two weekly SQLite snapshots by meal id (added/removed/price-changed); the web app keeps no history | none |
| 3 | Best-value leaderboard | value --metric protein-per-dollar --limit 20 | hand-code | Computes nutrient-per-price ratios from macros × per-box price; a ratio the site never shows | none |
| 4 | Chef roster analytics | chefs | spec-emits | Local aggregation of a first-class entity (dish count, avg rating, avg price, cuisines per chef) | none |
| 5 | Favorites on this week's menu | favorites | hand-code | Local query on the synced `isFavorite` flag to show which liked meals are available now | Use to see which favorited meals are on the current menu. Do NOT use it for browsing (meals list). |
| 6 | Meal comparison | compare <meal-id> <meal-id> [...] | hand-code | Local join over selected meal rows into a macro/price/chef/allergen table | Use to compare specific meals by id. Do NOT use it to discover meals ('search'/'meals list') or auto-build a week ('plan'). |

## Optional / secondary (flag at gate)

| Feature | Command | Buildability | Note |
|---------|---------|--------------|------|
| Upcoming order & delivery view | orders | hand-code (GraphQL) | Read-only view of the upcoming delivery (date, status, address) via `/subscription-back/graphql/user`. **Risk:** exact GraphQL query strings were observed by response shape but not captured verbatim; reconstruction carries moderate risk. Not required for the user's offline-meal-planning vision. Recommend deferring to a follow-up unless the user wants it in v1. |

## Auth & runtime notes
- Auth: `Authorization: <raw Auth0 JWT>` (NO "Bearer " prefix) + `platform: web` header. Token from the browser (Auth0 localStorage), ~24h expiry. CLI reads `COOKUNITY_TOKEN` env/config. `auth login --chrome` cannot capture it (localStorage, not a cookie), so token is pasted. Auth0 refresh-token auto-renew is a possible future feature.
- Runtime: standard HTTP (probe mode=standard_http). No clearance cookie, no resident browser.
- The entire meal surface is SDUI-wrapped, so the `sync` client + meal flattening is **hand-written Priority-0 code**, not generator-emitted endpoint commands.
