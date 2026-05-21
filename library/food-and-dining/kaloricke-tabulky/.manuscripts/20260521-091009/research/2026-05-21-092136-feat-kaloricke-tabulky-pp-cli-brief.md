# Kalorické Tabulky CLI Brief

## API Identity
- **Domain:** Czech (and SK/PL/HU/RU/UA localized) nutrition + diet tracking. Site name: "Kalorické tabulky" (calorie tables). Owned by IREsoft s.r.o.
- **Users:** Czech-speaking dieters, athletes, fitness coaches, nutritionists. 7.1M registered users, ~244k foodstuffs, ~556k diary entries (live counters off `/home/{x}/count`).
- **Data profile:** Foods (Czech-language, brand + generic, with full macro/micronutrient panel), recipes, exercise activities (kcal/min), per-user diary by meal time, weight log, favorites, custom user foods, water tracking, achievements/streaks.
- **Auth model:** Cookie session via `POST /login/create?format=json` with MD5-hashed password client-side. Session = JSESSIONID + `<session-cookie-name>`. No public API key, no OAuth.

## Reachability Risk
- **Low.** Direct HTTPS works without WAF challenges. Login + public read both confirmed via plain `urllib`. Cookie session persists. Three notes: (1) site is AngularJS over Spring MVC backend — endpoints expect `?format=json` query param; (2) endpoints not designed as a public API, so error responses are HTML-or-JSON mixed (some 404s come back as Tomcat error pages, others as JSON `USER_NOT_FOUND` envelopes); (3) password is MD5-hashed client-side before transmission.

## Top Workflows
1. **Look up nutrition for a Czech food** — search by Czech name (e.g. "jablko", "tvaroh"), get full macro/micronutrient breakdown per 100 g or per typical serving.
2. **Track daily diary** — view today's eaten food across meal times (Snídaně/Oběd/Svačina/Večeře), summary of energy/protein/carb/fat vs target.
3. **Log weight + see trend** — record weight for today, view recent weight history with delta.
4. **Search activities + estimate burn** — find an exercise (e.g. "běh", "joga"), multiply kcal/min by duration.
5. **Bulk-export / mine the diary** — agent task: pull a date range of diary entries to compute trends, find foods over-represented, compare against macro goals, plan meal substitutions.

## Table Stakes
- Czech-language food search with diacritics tolerance (`/autocomplete/foodstuff?query=...`)
- Per-day diary read (`/user/diary/<DD.MM.YYYY>/get`)
- Daily summary with macro/micronutrient targets (`/statistic/summary/<DD.MM.YYYY>/get`)
- Weight logging (`POST /user/weight/add`)
- Weight history (via summary `data.monthWeight[]`)
- Recipe search + lookup (`/autocomplete/meal`)
- Activity search + kcal-per-minute (`/autocomplete/activity`)
- Favorites list (`/user/settings/favorite/foodstuff?format=json`)

## Data Layer
- **Primary entities (sync targets):** `foodstuff`, `activity`, `meal` (recipe), `diary_entry`, `weight_record`, `favorite`, `user_setting`.
- **Sync cursor:** date-driven for `diary_entry` (DD.MM.YYYY one-day-at-a-time pull); foodstuff/activity/meal are population-driven (autocomplete + JSON-LD scrape — large catalog so we sync on-demand via search rather than full mirror).
- **FTS/search:** Czech-language FTS5 over `foodstuff.title` + `foodstuff.brand` so offline lookups work without round-tripping to the autocomplete endpoint. Strip diacritics for tolerant matching.
- **Date encoding:** API uses `DD.MM.YYYY` (Czech locale). Generator must encode/decode this format, accept `--date today/yesterday/2026-05-21` from users and convert.

## Codebase Intelligence
- **Source 1:** [TomasHubelbauer/kaloricke-tabulky-api](https://github.com/TomasHubelbauer/kaloricke-tabulky-api) — TypeScript/Bun wrapper, proves: login flow (MD5 password), weight-add, summary-get with `monthWeight[]` for weight history. ~5 endpoints documented.
- **Source 2:** Live AngularJS bundles. Mined `bundledHpJs.js` (homepage, public endpoints) + `controller-diary-*.js` (logged-in diary controller, 30+ endpoints exposed). All endpoints follow `root + 'segment/...?format=json'`.
- **Source 3:** Live authenticated probing with the user-provided credentials. Confirmed: login, `/user/diary/<date>/get`, `/statistic/summary/<date>/get` return populated envelopes.
- **Auth:** Cookie session. Login posts `{email, password: md5(password_hex)}` to `/login/create?format=json`. Response is `{requestId, code, message, data}` envelope with `code: 0` on success, plus `Set-Cookie: JSESSIONID=...; <session-cookie-name>=...`. All subsequent calls send those cookies via `Cookie:` header.
- **Data model:** Diary day → `times[]` (meal slots; id 1=Snídaně/Breakfast, 2=Svačina, 3=Oběd/Lunch, 4=Svačina, 5=Večeře/Dinner) → `foodstuff[]` + `notes[]`. Summary day → `{todayEnergy, todayEnergyTarget, energyUnit (kJ|kcal), todayDrink, todayDrinkTarget, monthWeight[]}`. Foodstuff autocomplete item → `{clazz, id (16-hex), url (slug), title, type, unit (g|ml), value (kJ per unit), isLiquid, hasImage, brandName, locked}`.
- **Rate limiting:** Not observed during probing. AdaptiveLimiter conservative defaults are sufficient.
- **Architecture:** Spring MVC backend; URLs heavily resource-rooted (`/user/*`, `/statistic/*`, `/home/*`, `/autocomplete/*`); `?format=json` is the JSON content negotiation; HTML routes (`/potraviny/...`, `/recepty/...`, `/aktivita/...`) return rich `application/ld+json` script blocks with full nutrition keywords — a viable fallback for foodstuff detail when no JSON detail endpoint exists.

## User Vision
The user shared their personal account credentials, indicating they want a working authenticated CLI they can use day-to-day — search foods, view diary, log weight, log meals from the command line — not just a read-only catalog browser. They asked me to revert any test entries created during dogfood.

## Product Thesis
- **Name:** `kaloricke-tabulky-pp-cli` (binary), shipped as the `kaloricke-tabulky` CLI.
- **Why it should exist:** Kalorické Tabulky has 7.1M users and is the dominant Czech-language nutrition database, but every interaction goes through the AngularJS web app or the mobile app. There is no first-party API, no CLI, and the only OSS wrapper covers 3 endpoints (login + weight only). A proper CLI unlocks: (a) one-line Czech-food nutrition lookups (`kal food jablko`), (b) keystroke-fast diary logging from terminal-resident users, (c) agent-driven diet analysis (an LLM can `--json` query 30 days of diary and recommend changes), (d) offline FTS over the food catalog so lookups work on a train, (e) export workflows the web UI gates behind premium PDF/XLS.

## Build Priorities
1. **Search + lookup** (no auth): foods, activities, recipes by Czech text — backed by `/autocomplete/*` + JSON-LD scrape for detail. Ships value even before login.
2. **Auth + session** (cookie auth via MD5 password OR Chrome cookie import): `auth login --email --password` and `auth login --chrome` (for users who don't want passwords on disk).
3. **Daily diary + summary** (`/user/diary/<date>/get`, `/statistic/summary/<date>/get`): view, JSON, agent-friendly summarization.
4. **Weight log** (`POST /user/weight/add`, history via summary): record and trend.
5. **Local store + sync**: cache foodstuffs accessed (search-hit-driven) + every diary day pulled into SQLite for offline + cross-day analytics.
6. **Novel transcendence**: macro-target gap analysis, food substitution by macro distance, allergen mining from JSON-LD keywords, weight trend regression with regression line, frequency-of-foods over the trailing 30 days, "what should I eat to hit my protein target tonight" — these only work because we have the local store.
