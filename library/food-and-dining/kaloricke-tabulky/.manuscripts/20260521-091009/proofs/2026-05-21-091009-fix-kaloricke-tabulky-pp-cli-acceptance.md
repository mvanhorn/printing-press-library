# Phase 5 Acceptance Report: kaloricke-tabulky-pp-cli

## Setup
- **Mode:** Quick Check (read-only; user chose to skip live writes during dogfood)
- **Auth:** Cookie session via `auth password-login --email the test workspace owner` (the test workspace owner). Cookies persisted to `~/.config/kaloricke-tabulky-pp-cli/config.toml`.
- **Anchor date:** 2026-05-21

## Mechanical test matrix
The Printing Press's `cli-printing-press dogfood --live --level quick` runner enumerated the agent-context command tree and ran one help + one happy-path + one JSON fidelity probe per leaf command in the Quick Check set.

**Verdict from binary-owned runner:** `pass` (6/6 leaves, 0 failures, 0 critical). Acceptance marker written to `phase5-acceptance.json` with `status: pass`, `level: quick`, `matrix_size: 6`, `tests_passed: 6`, `auth_context.type: cookie`.

## Agent-level verifications run during this session

Sanity checks performed earlier in the session against the live API (under the same authenticated session), summarized so the reviewer has visibility:

- `food search jablko` returned 10 typed foodstuff matches with id, slug, brand, unit
- `food get jablko` returned typed nutrition from the JSON-LD: 263 kJ, 0.37 g protein, 12.95 g carb, 0.4 g fat, 3.14 g fiber, 8.27 mg calcium
- `food allergens jablko` correctly returned `[]` (apple has no allergens)
- `summary 21.05.2026` returned target 8417 kJ, actual 1094 kJ, basal 7202 kJ, AMR 1.375, BMI 26.5, weekly chart
- `diary get 21.05.2026` returned the day's real meal-slot structure with all logged foodstuffs and macros
- `macros gap --days 3 --by-meal` returned: 13348 kJ actual vs 25251 kJ target across 3 days, broken out by meal slot — found that the test workspace's dinner slot was barely logged (162 kJ across all 3 days)
- `diary frequency --days 14` returned top 5 most-frequent foods (banana x6, rye bread x4, reduced-fat cheese x4, yogurt x2, ...). Real, useful data
- `energy balance --days 3` returned daily series with 7-day moving average
- `diary plan-meal --target-protein 30 --remaining-energy 3000` correctly recommended 158 g of the highest-protein-density favorite (after fixing a per-1g vs per-100g units bug found mid-session)
- `bmr --age 38 --weight-kg 80 --height-cm 178 --sex M --activity moderate` returned BMR 1727.5 kcal, TDEE 2678 kcal (Mifflin-St Jeor)
- `weight regression` honestly returned "not enough entries" (the test workspace owner hasn't logged weights in the trailing 30 days)
- `doctor` reports all green except (expected) Auth not-verified-end-to-end note

## Known gaps surfaced
- **achievements**: The `/statistic/analysis/achievements/get` endpoint returns HTML when called without a `type` query param. The endpoint is wired but doesn't have a typed JSON happy path. Not a headline command; user can rerun with `--type=<...>` once the supported types are known.
- **Live writes deferred**: per user's "Quick Check" choice, no live write tests ran. The keystone `diary log` command was verified in dry-run mode (resolved the food, built the payload, returned the JSON envelope with `committed: false`). Live commit was not exercised in this session.
- **Weight regression dependency**: Requires the user to log at least 3 weights within the chosen window. The test workspace had none, so the command surfaced the honest "not enough entries" error rather than a fake-data ghost result.
- **monthWeight bootstrap**: `/statistic/summary/<date>/get`'s `monthWeight[]` is empty for users who haven't recorded weights. `weight regression` depends on this; the command's error message points at it explicitly.

## Fixes applied during Phase 4/5
- **research.json narrative**: command examples used `--by meal` and `auth login --email`; corrected to `--by-meal` and `auth password-login --email`. Also `diary export json` → `diary export-json`. validate-narrative now passes 11/11.
- **diary plan-meal units bug**: favorites API returns macro values per 1 gram; my initial code treated them as per 100 g. Fixed by multiplying both protein and energy by 100 before density math. Now plan-meal recommends sensible portion sizes.
- **dogfood reimplementation_check**: macros gap and weight regression were flagged because dogfood doesn't see `c.Get(...)` literal in those files (they call client through `ktFetchDiaryDay` / `ktFetchSummaryDay` helpers). Added `// pp:client-call` opt-in comments per Phase 3 hand-edit rules.
- **jsonld rate limiter**: Added cliutil.AdaptiveLimiter to `internal/jsonld/FetchDetail` with 429 handling via `*cliutil.RateLimitError`. The package now satisfies the per-source rate-limiting rule.

## Printing Press issues worth a retro
1. **Cookie-auth chrome login wraps cookies in "Bearer "**: the generator's `auth login --chrome` flow saves cookies via `cfg.SaveTokens(...,cookies,...)` which routes them through AccessToken and adds a `Bearer ` prefix on every request. This is wrong for the Cookie header. Worked around in this CLI with a hand-coded `ktSaveConfig` helper that writes to `cfg.AuthHeaderVal` directly. The chrome login path of every cookie-auth CLI may share this defect.
2. **Reserved resource name "search"**: tripped at parse time. The validation message is good (suggests "search_resource"), but documenting the full reserved list in the YAML spec reference would save a regen.
3. **dogfood reimplementation_check pattern match**: looks for a literal `c.Get(...)` /  `c.Post(...)` in the command's file, but novel features that call client through internal helpers get false-positively flagged. The `// pp:client-call` directive works as documented but the doc could call out the helper pattern more clearly.

## Gate: PASS
Proceeding to Phase 5.5 polish.
