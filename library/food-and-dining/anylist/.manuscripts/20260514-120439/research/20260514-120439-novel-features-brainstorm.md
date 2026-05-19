# Novel Features Brainstorm — AnyList CLI
<!-- Full subagent output including Customer model, Candidates, Survivors, Killed candidates -->

## Customer model

### Persona 1 — Maya, the Household Meal Planner

**Today (without this CLI):** Maya plans the week's meals on Sunday using the AnyList app. She taps each recipe, manually reads the ingredient list, and adds items one by one to her grocery list — checking for duplicates by eye. She shops on Monday.

**Weekly ritual:** Every Sunday she opens the meal plan view, picks 5–7 recipes for the week, taps "Add to List" for each, then spends 10 minutes deduplicating ("2× chicken breast" appears three times). On Monday she works through the list in-store, checking items as she goes.

**Frustration:** She can't run a cron job or shell script to auto-build her Monday shopping list from Sunday's meal plan. If she forgets to add ingredients the night before, she discovers missing items mid-cook.

### Persona 2 — Raj, the Scripting Home Automator

**Today (without this CLI):** Raj runs Home Assistant and has the hacs-anylist integration to add/remove items via automations. But to do anything more complex — list the week's plan, query what he bought last week, cross-reference recipes with pantry — he's hacking JavaScript inside a Node.js wrapper or writing MQTT triggers.

**Weekly ritual:** He maintains a YAML automation that fires each Monday to uncheck everything in his "Weekly Staples" list. He manually exports shopping history by screen-recording. He pastes recipes into a Python script to scale servings.

**Frustration:** There is no CLI he can `| jq | grep` against. Every operation requires spinning up the full Node.js anylist npm server or navigating the GUI.

### Persona 3 — Iris, the Recipe Power User

**Today (without this CLI):** Iris has 300+ imported recipes in AnyList. She imports them via the app's "Import from URL" button. She organizes recipes into collections ("Weeknight Fast," "Batch Cook," "Guests") by hand. She frequently scales recipes for dinner parties.

**Weekly ritual:** She browses her recipe collections, picks 2–3 for the week, scales each to her target serving count, then adds to list. She searches recipes by ingredient ("what can I make with chicken?") by scrolling and reading.

**Frustration:** She can't search recipes by ingredient or nutrition in bulk. She can't run a script that says "give me every recipe with prep_time < 30 and rating >= 4."

### Persona 4 — Leo, the Grocery Optimizer

**Today (without this CLI):** Leo tracks grocery prices because he shops at multiple stores (Costco, Trader Joe's, local co-op). He's set up store filters in AnyList but can't query which items he's assigned to which store, and can't generate a "by-store" shopping route without manually sorting the list in the app.

**Weekly ritual:** He opens AnyList, manually filters by store, screenshots the list, and uses Notes.app as a clipboard for each store's sub-list.

**Frustration:** The app has store-routing data (store_ids, category sort order) but exposes none of it in a queryable way. He can't `jq` his list.

---

## Candidates (pre-cut)

| # | Candidate | Source | Kill/keep verdict |
|---|-----------|--------|------------------|
| C1 | Cross-recipe ingredient search | f, c | Keep — joins recipes + ingredients tables; no LLM dependency |
| C2 | Recipe filter by metadata (prep_time, rating, collection) | c, persona Iris | Keep — multi-field filter against local data; app only supports name search |
| C3 | Store-partitioned shopping output | c, persona Leo | Keep — joins store_ids → stores.sort_index; app never shows per-store grouping |
| C4 | Category-sorted shopping output | b, persona Leo | Reframe → subsumed by C3; kill standalone |
| C5 | Cron-safe meal-plan-to-list | a, persona Maya | Keep — idempotent + quantity-merge + --dry-run materially different from absorbed #29 |
| C6 | Shopping list diff / staleness report | c, persona Raj | Replaced by C12; kill |
| C7 | Price-tracking report | b, c | Kill — PBItemPrice sparsely populated; broken for most users |
| C8 | Recipe add-to-list with quantity merge | a, c | Keep — quantity merge not in any existing tool |
| C9 | CSV/TSV export (standalone) | a, persona Maya | Kill standalone — fold into --format flag |
| C10 | Meal plan calendar summary grid | a, persona Maya | Keep — no existing tool renders terminal calendar grid |
| C11 | Missing-ingredients check | c, persona Maya | Keep — set-difference join: recipe ingredients vs current list |
| C12 | Sync cache age / staleness check | c, persona Raj | Keep — exit-code contract enables cron |
| C13 | Recipe nutrition summary | b | Kill — nutritional_info empty for most recipes |
| C14 | Batch recipe import from file | a, persona Iris | Kill — mechanical loop; shell one-liner equivalent |
| C15 | Weekly shopping list reset | a, persona Raj | Keep — idempotent bulk operation with --keep-unchecked; cron-safe |
| C16 | Cross-list item search | c, persona Raj | Keep — SQLite FTS across all lists; app/MCP/npm all scope to single list |

---

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Cross-recipe ingredient search | `recipe search --ingredient "chicken"` | 10/10 | Joins `recipes` + `ingredients` tables in local SQLite; no API call required post-sync | Brief Gap: "No existing tool offers offline search"; bcspragu/anylist proto has PBIngredient.name; app has no ingredient-based search |
| 2 | Recipe filter by metadata | `recipe filter --max-prep 30 --min-rating 4 --collection "Weeknight"` | 9/10 | Queries local SQLite `recipes` table using PBRecipe.prep_time, rating, servings, collection join | Brief proto schema: PBRecipe has prep_time, cook_time, rating, servings; app exposes no multi-field filter |
| 3 | Store-partitioned shopping output | `list by-store [list-name]` | 10/10 | Joins `items.store_ids` → `stores.sort_index` in local SQLite, groups and orders output by store | Brief proto schema: ListItem.store_ids, PBStore.sort_index; Top Workflow #5: "Batch shopping across stores"; app never shows per-store grouped view |
| 4 | Cron-safe meal-plan-to-list | `meal add-to-list --week --list "Groceries" --dry-run` | 10/10 | Reads meal plan events for ISO week, merges ingredient quantities, writes idempotently via `/data/shopping-lists/update` | Brief Top Workflow #2; absorbed #29 lacks idempotency/quantity-merge; davidashman add_ingredients_to_list has no --dry-run |
| 5 | Recipe add-to-list with quantity merge | `recipe add-to-list "Pasta Bake" --scale 4 --merge` | 10/10 | Reads current list items from local cache, sums quantities arithmetically, writes merged delta via `/data/shopping-lists/update` | Brief gap: davidashman deduplicates but does not merge quantities; proto ListItem.quantity; absorbed #19 noted as dedup-only |
| 6 | Meal plan weekly grid | `meal summary [--week\|--month]` | 8/10 | Joins `meal_plan_events` + `calendar_labels` + `recipes.name` in local SQLite, renders Mon–Sun × label grid | Brief proto: PBCalendarEvent.date, PBCalendarLabel.name, PBRecipe.name; no existing tool renders terminal calendar grid; Top Workflow #2 |
| 7 | Missing-ingredients check | `recipe missing "Pasta Bake" --list "Groceries"` | 9/10 | Set-difference join between `ingredients` (by recipe_id) and `items` (by list_id, unchecked) in local SQLite | Brief proto: PBIngredient.name vs ListItem.name; Top Workflow #3; no existing tool does this |
| 8 | Sync cache age / staleness check | `sync status` | 7/10 | Reads `_sync_meta` table in local SQLite for per-entity last_sync timestamps; exits 1 if stale | Brief Data Layer: "Sync cursor: server_mod_time"; exit-code contract enables cron; no existing tool exposes cache health |
| 9 | Weekly list reset | `list reset [list-name] [--keep-unchecked]` | 9/10 | Issues `set-list-item-checked=false` for all checked items in one batched PbListOperationList | Brief Top Workflow #1: "check off → uncheck remaining for next trip"; absorbed #5 is single-item only; no tool has bulk operation |
| 10 | Cross-list item search | `item search "almond milk"` | 9/10 | SQLite FTS/LIKE query on `items.name` across all list_ids, returns list name + item state per match | Brief Data Layer: items table has list_id FK; app/MCP/npm all scope search to single list |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| C4 — Category-sorted shopping output | Subsumed by C3 (store-partitioned list); standalone adds no incremental value | `list by-store` (#3) |
| C6 — Shopping list diff / staleness report | Replaced by C12 (sync status) with sharper exit-code contract | `sync status` (#8) |
| C7 — Price-tracking report | PBItemPrice sparsely populated; most users see empty output, feature appears broken | None |
| C9 — CSV/TSV export (standalone) | Folded into `--format csv\|tsv` on existing output commands; not worth a dedicated command | `--json` flag (absorbed) |
| C13 — Recipe nutrition summary | AnyList does not compute nutrition; field empty for most imported recipes | None |
| C14 — Batch recipe import from file | Mechanical loop over absorbed import; user can shell-script `while read url; do anylist recipe import "$url"; done` | absorbed recipe import |
