# Nutrition CLI Brief (combo: USDA FoodData Central + NutritionValue.org)

## API Identity
- **Domain:** Food nutrition facts lookup, comparison, and nutrient ranking.
- **Sources (peers, no primary):**
  - **USDA FoodData Central (FDC)** — official REST API, `https://api.nal.usda.gov/fdc`, OpenAPI 3.0, ~600K foods (Foundation, SR Legacy, Survey/FNDDS, Branded). Free api.data.gov key; `DEMO_KEY` works rate-limited.
  - **NutritionValue.org** — server-rendered HTML nutrition database (no API). Search, food-detail, stateless compare, and nutrient-ranking pages, all replayable over standard HTTP with HTML extraction.
- **Users:** Dieters, athletes, developers building nutrition features, and AI agents that need trustworthy (cited, non-hallucinated) nutrition numbers.
- **Data profile:** Per-food macros + micros (vitamins, minerals, amino/fatty acids), serving sizes, daily-value %, dataType/brand metadata. High-gravity entity: **food** (fdcId / NV id).

## Reachability Risk
- **USDA FDC:** None. Verified live 200 with `DEMO_KEY` (`/v1/foods/search?query=cheddar` → 64,784 hits, rich JSON). No systemic 403s. Real risks (mitigate in CLI): 429 rate-limit (1000/hr real key, 30/hr DEMO_KEY) that can return HTTP 200 with an HTML body; 500s on deep pagination; government-shutdown availability. Mitigation: local SQLite cache + typed 429 errors + normalization layer for the 4 inconsistent dataTypes.
- **NutritionValue.org:** None technically. `probe-reachability` → `standard_http` (0.95 confidence, stdlib + surf both 200). Data pages serve full HTML to a browser UA. **ToS caveat:** homepage HTML prohibits scripted access and threatens IP blacklisting; user explicitly approved ("YOLO mode"). CLI must stay polite (conservative rate limiting, honest UA, no aggressive crawl) and document the caveat.

## Top Workflows
1. **One-shot lookup:** `food "chicken breast"` → best generic match with macros; `--source usda|nv|all`.
2. **Compare N foods on a common basis:** side-by-side macro table scaled per-100g / per-serving / **per-100kcal (protein density)** — including **cross-source** (USDA vs NV for the same food).
3. **Rank/filter discovery:** "top foods high in <nutrient>" (NV ranking pages + USDA sort), compound filters (high protein, under N kcal).
4. **Portion & serving math:** per-100g → any serving/household measure via foodPortions.
5. **Daily intake log with targets:** log/today/summary/targets/progress backed by local SQLite; agent-native JSON makes it a Claude-loggable tracker.

## Table Stakes (union of every competing tool — see absorb manifest)
Search (dataType/brandOwner/category filters, UPC/GTIN barcode, ingredient search); get by id (abridged/full, nutrient subset); batch get (<=20, partial-failure tolerant); paginated list; top-N by nutrient; food categories; static nutrient reference table with name→id resolution; citation output (APA/MLA); portion scaling; common-basis compare (2-5 foods); %DV/DRI; JSON + `--select`; CSV export; local cache; DEMO_KEY fallback; multi-env key config.

## Data Layer
- **Primary entity:** `foods` unified table keyed by `(source, source_id)` — lossy cross-source model (USDA fdcId + NV id).
- Typed columns for high-gravity fields (name, calories, protein, fat, carbs, brand, dataType) + `raw_json`.
- **Sync cursor:** on-demand (search populates store); no global sync feed. Cache TTL domain-appropriate (reference data is stable → long TTL).
- **FTS/search:** FTS5 over food name/brand for offline search.
- **Nutrient reference:** static ~150-nutrient table (id, SR number, unit, category) for name→id resolution with zero API calls.

## Codebase Intelligence (absorb)
- No mature FDC CLI exists (best competitor < 5 stars; field wide open). Richest feature density is in MCP servers (cyanheads/usda-mcp-server, razvannicolae/Food-Facts-MCP, charliezstong/nutri-mcp).
- Winning architecture across the best tools (caltui, daveremy/nutrition-mcp, NUT): **bundled/cached local dataset + API fallback + aggressive caching + normalization layer.**
- NutritionValue.org has almost no tooling (only 2 scrapers) → being a first-class NV source is itself a differentiator.
- Auth consensus: accept key from flag > env (`FDC_API_KEY` **and** `USDA_API_KEY`) > `.env` > config; DEMO_KEY fallback with warning.

## User Vision
- User (YOLO mode) wants a combo of NutritionValue.org (the site they like) + USDA (the one with an API). Peers, no primary. Explicitly approved building against NV despite its ToS.

## Shared-ID insight (load-bearing for combo design)
NutritionValue.org's internal food IDs **are literally USDA FDC IDs / FNDDS food codes** (cheddar id 173414 = FDC 173414), and NV's underlying numbers are mirrored USDA data. Consequences:
- Cross-referencing is trivial and clean — one id key works on both sources; `food 173414` resolves on either.
- Raw macros for SR/Foundation/Survey foods are **identical** across sources, so "compare the same food's calories on USDA vs NV" is redundant. The real NV value-add is its **derived analytics USDA's API does not expose**: omega-6/omega-3 ratio, net carbs, percentile-of-database ranking, calories-by-source breakdown, %DV for individual amino acids, protein-by-amino-acid, human portion conversions, "low carb" style badges.
- NV also has genuinely unique content: **community foods** (`s`-prefixed shared ingredients) and **public recipes** (`p`-prefixed), and **precomputed nutrient-ranking pages** for ~60 nutrients filterable by category/dataset.
- **Design consequence:** NV is a first-class source contributing an *analytics/enrichment overlay* + rankings + community items, NOT a duplicate data feed. Cross-source `compare`/`enrich` shows USDA's authoritative nutrient arrays alongside NV's derived metrics (omega ratio, net carbs, percentile) — additive, not redundant.

## Source Priority (combo)
- **Peers, no primary** (confirmed). Both feed a unified `foods` model.
- USDA: official OpenAPI, free key (DEMO_KEY testable). Seeds generation (dominant spec source).
- NutritionValue.org: no spec — hand-authored HTML-extraction source layer (aggregator pattern), `internal/source/nutritionvalue/`.
- **Economics:** USDA needs a free key (DEMO_KEY works); NV needs no key. Keep NV commands key-free; USDA commands read the key with DEMO_KEY fallback so nothing hard-blocks on a missing key.
- **Inversion note:** USDA has the clean 4-endpoint spec; NV has none. Do NOT let spec completeness demote NV — user values NV. NV gets equal command billing (search/food/compare/rank all support `--source nv`).

## Product Thesis
- **Name:** nutrition (`nutrition-pp-cli`)
- **Thesis:** The first agent-native nutrition CLI that unifies USDA FoodData Central and NutritionValue.org behind one `food`/`compare`/`rank` surface, with a local SQLite cache, offline FTS search, cited numbers (no hallucination), cross-source comparison no other tool offers, and a normalization layer that tames USDA's four inconsistent dataTypes.

## Build Priorities
1. **P0 foundation:** USDA-generated baseline (search, food get, foods batch, foods list) + unified `foods` store + FTS + static nutrient reference table + normalization layer + key config (env/DEMO_KEY fallback).
2. **P1 absorb:** every table-stakes feature above, matched with `--json`/`--select`/`--csv`, typed exit codes, local caching, 429 handling.
3. **P2 transcend:** cross-source compare (USDA vs NV), per-100kcal protein-density ranking, NV nutrient-ranking ingestion, daily-intake log with targets, citation output, high-in-nutrient compound filters, barcode lookup.

## Reachability Gate (Phase 1.9)
- Decision: PASS
- USDA FDC: live 200 with DEMO_KEY on `/v1/foods/search` (64,784 hits, rich JSON). No auth-type is OAuth; apiKey-in-query — no OAuth grant probe needed.
- NutritionValue.org: `probe-reachability` → standard_http (0.95); food detail + search + compare + ranking all 200 with browser UA over standard HTTP.
- No HARD STOP conditions. Both sources reachable.
