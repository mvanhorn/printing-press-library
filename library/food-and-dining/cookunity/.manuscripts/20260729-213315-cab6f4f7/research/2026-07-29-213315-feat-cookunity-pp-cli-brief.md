# CookUnity CLI Brief

## API Identity
- Domain: Chef-driven prepared-meal delivery subscription. Independent chefs post a
  rotating weekly menu (300+ dishes, 100+ chefs); subscribers pick 4-16 meals/week,
  delivered ready-to-heat. Menu published ~2 weeks ahead; orders editable/skippable
  until a weekly cutoff.
- Users: Subscribers who browse the weekly menu, select meals, manage a plan/
  subscription, schedule/track deliveries, and rate meals. Power users want to plan
  meals against dietary and macro constraints.
- Data profile: **Meals** (name, chef, cuisine/category, dietary tags, macros/nutrition,
  price/credits, image, availability week) is the highest-gravity entity. Secondary:
  chefs, weekly menu windows, orders/deliveries, subscription/plan, ratings/favorites,
  delivery address/zones.

## Reachability Risk
- Level: **Low**. Marketing origin (www.cookunity.com) returns HTTP 200 over plain
  stdlib HTTP and surf-chrome (probe mode=standard_http, confidence 0.95). No transport
  bot-protection. Authenticated menu/account API is a separate backend surface to be
  discovered via browser-sniff with the user's logged-in Chrome session.
- No official public API; no community wrapper/SDK/MCP/CLI found. The one GitHub hit
  ("Cookunity-Challenge") is an unrelated coding exercise, not the real API.
- Auth: browser-session (cookie) based via logged-in web app. User confirmed logged in
  to Chrome (AUTH_SESSION_AVAILABLE=true). Pattern mirrors library's `harris-teeter`
  CLI ("grocery shopping API discovered from the logged-in web app").

## Top Workflows
1. **[PRIMARY / user vision]** Export the complete list of available meals to a local
   store for offline meal planning — full menu with nutrition, chef, dietary tags,
   category, and price.
2. Filter/search meals offline by diet (GF/DF/keto/paleo/vegan), protein, calories,
   macros, cuisine, or chef, to build a week's plan.
3. View and manage the current subscription and the upcoming (editable) order before
   cutoff; see meals-per-week plan and credits.
4. Delivery schedule / upcoming delivery window and address.
5. Ratings / favorites — what have I liked; surface recommendations from history.

## Table Stakes
- Browse the current weekly menu (list + detail per meal).
- Filter by dietary tag, cuisine, chef.
- View nutrition/macros per meal.
- See upcoming order and delivery date.
- Authentication via existing logged-in browser session (cookie import).

## Data Layer
- Primary entities: meals, chefs, menu-weeks, orders, deliveries, subscription, ratings.
- Sync cursor: weekly menu window (menu changes each week; snapshot per week enables
  historical availability + drift across weeks).
- FTS/search: full-text over meal name + description + chef + dietary tags + cuisine.
- Offline is the whole point: the primary workflow is querying the mirrored menu with
  no network, so a robust SQLite store + `sync` + `search`/`sql` is the foundation.

## User Vision
- "I'd like to pull a complete list of available meals so that I can do my own meal
  planning offline." Headline = full menu export + offline query/filter/plan. This is
  the first-run experience and the top of the README.

## Product Thesis
- Name: cookunity-pp-cli
- Why it should exist: CookUnity's web app is the only way to see the menu, it requires
  login, and there is no way to take the menu offline, filter by macros, or diff week
  over week. A local-first CLI that mirrors the full menu into SQLite turns "browse the
  web app one meal at a time" into "query my whole menu offline, plan a week against my
  macros, and track what I've liked" — none of which any existing tool offers.

## Build Priorities
1. Data layer + `sync` for meals (full menu) + chefs; browser-session cookie auth.
2. Offline `menu`/`meals` list + detail, `search`, and `--json`/`--select` agent output.
3. Dietary/macro/cuisine/chef filters for meal planning.
4. Subscription + upcoming order + delivery views (read-only).
5. Transcendence: offline macro-constrained meal planner, week-over-week menu drift,
   ratings/favorites history, chef explorer.
