# Allrecipes CLI Brief (re-validated 2026-05-03)

This brief is a **re-validated reuse** of the 2026-04-26 brief from run
`20260426-230519`. It is updated against the current binary (v3.7.0) and
the current state of allrecipes.com. Sections without re-validation
findings are kept verbatim from the prior brief; sections with material
deltas are marked **[updated 2026-05-03]**.

## API Identity

- **Domain**: cooking, recipe discovery, ingredient-driven cooking
- **Users**: home cooks, agentic shoppers, weeknight diners, cookbook builders
- **Data profile**: ~250k recipes with full Schema.org Recipe JSON-LD on every
  recipe page (title, ingredients with qty+unit+name, instructions,
  prep/cook/total time, yield, nutrition, ratings, review counts, "Made It"
  counts, author, images, cuisine, category, keywords). Rich review text per
  recipe. Categories: dish type, cuisine, ingredient, occasion, diet, holiday.

## Reachability Risk **[updated 2026-05-03]**

Two-tier surface — different transports per page type:

| Surface | Probe result | Transport |
|---|---|---|
| `/` (home) | `mode: standard_http`, stdlib HTTP 200 | plain stdlib |
| `/search?q=…` | `mode: standard_http`, stdlib HTTP 200 | plain stdlib |
| `/recipe/<id>/<slug>/` (detail) | `mode: browser_clearance_http`, stdlib 403 (`cf-mitigated: challenge`), surf-chrome 404 | needs Cloudflare clearance cookie |

**Material change vs prior brief**: prior assumed Surf (`http_transport:
browser-chrome`) alone would clear the wall. As of today,
`printing-press probe-reachability` returns `browser_clearance_http` for
recipe detail pages — Surf gets a Cloudflare-flagged 404. Recipe details
need a clearance cookie captured via `auth login --chrome`.

Browse and search surfaces still work without clearance.

**Mitigation strategy** (approved by user 2026-05-03):

- Default transport for browse/search: plain stdlib (no clearance).
- Recipe-detail commands gate on a Chrome clearance cookie via
  `auth login --chrome`. The user is not authenticating an Allrecipes
  account — only capturing a per-browser bot-protection token.
- `doctor` should diagnose missing-clearance-cookie on a recipe-detail call
  and prescribe `auth login --chrome` instead of failing opaquely.

## Top Workflows

(verbatim from prior brief)

1. **Find a recipe** — `search "brownies"`, sorted by rating × review count.
2. **Get the full recipe as data** — `recipe https://www.allrecipes.com/recipe/.../...` returns ingredients/instructions/times/nutrition/rating as parsed JSON.
3. **Build a grocery list from a meal plan** — pick N recipes → aggregate de-duped ingredient quantities.
4. **Scale a recipe** — change servings; quantities and (best-effort) units rescale.
5. **Browse what's good in a category/cuisine** — top-rated Italian, top-rated weeknight, top-rated under 30 min.
6. **Export to markdown / shareable file** — agent writes a recipe to a file in clean markdown for cooking-mode reading.

## Table Stakes (incumbent features we must match)

(verbatim from prior brief)

Search (q + pagination), category browse, cuisine browse, ingredient browse.
Full recipe extraction: title, ingredients (qty+unit+name), instructions,
prep/cook/total time, servings/yield, author, image, nutrition, rating,
review count, "Made It" count. Reviews list. JSON output everywhere; markdown
export. Robust to Cloudflare interstitial.

## Data Layer

- **Primary entities**: `recipes`, `ingredients` (per-recipe rows: qty, unit,
  name, raw_text), `reviews`, `categories`, `cuisines`, `nutrition`.
- **Sync cursor**: not strictly needed (no auth, no per-user state). But a
  local cache keyed by recipe ID + URL is essential — every successful fetch
  populates the cache so search/scale/export work offline thereafter.
- **FTS**: title, ingredient names, cuisine, category, keywords from JSON-LD.
  Enables transcendence features (pantry-match, dietary-filter, swap-aware
  search).

## Codebase Intelligence

(verbatim from prior brief — confirmed still accurate)

- **recipe-scrapers (hhursev/recipe-scrapers)** — `allrecipes.py` is
  intentionally minimal; inherits `AbstractScraper` which uses JSON-LD with
  HTML selector fallbacks. **JSON-LD is the primary surface.**
- **remaudcorentin-dev/python-allrecipes** — surfaces
  `AllRecipes.search(query)` and `AllRecipes.get(url)`.
- **ryojp/recipe-scraper (Go, Colly)** — full-coverage Go scraper.
- **marcon29/CLI-dinner-finder-grocery-list (Ruby CLI)** — only known CLI
  competitor; interactive only.
- **Apify Allrecipes Data Extractor** — paid commercial scraper.

## Re-validation Notes (v2.3.9 → v3.7.0)

| Bucket | Material delta | Action this run |
|---|---|---|
| **Transport / reachability** | Cloudflare tightened on recipe detail pages; Surf insufficient. | Adopt `auth login --chrome` for clearance; per-resource transport in spec. |
| **MCP surface** | Pre-generation MCP enrichment exists for >30 tool surfaces; runtime cobratree mirror replaces static MCP list; NoCache=true is default; spec `mcp.transport`/`orchestration`/`intents`/`endpoint_tools` keys available. | Estimated 36+ commands → 40-50 MCP tools at runtime. Decide enrichment in Phase 2 pre-gen. |
| **Scoring rubrics** | Insight prefix, MCP dir selection, freshness coupling, live-check empty-handling treats as PASS. | Scorecard will reflect runtime cobratree surface; previously static-MCP scoring would have under-counted. |
| **Discovery** | `validate-narrative` subcommand validates README/SKILL command examples; browser-sniff v2 traffic-analysis shape. | Run `validate-narrative` after generation per skill Phase 2 step. |
| **Auth modes** | Chrome cookie auth pattern documented and templated (`auth login --chrome`). | Use this for clearance cookie capture. |

## User Vision

> "do it for unauthenticated scenarios only. do not require authentication"

> [2026-05-03] "Allow Chrome clearance cookie (recommended)" — user
> approved a Cloudflare clearance cookie captured via `auth login --chrome`,
> on the explicit understanding that this is bot-protection clearance, not
> Allrecipes account login. No login form, no token storage tied to a user
> account, no saved-recipes / meal-plan / profile features.

The CLI ships with `auth login --chrome` (clearance cookie capture only),
no account login flow, no saved-recipes API.

## Product Thesis

- **Name**: Allrecipes Pocket (binary `allrecipes-pp-cli`)
- **Why it should exist**: Allrecipes is the largest crowd-rated recipe corpus
  on the web — 250k+ recipes, thousands of "Made It!" datapoints per popular
  recipe — but the website is ad-heavy, story-driven, and slow on mobile.
  Power users want recipes-as-data: search, scale, shop, export, filter by
  ingredient, all from the terminal or their agent. JSON-LD makes the data
  clean; SQLite makes it fast and composable; a Cloudflare clearance cookie
  unwalls the recipe pages. Nothing in the existing tool zoo combines these.

## Build Priorities

1. **Foundation**: per-resource transport (stdlib for browse/search,
   browser-clearance for recipe details), JSON-LD extractor lifted from
   recipe-goat shape, SQLite schema for recipes/ingredients/reviews.
2. **Absorbed (P1)**: 28 absorbed features per the prior absorb manifest
   (re-validated; same scope).
3. **Transcendence (P2)**: 8 transcendence features per the prior absorb
   manifest, all scoring ≥5/10.
4. **Polish (P3)**: SKILL recipes pairing `--agent` and `--select`, README
   cookbook section, doctor that names the Cloudflare clearance failure
   mode (no longer just "TLS interstitial").
