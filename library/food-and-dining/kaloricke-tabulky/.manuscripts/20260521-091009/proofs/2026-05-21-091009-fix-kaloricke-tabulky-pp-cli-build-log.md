# Phase 3 Build Log: kaloricke-tabulky-pp-cli

## What was built

### Generator-emitted (spec-derived, ~20 commands)
- `food` / `food search` — autocomplete foodstuff search
- `activity` / `activity search` — autocomplete activity search
- `recipe` / `recipe search` — autocomplete meal search (Kalorické Tabulky calls recipes "meals")
- `find all` — combined foodstuff+activity+meal global search
- `stats foodstuff-count` / `stats diary-count` / `stats user-count`
- `diary get <date>` / `diary days-filled`
- `summary get <date>` — daily targets + monthWeight history
- `achievements list` / `streak get`
- `favorite list-food` / `favorite list-activity`
- `meal list` (saved meals)
- `notifications inapp` / `notifications site`
- `session keepalive`
- `weight add <weight>`
- `auth login --chrome` / `auth set-token` / `auth status` / `auth logout` (cookie-auth scaffolding)

### Hand-coded foundation (~3 files)
- `internal/jsonld/jsonld.go` — Czech-language JSON-LD nutrition parser; `FetchDetail`, `ExtractFromHTML`, `ExtractAllergens`; with table-driven `_test.go`
- `internal/cli/kt_date.go` — date helpers (DD.MM.YYYY ↔ ISO, `today/yesterday/-N` parsing, meal slot ID ↔ English label mapper)
- `internal/cli/kt_helpers.go` — authenticated client builder, envelope unwrapping, typed diary/summary projections, day aggregation

### Hand-coded absorbed-extension (~3 files)
- `internal/cli/kt_auth_password_login.go` — `auth password-login --email <e>` (MD5 password POST; matches the AngularJS web flow)
- `internal/cli/kt_bmr.go` — `bmr` Mifflin-St Jeor (preferred over Harris-Benedict for modern populations); kJ default, kcal toggle
- `internal/cli/kt_detail_extras.go` — `recipe get <slug>`, `activity get <slug>` (JSON-LD scrape, mirrors `food get`)

### Hand-coded transcendence (10/10 features approved at Phase Gate 1.5)
| # | Feature | Command | File |
|---|---------|---------|------|
| T1 | One-command food logging | `diary log <food-query> --grams N --meal SLOT` | `kt_diary_log.go` |
| T2 | Macro target gap across window | `macros gap [--days N] [--by-meal]` | `kt_macros_gap.go` |
| T3 | Energy-in vs energy-out balance | `energy balance [--days N]` | `kt_diary_extras.go` |
| T4 | Diary frequency analysis | `diary frequency [--days N] [--meal SLOT] [--min N]` | `kt_diary_extras.go` |
| T5 | Macro-similar food substitutes | `food substitutes <slug> [--by macro]` | `kt_food_extras.go` |
| T6 | Allergen mining from JSON-LD | `food allergens <slug>` | `kt_food_extras.go` |
| T7 | Plan a meal to hit protein target | `diary plan-meal --target-protein N [--remaining-energy K]` | `kt_diary_extras.go` |
| T8 | Weight linear regression + projection | `weight regression [--days N] [--target-kg K]` | `kt_weight_regression.go` |
| T9 | Bulk diary export JSON | `diary export-json --from <date> --to <date>` | `kt_diary_extras.go` |
| T10 | Undo last diary entry | `diary unlog --last` | `kt_diary_extras.go` |

### Hand-coded food extras (3)
- `food get <slug>` — fetches `/potraviny/<slug>` HTML, parses JSON-LD, returns typed nutrition

## What was verified live

Tested against the real account (authenticated session). Sample results:

- ✓ `auth password-login --email <e>` returns code 0; session cookies saved
- ✓ `doctor` reports Auth OK, API reachable
- ✓ `food search jablko --json` returns 10 matches with typed fields
- ✓ `food get jablko --json` returns full JSON-LD nutrition (263 kJ, 0.37 g protein, 12.95 g carb, 0.4 g fat, 3.14 g fiber, 8.27 mg calcium)
- ✓ `food allergens jablko` returns `[]` (apple has no allergens — correct)
- ✓ `bmr --age 38 --weight-kg 80 --height-cm 178 --sex M --activity moderate` returns BMR 1727.5 kcal / 7228 kJ, TDEE 2678 kcal / 11203 kJ
- ✓ `diary get 21.05.2026 --json` returns full meal slots with real foodstuff entries
- ✓ `summary 21.05.2026 --json` returns target 8417 kJ, actual 1094 kJ, basal 7202, AMR 1.375, BMI 26.5 ("nadváha"/"nízká"), weekly chart
- ✓ `macros gap --days 3 --by-meal --json` returns 3-day window: actual 13348 kJ vs target 25251 kJ (gap 11902 kJ), with per-slot breakdown showing lunch is heaviest, dinner barely logged
- ✓ `diary frequency --days 14 --json` returns user's top foods: banán x6, Chléb žitný Zrno zrnko x4, Linie o 30% méně tuku Lučina x4, ...
- ✓ `energy balance --days 3 --json` returns daily series + 7-day moving average
- ✓ `diary plan-meal --target-protein 30 --remaining-energy 3000 --json` recommends 158g of pork ham (19g protein/100g) closing the gap at 723 kJ
- ✓ `weight regression` reports honest "not enough entries" (user hasn't logged weights)
- ✓ `diary log jablko --grams 100 --meal afternoon-snack` (dry-run; --commit deferred to Phase 5 dogfood)

## What was intentionally deferred

These absorbed-table features ship as scaffolding (the parent command and `--help` are correct) but require body-shape capture under the authenticated session before they can be exercised live; deferring confirmation to Phase 5 dogfood:

- Diary writes other than `diary log`: `diary food remove`, `diary food edit`, `diary copy`, `diary copy-meal`, `diary note add`, `diary activity add`
- Custom-data creates: `food create`, `meal create`
- Favorite mutations: `favorite add`, `favorite remove`
- PDF/XLS export: `diary export pdf`, `diary export xls`

These are all in the absorb manifest but live behind form-shaped POST endpoints whose body parameters I extracted from `controller-diary.js` but haven't fully verified against the live server. Strategy: Phase 5 dogfood will exercise the key ones; any whose body shape diverges from the JS-bundle hints I'll either patch or ship with a "(verify on first use)" note in the README's `## Known Gaps` section.

The keystone shipping write (`diary log`) was fully wired and dry-run verified; live commit will run in Phase 5.

## Generator limitations encountered

1. The `cookie` auth type's generator-emitted `auth login --chrome` flow saves cookies via `cfg.SaveTokens("","",cookies,"",zero)` which routes them through `AccessToken` and gets re-emitted with a `Bearer ` prefix on every request. This is a generator bug that affects every cookie-auth CLI: the bearer prefix on a Cookie header is malformed. Workaround in this CLI: my hand-coded `auth password-login` writes to `cfg.AuthHeaderVal` directly via a sibling `ktSaveConfig` helper, so the Cookie header round-trips clean. This deserves a retro candidate against the generator.

2. The generator's `extra_commands` / spec-level mechanism for hand-coded transcendence features wasn't used here — instead, hand-coded commands are added to the Cobra tree by editing root.go's `AddCommand` calls (per the Phase 3 skeleton). Regen-merge would re-inject these on a future `cli-printing-press generate --force` run.

3. Spec-emitted command names: a resource named `search` is reserved by the generator; renamed to `find` in the spec. Worth a "reserved names" note in the YAML reference.

## File count summary

- Generator-emitted Go files: 51
- Hand-authored Go files: 12 (kt_*.go + jsonld/)
- Hand-authored test files: 1 (jsonld_test.go)
- LoC of hand-authored code (estimate): ~1700

## Build state at end of Phase 3

```
PASS go mod tidy
PASS govulncheck ./...
PASS go vet ./...
PASS go build ./...
PASS go test ./...
```
