# MyFitnessPal CLI Brief

## API Identity
- **Domain:** Calorie/food/exercise/weight tracking. Daily diary structure: meals (breakfast/lunch/dinner/snacks/custom) of food entries, plus cardio + strength logs, water, measurements, goals, recipes, saved meals, free-text notes.
- **Users:** People logging food daily (often multiple times per day), athletes tracking macros, dieters tracking calories, weight-loss/gain trackers. ~200M registered users; ~1M active CSV-export-curious power users.
- **Data profile:** Time-series per user, append-mostly, multi-meal-per-day. Free tier retains 2 years; premium ($80/yr) gates CSV export and rolls it up to meal level (per-food granularity is the #1 community complaint).

## Reachability Risk
- **HIGH.** Captcha added 2022-08-25 broke direct username/password login for every wrapper.
- Recent breakage: issue #196 (2025-06) `/user/auth_token` returning 403 after a site redesign; #198 (2025-07) saved-meals/measurements endpoints changed shape; #203 (2025-12) "MFP changed their site?"; #205 (2026-02) "API still works?" — open within the last three months.
- No Cloudflare/Turnstile-style WAF observed; protection is captcha + session-cookie validation. Real-browser cookies still work for read paths; headless login does not.
- Author of the Java wrapper now warns "may no longer work due to MFP's security improvements."
- **Mitigation:** the printed CLI must accept a cookie file path (Chrome/Firefox cookie-jar import), surface 403/captcha errors with an actionable message ("re-login in your browser, then re-run sync"), and treat HTML scrapers as best-effort. Prefer `/user/auth_token` → `https://api.myfitnesspal.com/v2/...` JSON wherever both surfaces exist.

## Top Workflows
1. **Daily/weekly diary export to CSV** — #1 unmet need. Free tier has no CSV; premium CSV rolls up to meal level. Per-food granularity is the headline community complaint (community.myfitnesspal.com discussion 10957736).
2. **Sync diary to SQLite for local analysis** — `seeM/myfitnesspal-to-sqlite` (7★, dormant) and `bhutley/myfitnesspal` (2013, dead) prove the demand. No active maintained tool today.
3. **Trend reports over arbitrary date ranges** — macros over time, weight trend, weeks-vs-weeks. The `/api/services/reports/results/{category}/{report_name}/{days}.json` endpoint already returns date→value series; nobody surfaces it well.
4. **Push to Google Sheets / dashboards** — `hbmartin/myfitnesspal-to-google-sheets` (14★, inactive); ImportXML/Looker hacks all over Reddit.
5. **Cross-app sync narratives** — Apple Health, Garmin, Fitbit, Oura, Withings. MFP has native integrations for some; users want a CLI for batch backfill and audit.
6. **AI nutrition coach** — Medium posts ("How I Built a Nutrition Coach with Claude Code") describe gluing MFP data into Claude/ChatGPT manually. **No published Claude Code skill exists today.** Greenfield.

## Table Stakes
Match what `python-myfitnesspal` (861★) and `AdamWalt/myfitnesspal-mcp-python` (11★) already do:
- Get day's diary (foods, exercises, water, notes, totals, goals, completion)
- Search foods, get food details (full nutrient panel)
- Add food/exercise/water/measurement
- Get measurements time-series (weight, body fat %, custom)
- Set measurements / set goals / set water
- Get goals (calorie target, macro split)
- Get reports (any nutrient or weight as a date→value series)
- Get/list recipes; get/list saved meals
- Per-meal exercise log (cardio: name, duration, calories; strength: name, sets, reps, weight)
- Free-text food notes / exercise notes per day

## Data Layer
- **Primary entities (in gravity order):**
  1. `diary_entry` (date, meal, food_id, servings, serving_size, full nutrient panel snapshotted)
  2. `food` (`mfp_id`, brand, description, full nutrient panel, serving variants, public/custom flag)
  3. `exercise_entry` (date, type=cardio|strength, name, duration, calories, sets, reps, weight, distance)
  4. `measurement` (date, type=Weight|BodyFat|Neck|Waist|Hips|custom, value, unit)
  5. `goal_snapshot` (date, calorie_target, protein_g, carbs_g, fat_g, water_target, weight_target)
  6. `water_entry` (date, amount, unit)
  7. `note` (date, kind=food|exercise, text)
  8. `recipe` (id, title, ingredient_list_json, per_serving_nutrients)
  9. `saved_meal` (id, title, member_entries_json)
  10. `report_snapshot` (category, report_name, date, value) — derivable, also worth caching
  11. `user_profile` (id, username, units, gender, age, height, locale, premium_flag)
- **Sync cursor:** primary cursor is `(diary_date)`. Sync ingests one day at a time over a window (default last 30d, configurable). Foods/exercises/measurements pulled by date range. Recipes/saved-meals are full pulls (small tables).
- **FTS/search:** `foods_fts(description, brand)` for food lookup without re-hitting `/food/search`; `diary_entries_fts(food_name)` for "find every time I logged X"; `recipes_fts(title)`; `notes_fts(text)`.

## Codebase Intelligence
- Source: github.com/coddingtonbear/python-myfitnesspal (861★, primary), AdamWalt/myfitnesspal-mcp-python (11★, MCP-shaped), seonixx/myfitnesspal (Go, claims working).
- **Auth:** Cookie-session via `browser_cookie3` (lifts cookies from a logged-in Chrome/Firefox profile). Boots a bearer token via `GET /user/auth_token`. Required headers on `api.myfitnesspal.com/v2/...`: `Authorization: Bearer <token>`, `mfp-client-id`, `mfp-user-id`, `Accept: application/json`. NextAuth-shaped session cookies (`__Secure-next-auth.session-token`, `__Host-next-auth.csrf-token`).
- **Two surfaces, used together:**
  - `https://www.myfitnesspal.com` — primary read path. HTML scraping + a small JSON-reports island under `/api/services/reports/...`. Endpoints: `/food/diary/{username}?date=`, `/exercise/diary/{username}?date=`, `/measurements/edit?type&from&to`, `/food/water?date=`, `/food/note?date=`, `POST /food/search`, `POST /food/submit`, `/food/duplicate`, `/food/new`, `/reports/printable_diary?from&to`, `/api/services/reports/results/{category}/{report_name}/{days}.json`, `GET /user/auth_token`.
  - `https://api.myfitnesspal.com/v2` — bearer-protected JSON. Endpoints in the wild: `GET /v2/users/{user_id}`, `GET /v2/foods/{mfp_id}`. Documented partner-only (closed): `GET/POST/PATCH/DELETE /v2/diary`, `GET/POST /v2/measurements/`, `GET /v2/exercises/`, `POST/GET/DELETE /v2/subscription/`.
- **Data model insight:** the `Day` object is the natural batching unit; `meals` is a dict keyed by meal name; `entries` are flat. `Exercise` is a thin wrapper; cardio vs strength distinguished by which `Entry` fields are present. `Measurement` is a date→float dict per measurement type.
- **Rate limiting:** No documented hard limit; intermittent 403/429 under bursty access. Wrappers serialize requests. CLI should default to ~1 req/sec on sync.
- **Architecture insight:** Reports endpoint is a hidden gem — returns clean JSON time-series without scraping. CLI should prefer it over scraping `/food/diary/.../totals` for trend analysis.

## Source Priority
- Single source (myfitnesspal). No combo CLI, no priority gate to run.

## Product Thesis
- **Name:** `myfitnesspal-pp-cli`
- **Why it should exist:**
  1. **No active CLI today.** `savaki/myfitnesspal` (19★, last touched 2015) is the closest, with one command. Every other CLI-shaped tool is dead or single-purpose.
  2. **Per-food CSV export is the #1 unmet community pain.** Free tier has no export at all; premium CSV is per-meal, not per-food. A CLI that ships per-entry rows beats both tiers.
  3. **Local SQLite + offline FTS is novel for this API.** No competitor caches the diary locally; every wrapper re-hits the site for every query.
  4. **Agent-native angle is greenfield.** The AdamWalt MCP server (11★) hints at the demand, but there's no Claude Code skill, no first-class CLI/MCP combo, no offline-capable agent surface.
  5. **The CLI naturally encapsulates the auth fragility.** A user logs in once in Chrome, the CLI imports cookies, and every future agent invocation reuses the same local session. No more "rebuild my auth headers in every tool" boilerplate.

## Build Priorities
1. **Foundation (Priority 0):** Cookie-jar import auth (Chrome/Firefox/explicit path), bearer-token bootstrap via `/user/auth_token`, local SQLite store, sync command pulling diary/exercise/measurements/goals/water for a date window, `doctor` checking session validity.
2. **Absorb (Priority 1):** Every method `python-myfitnesspal` exposes + every tool `AdamWalt/myfitnesspal-mcp-python` exposes — get-diary, search-food, food-details, get-measurements, set-measurement, get-exercises, get-goals, set-goals, get-water, set-water, get-report, get-recipes, get-recipe, get-meals, get-meal, set-new-food, add-food-to-diary. Plus the JSON reports surface as `report` commands.
3. **Transcend (Priority 2):** Per-food CSV export (the table-stakes feature MFP itself blocks); offline FTS over diary + foods; macro-trend / weight-trend reports computed locally; "what did I miss" since-last-sync; week-vs-week diff; meal-pattern detection (e.g., "you eat 40g+ sugar by 10am on weekdays"); recipe nutrition recompute with `--servings N` scaling; saved-meal expansion to per-food rows; SQL pass-through for power users; agent context command exposing all of this.
