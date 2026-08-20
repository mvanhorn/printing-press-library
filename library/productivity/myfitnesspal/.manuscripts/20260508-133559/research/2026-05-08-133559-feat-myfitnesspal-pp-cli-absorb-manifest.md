# MyFitnessPal CLI — Absorb Manifest

This manifest captures every feature our CLI must ship, sourced from every existing tool that touches the MyFitnessPal API, plus the differentiating commands that only our local-SQLite + agent-native approach can deliver.

## Source Tools Mined

| Tool | URL | Lang | Stars | Status | Features Absorbed |
|---|---|---|---:|---|---:|
| coddingtonbear/python-myfitnesspal | github.com/coddingtonbear/python-myfitnesspal | Python | 861 | Active (slowing) | 13 |
| AdamWalt/myfitnesspal-mcp-python | github.com/AdamWalt/myfitnesspal-mcp-python | Python (MCP) | 11 | Active | 12 |
| seonixx/myfitnesspal | github.com/seonixx/myfitnesspal | Go | 1 | Active | 4 |
| marcosav/myfitnesspal-api | github.com/marcosav/myfitnesspal-api | Java | 23 | Author warns may not work | reference only |
| seeM/myfitnesspal-to-sqlite | github.com/seeM/myfitnesspal-to-sqlite | Python | 7 | Dormant | 1 (sync pattern) |
| hbmartin/myfitnesspal-to-google-sheets | github.com/hbmartin/myfitnesspal-to-google-sheets | Python | 14 | Inactive | reference only |
| Browser-sniff capture (HAR) | discovery/browser-sniff-capture.har | — | — | This run | 2 (top_foods, measurements/types) |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|---------------------|-------------|---|
| 1 | Get one day's full diary (foods, exercises, water, notes, totals, goals, completion) | python-myfitnesspal `get_date(date)`; AdamWalt MCP `mfp_get_diary` | `diary get-day --date YYYY-MM-DD` (HTML scrape via /food/diary, parsed into structured JSON) | --json output, --select fields, idempotent, offline replay against synced data | shipping |
| 2 | Search the public food database | python-myfitnesspal `get_food_search_results(query)`; AdamWalt MCP `mfp_search_food` | `food search --query "banana" [--meal N] [--date]` (POST /food/search HTML scrape) | --json output, agent-friendly, paginated | shipping |
| 3 | Get full nutrient panel for a food by id | python-myfitnesspal `get_food_item_details(mfp_id)`; AdamWalt MCP `mfp_get_food_details` | `food details --food-id <id> [--fields nutrients,serving_sizes]` (GET /v2/foods/{id}) | --json, --select | shipping |
| 4 | Get serving-size suggestions for a food | python-myfitnesspal extracted lazily; not in any MCP | `food suggested-servings --food-id <id>` (GET /v2/foods/{id}/suggested_servings) | First-class command for an endpoint nobody else exposes | shipping |
| 5 | Get the recent-foods quick-pick list for a meal | python-myfitnesspal HTML scraper; not in any MCP today | `diary load-recent --meal 0 --base-index 29` (POST /food/load_recent) | New surface; previously HTML-only | shipping |
| 6 | Get cardio + strength exercises logged on a day | python-myfitnesspal /exercise/diary scraper; AdamWalt MCP `mfp_get_exercises` | `exercise get-day --date YYYY-MM-DD` (HTML scrape) | Structured cardio vs strength, --json, --csv | shipping |
| 7 | Get water intake for a day | python-myfitnesspal `get_water(date)`; AdamWalt MCP `mfp_get_water` | `water get --date YYYY-MM-DD` (GET /food/water) | --json, --select | shipping |
| 8 | Get the day's free-text food note | python-myfitnesspal scraper; AdamWalt MCP no specific tool | `note get --date YYYY-MM-DD` (GET /food/note) | --json | shipping |
| 9 | Get a measurement type's date range (Weight, BodyFat, custom) | python-myfitnesspal `get_measurements(type, lower, upper)`; AdamWalt MCP `mfp_get_measurements` | `measurement get-range --type Weight --from <date> --to <date>` (HTML scrape from /measurements/edit) | --json, --csv, time-series shaped | shipping |
| 10 | List measurement types defined for the account | NEW — not in any wrapper today (browser-sniff only) | `measurement types` (GET /api/user-measurements/measurements/types) | First-class command for an endpoint nobody else has | shipping |
| 11 | Get current daily goals (calorie target, macro split, water target) | python-myfitnesspal `get_date().goals`; AdamWalt MCP `mfp_get_goals` | `goals get` (GET /account/my-goals HTML scrape) | --json structured | shipping |
| 12 | Get a time-series report (any nutrient or progress measurement) | python-myfitnesspal `get_report(name, category, lower, upper)`; AdamWalt MCP `mfp_get_report` | `reports get --category nutrition --report-name "Net Calories" --days 30` (GET /api/services/reports/results) | --json, --csv | shipping |
| 13 | Get user account info (units, goals prefs, paid subs) | python-myfitnesspal `user_metadata`; AdamWalt MCP no specific tool | `api-user get --user-id me [--fields ...]` (GET /v2/users/{id}) | --json, --select | shipping |
| 14 | Bootstrap a v2 bearer token from session cookies | python-myfitnesspal `access_token` property | `user auth-token` (GET /user/auth_token) | First-class for power users | shipping |
| 15 | Cookie-based auth + session validation | python-myfitnesspal `browser_cookie3`; AdamWalt MCP `refresh_browser_cookies` | `auth login --chrome [--profile Default]`, `auth status`, `auth logout`, `doctor` | Standard generator-emitted | shipping |
| 16 | Sync a date window to local SQLite | NEW — no wrapper does this; closest is seeM/myfitnesspal-to-sqlite which is dormant | `sync --from <date> --to <date>` (foundation, not transcendence) | The foundation that powers all transcendence below | shipping |
| 17 | Search across all synced data with FTS | NEW — generator-emitted | `search "banana"` (FTS5 across foods + diary entries + recipes + notes) | Standard framework feature | shipping |
| 18 | SQL pass-through over local SQLite | NEW — generator-emitted | `sql "SELECT meal, SUM(calories) FROM diary_entry WHERE date=DATE('now') GROUP BY meal"` | Standard framework feature | shipping |
| 19 | Health/diagnostics check | NEW — generator-emitted | `doctor` (validates session cookies, API reachability, store integrity) | Standard framework feature | shipping |
| 20 | Get top-logged foods over a date range | NEW — discovered via browser-sniff (not in any wrapper) | `analytics top-foods-server --from <date> --to <date>` (GET /api/services/top_foods server-side) | First-class command for an endpoint nobody else exposes; complements local-only `analytics top-foods` (#22) | shipping |

**Total absorbed: 20.** Closest competitor (AdamWalt MCP) ships 12 tools; python-myfitnesspal exposes ~13 client methods.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Persona Served |
|---|---------|---------|-------|--------------|---------------|
| 21 | Per-food CSV export | `export csv --from <date> --to <date> [--meal <name>] [--out diary.csv]` | 10/10 | Reads `diary_entry` rows from local SQLite (populated by `sync`) and writes one row per food entry with the snapshotted nutrient panel | Maya |
| 22 | Top foods by nutrient driver (local) | `analytics top-foods --nutrient protein --days 60 [--cumulative-percent 80]` | 8/10 | SQL group-by over `diary_entry` rows in local SQLite, ranked by total nutrient contribution and cut at the Pareto threshold | Maya |
| 23 | Weight-trend vs deficit | `analytics weight-trend --weeks 8 [--smooth 7d]` | 9/10 | Joins `measurement` (Weight), summed `diary_entry` calories per day, and `goal_snapshot` calorie targets in local SQLite; computes weekly slope and implied calories-per-lb | Priya |
| 24 | Macro-trend report | `analytics macro-trend --days 30 [--smooth 7d]` | 8/10 | Calls `/api/services/reports/results` for canonical per-nutrient series, fills any gaps from local `diary_entry` rows, emits JSON or CSV | Maya, Priya |
| 25 | Week-vs-week diff | `analytics weekly-diff [--weeks 2]` | 7/10 | Local SQL: two windowed aggregates over `diary_entry` + `measurement` + `goal_snapshot`; emits side-by-side cal/macro/weight deltas plus the top-3 foods that changed contribution most | Maya, Priya |
| 26 | Find food in diary (FTS) | `diary find --food "Chipotle Bowl" [--from <date>]` | 8/10 | Queries local `diary_entries_fts` + `foods_fts`; returns date/meal/servings/calories per match | Maya, Priya |
| 27 | Sync diff (since-last) | `sync diff [--since-last]` | 7/10 | Reads `diary_entry`/`measurement`/`water_entry`/`note` rows touched since the last sync timestamp in local SQLite; emits a structured changelog | Sam |
| 28 | Agent context dump | `context [--days 14] [--include diary,measurements,goals]` | 9/10 | Composes diary totals, weight trend, current goals, recent foods, and macro deltas from local SQLite into a single JSON payload sized for an agent context window | Devin |
| 29 | Reports backfill | `reports backfill --category nutrition --names "Net Calories,Protein,Fat,Carbs" --days 730` | 8/10 | Fans out over `/api/services/reports/results/{category}/{name}/{days}.json` with 1 req/sec pacing, persists each date→value series into `report_snapshot` | Priya, Sam |
| 30 | Macro-gap candidate foods | `analytics gap-candidates --remaining protein:60g,fat:<10g [--limit 20]` | 7/10 | Selects from `food` (favorites + last-30d recent) where servings satisfy the remaining-macro envelope, ranked by recency; pipes cleanly into `\| claude "pick one"` | Devin |
| 31 | Adherence streak | `analytics streak [--goal calories] [--tolerance 0.05]` | 6/10 | Counts the longest run of consecutive days where `diary_entry` daily totals fall within `tolerance` of `goal_snapshot.calorie_target` | Priya |
| 32 | Saved-meal expansion | `meal expand --meal-id <id> [--as-csv]` | 6/10 | Dereferences a saved meal's member entries via `GET /v2/foods/{id}` per member; emits per-food rows with nutrients instead of MFP's collapsed view | Maya |

**Total transcendence: 12.** All score ≥6/10. All are grounded in named personas from the customer model. All exploit the local SQLite store, a service-specific content pattern, or agent-shaped output — none are thin wrappers.

## Total CLI Surface

- **20 absorbed features** (matches and beats every competitor's full surface)
- **12 transcendence features** (impossible with any other tool in existence today)
- **32 total user-facing commands**, exposed as both CLI subcommands and MCP tools via the runtime cobratree mirror.

The closest competitor (AdamWalt MCP) ships 12 tools. The most-loved wrapper (python-myfitnesspal) exposes 13 methods. We ship 32, all working offline against a local SQLite mirror.
