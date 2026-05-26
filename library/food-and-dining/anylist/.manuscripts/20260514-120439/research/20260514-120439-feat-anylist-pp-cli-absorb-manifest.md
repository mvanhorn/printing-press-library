# Absorb Manifest — AnyList CLI
<!-- Generated: 2026-05-14 -->

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List all shopping lists | bobby060/anylist-mcp, davidashman/anylist-mcp | `lists list` | `--json`, offline from SQLite cache |
| 2 | Show items in a list (filter: checked/unchecked/all) | All MCP tools, hacs-anylist | `items list <list>` | `--json`, `--checked`, `--unchecked`, `--category`, `--store` filters |
| 3 | Add item to list | All MCP tools, anylist npm, hacs-anylist | `items add <list> <item>` | `--quantity`, `--notes`, `--category`, `--store`, `--stdin` batch, `--json` |
| 4 | Check item off list | All MCP tools, hacs-anylist | `items check <list> <item>` | `--all` (check entire list), `--json` |
| 5 | Uncheck item | hacs-anylist, anylist npm | `items uncheck <list> <item>` | `--all`, `--json` |
| 6 | Remove item from list | hacs-anylist, anylist npm | `items remove <list> <item>` | `--all` (remove checked), `--json` |
| 7 | Update item (quantity, notes, category) | anylist npm | `items update <list> <item>` | Multiple fields in one command, `--json` |
| 8 | Get favorite items | bobby060/anylist-mcp | `favorites list` | `--json` |
| 9 | Get recently added items | bobby060/anylist-mcp | `items recent [list]` | `--json`, `--limit` |
| 10 | Create shopping list | anylist_rs | `lists create <name>` | `--json` |
| 11 | Delete shopping list | anylist_rs | `lists delete <name>` | `--confirm`, `--json` |
| 12 | List all recipes | All MCP tools | `recipes list` | `--json`, offline |
| 13 | Show recipe details (ingredients + steps) | bobby060, davidashman | `recipes show <name>` | `--json`, `--ingredients-only`, `--steps-only` |
| 14 | Search recipes by name | bobby060, anylist npm | `recipes search <query>` | SQLite FTS5 offline search, `--json` |
| 15 | Import recipe from URL | bobby060/anylist-mcp (import_url) | `recipes import <url>` | `--add-to-list <list>`, `--scale <n>`, `--json` |
| 16 | Parse/normalize recipe from text | bobby060/anylist-mcp (normalize) | `recipes normalize` | stdin support, `--json` |
| 17 | Create recipe manually | bobby060, anylist npm | `recipes create` | `--from-stdin` JSON, all fields, `--json` |
| 18 | Delete recipe | bobby060, anylist npm | `recipes delete <name>` | `--confirm`, `--json` |
| 19 | Add recipe ingredients to list (dedup) | bobby060, davidashman | `recipes add-to-list <recipe> <list>` | `--scale <n>`, `--dedup`, `--json` |
| 20 | Batch add ingredients from multiple recipes | davidashman (add_ingredients_to_list) | `recipes batch-add <list> <recipe>...` | `--dedup`, `--json` |
| 21 | List recipe collections | bobby060, anylist npm | `collections list` | `--json` |
| 22 | Create recipe collection | bobby060, anylist npm | `collections create <name>` | `--json` |
| 23 | Add recipe to collection | anylist npm | `collections add <collection> <recipe>` | `--json` |
| 24 | Remove recipe from collection | anylist npm | `collections remove <collection> <recipe>` | `--json` |
| 25 | Delete recipe collection | anylist npm | `collections delete <name>` | `--confirm`, `--json` |
| 26 | Show meal plan (date range) | bobby060, davidashman | `meal show [--from <date> --to <date>]` | Defaults to current week, `--json` |
| 27 | List meal plan labels | bobby060/anylist-mcp | `meal labels` | `--json` |
| 28 | Create meal plan event | bobby060, davidashman | `meal add <date>` | `--recipe`, `--title`, `--label`, `--details`, `--json` |
| 29 | Delete meal plan event | bobby060 | `meal delete <event-id>` | `--json` |
| 30 | List categories | anylist_rs | `categories list` | `--json` |
| 31 | List stores/filters | anylist_rs | `stores list` | `--json` |
| 32 | List favorite items | bobby060/anylist-mcp | `favorites list` | `--json` (covered by #8) |
| 33 | Auth login (email + password) | All tools (env vars approach) | `auth login` | `--save`, `--json`; credentials via env or prompt |
| 34 | Auth logout | All tools | `auth logout` | Clears stored tokens |
| 35 | Auth status | Implied | `auth status` | Shows token expiry, user info, `--json` |
| 36 | Token refresh | All tools | `auth refresh` | Manual + auto-refresh on 401 |
| 37 | Sync all data to local SQLite cache | Unique to this CLI | `sync` | `--force`, `--quiet`, `--json` |
| 38 | Raw SQL queries against local cache | Unique to this CLI | `sql <query>` | `--json`, `--csv`, `--tsv` |
| 39 | JSON output for all commands | Unique to this CLI | `--json` flag everywhere | Agent-native; typed exit codes |
| 40 | Recipe scale by serving count | Unique to this CLI | `recipes scale <name> --servings <n>` | Scales all ingredient quantities proportionally, `--json` |
| 41 | Starter list management | Browser-sniff discovered `/data/starter-lists/update` | `starters list` / `starters add` / `starters remove` | (stub — proto body shape unconfirmed for write operations) |
| 42 | List folder management | anylist_rs, API endpoint confirmed | `folders list/create/delete/move` | `--json` |
| 43 | List settings view/update | anylist_rs, API endpoint confirmed | `lists settings <name>` | `--json` |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Cross-recipe ingredient search | `recipe search --ingredient "chicken"` | 10/10 | Joins `recipes` + `ingredients` tables in local SQLite; no API call required post-sync | Brief: "No existing tool offers offline search"; PBIngredient.name in proto; app has no ingredient-based search |
| 2 | Recipe filter by metadata | `recipe filter --max-prep 30 --min-rating 4 --collection "Weeknight"` | 9/10 | Queries local `recipes` table using PBRecipe.prep_time, rating, servings + collection join | PBRecipe has prep_time, cook_time, rating, servings in proto; app exposes no multi-field filter |
| 3 | Store-partitioned shopping output | `list by-store [list-name]` | 10/10 | Joins `items.store_ids` → `stores.sort_index` in local SQLite, groups and orders output by store | ListItem.store_ids + PBStore.sort_index in proto; Top Workflow #5 "Batch shopping"; app never shows per-store grouping |
| 4 | Cron-safe meal-plan-to-list | `meal add-to-list --week --list "Groceries" [--dry-run]` | 10/10 | Reads meal plan events for ISO week, merges ingredient quantities, writes idempotently via `/data/shopping-lists/update` | Top Workflow #2; absorbed #20 lacks idempotency/quantity-merge; davidashman has no --dry-run |
| 5 | Recipe add-to-list with quantity merge | `recipe add-to-list <recipe> <list> --scale 4 --merge` | 10/10 | Reads current list items from local cache, sums quantities arithmetically where ingredient names match, writes delta | davidashman deduplicates but does not merge quantities; proto ListItem.quantity; gap vs absorbed #19 |
| 6 | Meal plan weekly grid | `meal summary [--week\|--month]` | 8/10 | Joins `meal_plan_events` + `calendar_labels` + `recipes.name` in local SQLite, renders Mon–Sun × label grid | PBCalendarEvent.date + PBCalendarLabel.name + PBRecipe.name in proto; no existing tool renders terminal calendar grid |
| 7 | Missing-ingredients check | `recipe missing <recipe> --list <list>` | 9/10 | Set-difference join between `ingredients` (by recipe_id) and `items` (by list_id, unchecked) in local SQLite | PBIngredient.name vs ListItem.name in proto; Top Workflow #3; no existing tool computes this delta |
| 8 | Sync cache staleness check | `sync status` | 7/10 | Reads `_sync_meta` table for per-entity last_sync timestamps; exits 1 if any entity exceeds threshold | Sync cursor: server_mod_time in brief; exit-code contract enables cron; no existing tool exposes cache health |
| 9 | Weekly list reset | `list reset [list-name] [--keep-unchecked]` | 9/10 | Issues `set-list-item-checked=false` for all checked items in one batched PbListOperationList | Top Workflow #1: "uncheck remaining for next trip"; absorbed #5 is single-item only; no tool has bulk operation |
| 10 | Cross-list item search | `item search <query>` | 9/10 | SQLite FTS/LIKE query on `items.name` across all list_ids, returns list name + item state per match | items table has list_id FK; app/MCP/npm all scope search to single list at a time |
