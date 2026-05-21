# kaloricke-tabulky CLI Absorb Manifest

## Source landscape

- **Direct wrapper:** [TomasHubelbauer/kaloricke-tabulky-api](https://github.com/TomasHubelbauer/kaloricke-tabulky-api) — Bun/TypeScript wrapper with 3 endpoints (login, weight-add, weight-history-via-summary). MIT/no-license.
- **Generic CLI competitors:** [aquilax/hranoprovod-cli](https://github.com/aquilax/hranoprovod-cli) (Go, nested recipes), [zupzup/calories](https://github.com/zupzup/calories) (Harris-Benedict BMR), [hstsethi/dietcli](https://github.com/hstsethi/dietcli) (C++), [vrublack/TacoShell](https://github.com/vrublack/TacoShell) (Java, USDA), [peterkeen/calorific](https://github.com/peterkeen/calorific) (recipes + dated entries), [pfirsich/welo](https://github.com/pfirsich/welo) (weight tracker), [guitarkeegan/diet-tracker](https://github.com/guitarkeegan/diet-tracker).
- **MCPs:** None for Kalorické Tabulky specifically. Adjacent: nutritionix-mcp-server, mcp-opennutrition, thitiph0n/calorie-tracker-mcp-server (US-centric, no Czech food DB).
- **Verdict:** No CLI/MCP/skill covers Kalorické Tabulky. The direct wrapper covers ~5% of the API surface. Generic CLIs track locally with no live catalog; they cannot match a 244k-Czech-food online database.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Login with MD5 password | TomasHubelbauer | `auth login --email <e>` (interactive password from stdin, never persisted) | Also `auth login --chrome` to import session cookies; session cookie cached in `~/.config/kaloricke-tabulky/session`; no password on disk |
| 2 | Search foods by Czech text | Web UI `/autocomplete/foodstuff` | `food search <text> [--page N] [--json]` | Diacritics-tolerant FTS5 over local store; `--select` for narrow fields; `--csv` |
| 3 | Search activities by text | Web UI `/autocomplete/activity` | `activity search <text> [--page N]` | Same FTS5 + structured output |
| 4 | Search recipes (meals) | Web UI `/autocomplete/meal` | `recipe search <text>` | Same |
| 5 | Combined global search | Web UI g-search | `search <text>` | One command, mixed hits with `--kind` filter |
| 6 | Public counters (foods/users/diary) | Web UI `/home/*/count` | `stats public` | `--watch` for live |
| 7 | Food detail with nutrition | Web UI `/potraviny/<slug>` JSON-LD | `food get <slug>` | Parses JSON-LD to typed nutrition: energy(kJ/kcal), protein(g), carb(g), fat(g), fiber(g), saturated/mono/poly fat, sugars, calcium. `--per 100g` and `--per <weight>g` |
| 8 | Activity detail | Web UI `/aktivita/<slug>` | `activity get <slug>` | JSON-LD parsed; `--duration-min N` for kcal estimate |
| 9 | Recipe detail | Web UI `/recepty/<slug>` | `recipe get <slug>` | JSON-LD parsed; ingredient list + per-serving macros |
| 10 | View daily diary | Web UI `/user/diary/<date>/get` | `diary get [--date today\|YYYY-MM-DD\|DD.MM.YYYY]` | Default today; groups by Snídaně/Svačina/Oběd/Svačina/Večeře with English aliases |
| 11 | Daily summary (energy, drink, macros vs target) | Web UI `/statistic/summary/<date>/get` | `summary [--date]` | `--json` includes targets, %-of-target derived locally |
| 12 | Record weight | TomasHubelbauer + `/user/weight/add` | `weight add <kg> [--date]` | `--dry-run`; idempotent on same date |
| 13 | Weight history | TomasHubelbauer | `weight history [--days N]` | Mines `monthWeight[]` from summary endpoint |
| 14 | Add food to diary | `/user/diary/foodstuff/add` | `diary food add <foodstuff-id> --grams N --meal <slot> [--date]` | `--dry-run`; English meal aliases |
| 15 | Add activity to diary | `/user/activity/add?format=json` | `diary activity add <activity-id> --minutes N [--date]` | `--dry-run` |
| 16 | Add diary note | `/user/diary/note/add` | `diary note add <text> [--date] [--meal SLOT]` | |
| 17 | Delete diary entry | `/user/diary/foodstuff/delete/<id>` | `diary food remove <entry-id>` | `--dry-run` |
| 18 | Edit portion (unit) of diary entry | `/user/diary/foodstuff/unit/edit` | `diary food edit <entry-id> --grams N` | |
| 19 | Move diary entry to different slot | `/user/diary/foodstuff/time/edit` | `diary food edit <entry-id> --meal <slot>` | |
| 20 | Copy diary day | `/user/diary/copy?format=json` | `diary copy --from <date> --to <date>` | |
| 21 | Copy single meal slot | `/user/diary-time/copy` | `diary copy-meal --from <date> --slot lunch --to <date>` | |
| 22 | Create custom foodstuff | `/user/foodstuff/add?format=json` | `food create --title T --energy E --protein P ...` | `--dry-run` |
| 23 | Create custom meal | `/user/meal/create?format=json` | `meal create --title T --foods <id:grams,...>` | `--dry-run` |
| 24 | View favorites | `/user/settings/favorite/*` | `favorite list [--kind food\|activity]` | |
| 25 | Add/remove favorite | `/user/settings/favorite/<type>/{add,remove}/<id>` | `favorite add <id>` / `favorite remove <id>` | `--dry-run` |
| 26 | View saved meals | `/user/settings/meal/list` | `meal list` | `--limit`, `--json` |
| 27 | View achievements | `/statistic/analysis/achievements/get` | `achievements list [--type TYPE]` | |
| 28 | View streak | `/user/streak?format=json&date=<date>` | `streak [--date]` | |
| 29 | Which days have diary entries | `/user/diary/filled-out` | `diary days-filled [--month YYYY-MM]` | |
| 30 | In-app messages | `/user/messages/inapp` | `notifications` | |
| 31 | Site-wide messages | `/site/messages` | `notifications --site` | |
| 32 | Export day PDF | `/user/export/pdf/<date>` | `diary export pdf <date>` | `--output <path>` |
| 33 | Export day XLS | `/user/export/xls/<date>` | `diary export xls <date>` | `--output <path>` |
| 34 | Session keepalive | `/session/keepalive` | `auth refresh` | |
| 35 | Settings: common activity targets | `/user/settings/common/activity` | `settings activity` | |
| 36 | BMR (Harris-Benedict) | zupzup/calories competitor | `bmr [--age N --weight kg --height cm --sex F\|M --activity <level>]` | Inputs optional if `summary` has them; kJ default, `--unit kcal` toggle |
| 37 | Dated entries / today-default | All generic competitors | `diary` (today) + `--date` everywhere | Accepts `today\|yesterday\|-N\|YYYY-MM-DD\|DD.MM.YYYY` |
| 38 | Tracked-nutrient flexibility | calorific competitor | `--select energy,protein,carb,fat,fiber,sugar,calcium,saturatedFat,monoFat,polyFat` on every output | We have JSON-LD typed nutrition; competitors hand-curate |

**Total absorbed: 38 features. No stubs.**

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | One-command food logging | `diary log <food-query> --grams N --meal SLOT [--date]` | 9/10 | hand-code | Resolves food-query against local FTS5 cache (top hit, or `--pick` if ambiguous), then POSTs `/user/diary/foodstuff/add` | Brief Top Workflow #2, persona Tereza's click-cost; absorbed only exposes ID-based add |
| 2 | Macro target gap across window | `macros gap [--days N] [--by meal]` | 8/10 | hand-code | Reads cached daily summaries + diary for window, computes `target - actual` per macro, groups by meal slot | Brief Product Thesis ("macro-target gap analysis"); persona Tereza |
| 3 | Energy-in vs energy-out balance | `energy balance [--days N]` | 7/10 | hand-code | Joins per-day diary energy with activity energy from local store; daily series + moving average | Persona Martin |
| 4 | Diary frequency analysis | `diary frequency [--days N] [--meal SLOT] [--min N]` | 7/10 | hand-code | SQL count over cached diary entries by foodstuff_id | Persona Lenka; brief lists "frequency-of-foods" |
| 5 | Macro-similar food substitutes | `food substitutes <id> [--by protein\|carb\|fat\|energy]` | 7/10 | hand-code | Euclidean distance over typed nutrition struct from JSON-LD | Brief Product Thesis ("food substitution by macro distance") |
| 6 | Allergen mining from JSON-LD | `food allergens <foodstuff-id\|slug>` | 6/10 | hand-code | Fetches `/potraviny/<slug>`, parses JSON-LD `keywords`, regex-matches Czech allergen tokens | Brief Product Thesis ("allergen mining") |
| 7 | Plan a meal to hit protein target | `diary plan-meal --target-protein N [--remaining-energy K] [--meal SLOT]` | 7/10 | hand-code | Reads today's summary; greedy-selects from favorites + frequents within energy budget | Brief Product Thesis ("what should I eat to hit my protein target") |
| 8 | Weight linear regression | `weight regression [--days N] [--target-kg K]` | 6/10 | hand-code | OLS over `monthWeight[]`; slope + R^2 + days-to-target | Persona Lenka ("0.4 kg/week") |
| 9 | Bulk diary export JSON | `diary export json --from <date> --to <date>` | 6/10 | hand-code | Iterates cached daily diary, rolls into one JSON with typed totals | Persona Lenka ("PDF is write-only") |
| 10 | Undo last diary entry | `diary unlog --last [--meal SLOT]` | 5/10 | hand-code | Reads most-recent cached entry id, calls delete endpoint | User Vision explicit ask |

**Total transcendence: 10 features. All hand-code. No stubs.**

## Build summary

- **Absorbed:** 38 features (all spec-emits except `bmr`, `food allergens`-aware variants of `food get`, and JSON-LD parsing helpers which are hand-coded once but enable 8 commands)
- **Transcendence:** 10 features, all hand-code, ~1500 LoC total estimated
- **No stubs.** Every feature ships fully-implemented.
- **Hand-code count for Phase Gate 1.5:** 10 hand-code transcendence + ~3 hand-code absorbed support (BMR formula, JSON-LD parser, FTS dia-fold normalizer) = **~13 hand-code commitments**.
