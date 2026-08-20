# AnyList CLI Research Brief

## API Identity
- **Domain**: Grocery/shopping list management, recipe organization, meal planning
- **Users**: Households managing grocery shopping, meal planners, home cooks, productivity users
- **Data profile**: Lists (shared, with items), items (name, quantity, notes, category, store, checked state), recipes (ingredients, steps, times, rating, photos), meal plan events (calendar-based), recipe collections, categories, stores/filters, favorites
- **Premium tier**: AnyList Complete ($9.99/year individual, $14.99/year household) — unlimited recipe imports, premium themes, location reminders, passcode lock

## API Technical Profile
- **Base URL**: `https://www.anylist.com`
- **Spec type**: Unofficial/undocumented, reverse-engineered
- **Transport**: Protobuf binary serialized as multipart form data
- **Auth headers**: `Authorization: Bearer {access_token}`, `X-AnyLeaf-API-Version: 3`, `X-AnyLeaf-Client-Identifier: {uuid}`
- **Token storage**: access_token + refresh_token + user_id (JSON from auth, protobuf for data)
- **Proto schema**: Available in github.com/bcspragu/anylist/pb/api.proto (full Go proto generated code)

## Reachability Risk
- **LOW** — 9 open issues on kevdliu/anylist, none about 403/auth failures
- npm package `anylist@0.8.6` published 9 days ago (active maintenance)
- December 2025 API changes: meal planning feature additions only, no breaking auth changes
- Multiple active implementations (JS, Rust, Python, Go, Home Assistant) all functioning

## Key Endpoints (from anylist_rs source analysis)
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/auth/token` | POST | Login (email+password → access_token, refresh_token, user_id) |
| `/auth/token/refresh` | POST | Refresh tokens |
| `/data/user-data/get` | POST | Fetch all user data (lists, recipes, categories, stores, meal plans) |
| `/data/shopping-lists/update` | POST | CRUD for list items (protobuf PbListOperationList) |
| `/data/shopping-lists/update-v2` | POST | Store/filter CRUD |
| `/data/user-recipe-data/update` | POST | Recipe CRUD (protobuf PbRecipeOperationList) |
| `/data/meal-planning-calendar/update` | POST | Meal plan event CRUD |
| `/data/list-folders/update` | POST | List folder management |
| `/data/list-settings/update` | POST | Per-list settings |
| `/data/starter-lists/update` | POST | Starter list items |
| `/data/photos/upload` | POST | Photo upload |

## Top Workflows
1. **Grocery trip**: Open list → check off items in store → uncheck remaining for next trip
2. **Meal prep**: Plan week on calendar → add recipe ingredients to shopping list → shop → cook
3. **Recipe discovery**: Import recipe from URL → add to collection → scale servings → add to list
4. **List sharing**: Share shopping list with partner/household, real-time sync
5. **Batch shopping**: Merge ingredients from multiple planned recipes into one shopping list

## Table Stakes (from competing tools)
### MCP Servers (bobby060/anylist-mcp, davidashman/anylist-mcp):
- Get/browse all lists and items
- Add items with name, quantity, notes
- Check/uncheck items
- Search/get recipes
- Get/add meal plan events
- Add recipe ingredients to list (with dedup)
- Batch: add ingredients from multiple recipes

### Home Assistant (kevdliu/hacs-anylist):
- add_item, remove_item, check_item, uncheck_item, get_items, get_all_items

### pyanylist (Python):
- Full CRUD for lists and items
- Category support

### MMM-AnyList (Magic Mirror):
- Display unchecked items from a list

**Gap vs competitors**: No existing tool offers offline search, SQLite persistence, CSV/JSON output, pipeline composition, or recipe scaling by target servings. No CLI exists at all.

## Data Layer
- **Primary entities**: List, ListItem, Recipe, MealPlanEvent, RecipeCollection, Category, Store, FavoriteItem
- **Sync cursor**: server_mod_time on items and lists
- **FTS/search**: Items by name, recipes by name/ingredient, meal plan by date range
- **SQLite tables**: lists, items, recipes, ingredients, recipe_collections, meal_plan_events, categories, stores

## Proto Schema (from bcspragu/anylist/pb/api.proto)
Key types:
- `ShoppingList`: id, name, items[], shared_users[], note, is_checked_list
- `ListItem`: identifier, list_id, name, details, checked, quantity, category, recipe_id, store_ids, prices, product_upc, photo_ids, manual_sort_index
- `PBRecipe`: id, name, ingredients[], preparation_steps[], note, source_name, source_url, prep_time, cook_time, servings, rating, nutritional_info, photo_id
- `PBIngredient`: name, quantity, note, raw_ingredient
- `PBCalendarEvent`: id, date(YYYY-MM-DD), title, recipe_id, label_id, details
- `PBCalendarLabel`: id, name (Breakfast/Lunch/Dinner/etc)
- `PBStore`: id, name, sort_index
- `PBStoreFilter`: id, name, store_ids[]
- `PBItemPrice`: price, size, base_price

## Codebase Intelligence
- Source: github.com/bcspragu/anylist (Go), github.com/phildenhoff/anylist_rs (Rust), github.com/kevdliu/anylist (JS/npm)
- Auth: Bearer token, X-AnyLeaf-API-Version header (version 3), X-AnyLeaf-Client-Identifier (UUID)
- Protobuf operations use handler IDs: "add-shopping-list-item", "set-list-item-checked", "update-list-item"
- All write operations use PbListOperationList / PbRecipeOperationList / PbCalendarOperationList wrappers
- Real-time sync via WebSocket (firebase-based) for live updates
- Import from proto: github.com/bcspragu/anylist/pb is the canonical Go proto package

## Product Thesis
- **Name**: `anylist-pp-cli`
- **Why it should exist**: AnyList has no official CLI. The only automation paths require running a Node.js server (anylist npm) or an MCP server. A Go CLI would provide fast shell scripting, cron automation, pipeline composition (jq, grep), offline search via local SQLite cache, JSON output for scripting, and direct recipe management without a GUI. It's the missing piece for power users who live in the terminal.
- **Differentiator**: SQLite sync + offline search + batch operations + pipeline composition — none of the existing tools offer this

## Build Priorities
1. **Auth**: login (email+password → token), logout, status, token refresh
2. **Lists**: list, show, items (with filtering: unchecked, checked, all, category, store)
3. **Items**: add, check, uncheck, remove, update (quantity, notes, category)
4. **Sync**: full sync to local SQLite, incremental updates
5. **Recipes**: list, show, search, import (URL), add-to-list (with scaling), create, delete
6. **Meal plan**: show (date range), add event, delete event, add-to-list (bulk ingredient add)
7. **Categories & stores**: list, organize items
8. **SQL**: raw SQL queries against local SQLite
9. **Favorites**: list, manage
10. **Recipe collections**: list, create, add-recipe
