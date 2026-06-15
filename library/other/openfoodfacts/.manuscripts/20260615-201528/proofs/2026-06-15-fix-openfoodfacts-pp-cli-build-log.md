# Open Food Facts CLI — Build Log

## Generated
- Spec: bundled OFF v2 OpenAPI (redocly-bundled from openfoodfacts-server/docs/api/ref, 0 external refs, 12 paths). `x-pp-resource: find` added to /api/v2/search to avoid collision with the framework's reserved `search` (offline FTS) command.
- Clean generation passed all gates (govulncheck, vet, build, doctor). No auth (OFF reads are open data).
- Description surfaces (root Short, SKILL frontmatter, goreleaser) all driven by narrative.headline.

## Command surface
- Endpoint-mirror (P1 absorbed): `product` (barcode lookup, all fields/knowledge-panels/blame), `find` (online search with full tag + nutrient-range + sort + pagination filters), `attribute-groups`, `preferences`, `cgi *` (suggest, nutrients, ingredients, image ops). Framework: `search` (offline FTS), `sync`, `analytics`, `tail`, `import`, `doctor`, `which`, `api`, `profile`, `workflow`, `auth`, `feedback`, `agent-context`.

## Transcendence (P2) — all 8 built and live-verified
1. `diary add <code> [--grams|--servings]` — log food, scale per-100g, persist to local SQLite diary_entry.
2. `diary today` — sum today's macros vs goal, show remaining. (Verified: 30g Nutella = 161.7 kcal, remaining 1838 of 2000.)
3. `diary goal [--kcal --protein --fat --carbs]` — set/show daily goal (upsert).
4. `diary since <date>` — per-day totals + averages over range.
5. `compare <c1> <c2> ...` — side-by-side per-100g + Nutri-Score/NOVA/Eco. (Verified: Nutella 539/E vs Prince 465/D.)
6. `swap <code> [--max-nova N]` — healthier same-category alternatives. (Verified: 10 alts, all NOVA≤3, all nutriscore better than Nutella's 30.)
7. `recipe <c...> --servings N` — sum macros, per-serving block. (Verified: per_serving == total/4.)
8. `allergens set <list>` + `allergens check <code>` — stored profile, set-intersect, exit 3 on HIT. (Verified: Nutella vs milk,gluten → HIT milk, exit 3.)
9. `budget <query>` — search within remaining daily kcal. (Verified: 20 results, 0 over budget.)
10. `rank --category <c> --sort <nutrient>` — offline ranking over synced `find` table. (Verified: honest empty + guidance when store empty.)

## Hand-written code
- `internal/cli/novel_support.go` — shared nutriment math, product/search fetch+parse, local schema (diary_entry/diary_goal/pref_kv), friendly-nutrient mapping, --db resolution.
- Per-command files (compare/swap/recipe/budget/rank/diary_*/allergens_*) implement RunE; verify-friendly (no MinimumNArgs/MarkFlagRequired; len(args) guard + dryRunOK before IO).
- `internal/cli/novel_features_test.go` — real table-driven tests (nutriment scale/add, resolveGrams, friendlyNutrientKey, normalizeAllergenList, allergenHits, prodTags/canonical-tag selection, categoryMatches, coerceFloat). All pass.

## Bugs fixed during build
- `truncate` redeclare → reused existing helper.
- Novel command `search` collided with reserved framework command → renamed online-search command to `find` (research.json + narrative updated); diet-budget novel command renamed `budget`.
- `swap` returned 0: `prodFirstRawTag` picked the localized trailing category (`en:Pâtes à tartiner`), not a search-valid canonical tag → now selects most-specific canonical `en:slug` tag. Fixed + tested.

## Deferred / out of scope
- Write endpoints (product update, image upload, session) — auth-gated, not stubbed, excluded by design.
- v3 product endpoint not merged (v2 covers lookup + search; v3 lacks search).

## Test/build status
- `go build ./...` clean, `go vet ./...` clean, `go test ./...` all packages pass.
