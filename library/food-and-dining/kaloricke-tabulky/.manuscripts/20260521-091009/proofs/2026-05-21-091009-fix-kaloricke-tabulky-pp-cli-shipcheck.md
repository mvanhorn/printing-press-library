# Shipcheck Report: kaloricke-tabulky-pp-cli

## Commands run and verdicts

| Leg | Verdict | Notes |
|---|---|---|
| verify | PASS | 39/39 commands respond cleanly under verify-mode |
| validate-narrative | PASS | 11/11 narrative commands resolve and full-example dry-runs succeed |
| dogfood | PASS | After fixes: 0 dead flags, 0 dead helpers, 10/10 novel features built, 0 reimplementation issues, 0 source-client issues |
| workflow-verify | PASS (skip) | No workflow_verify.yaml authored |
| verify-skill | PASS | 0 errors, 1 likely-false-positive warning |
| scorecard | PASS | 81/100 Grade A (after polish, up from 79/100 Grade B initial) |

## Top blockers found and fixed

1. **validate-narrative FAIL → PASS**: Narrative referenced `auth login --email` (the hand-coded command is named `auth password-login --email`), `--by meal` flag (actual is `--by-meal`), `diary export json` with a space (actual is `diary export-json`), and `summary --json` without the required `<date>` positional. Fixed `research.json` and re-validated.

2. **dogfood reimplementation_check WARN → PASS**: `macros gap` and `weight regression` were flagged because they call the client through internal helpers (`ktFetchDiaryDay`, `ktFetchSummaryDay`) instead of literal `c.Get(...)`. Added `// pp:client-call` opt-in comments to both.

3. **dogfood source_client_check WARN → PASS**: `internal/jsonld/jsonld.go` made outbound HTTP without rate-limit handling. Added a package-level `*cliutil.AdaptiveLimiter` (4 req/s floor, ramps on 429) and a typed `*cliutil.RateLimitError` return path.

4. **dogfood dead code WARN → PASS**: Polish skill removed dead flag `--allow-partial-failure` and dead helpers `detectPartialFailure` / `partialFailureErr` (Google-Ads-style boilerplate not used by this API).

5. **MCP tools-audit (1 finding) → 0 findings**: Polish skill annotated the `stats` parent command with `mcp:read-only` (subcommands already carried it).

6. **PII-audit (1 finding) → 0 findings**: Polish skill replaced `<your-email>` placeholder in `kt_auth_password_login.go` with the non-matching `<your-email>`.

## Before/after metrics

| Metric | Before | After |
|---|---|---|
| Verify pass rate | 100% (all spec-emitted) | 100% |
| Scorecard total | 79 (Grade B) | 81 (Grade A) |
| dogfood issues | 4 (1 dead flag, 2 dead helpers, 2 reimpl, 1 source) | 0 |
| validate-narrative pass | 0/0 (failing, exit 1) | 11/11 PASS |
| MCP tool-audit findings | 1 | 0 |
| PII-audit findings | 1 | 0 |

## Known Gaps (documented but not fix-now)

These were surfaced by Phase 4.85 / 5.5 polish output review as real implementation bugs but are explicitly **feature work beyond polish/shipcheck scope** and remain in the shipped CLI's `## Known Gaps` section:

1. **`food allergens` premise is incorrect.** The JSON-LD `keywords` array on `kaloricketabulky.cz` food pages contains nutritional fact strings (Energetická hodnota, Bílkoviny, etc.), not allergen tokens. Every food currently returns `count: 0` from `food allergens`. Fixing requires re-sourcing allergen data (parse the HTML ingredient list, or query a separate endpoint). The transcendence feature ships, but with the caveat that it won't currently surface useful data.

2. **`food allergens` and `food get` slug resolution can land on unrelated foreign-language foods.** Example: `food allergens chleba` returns title "Pehmeät kaurapalat Vaasan" (a Finnish oat product) because the slug fuzzy-matches that. Fixing requires adding a name-match validation or routing through a Czech-biased search endpoint first.

3. **`weight regression` requires user-logged weight data.** If the user hasn't logged weights in the window, the command honestly returns "not enough entries (have N, need at least 3)" rather than a 0-slope ghost result. Honest behavior, but a fresh user with no logged weights will see an error rather than a regression — by design, but worth surfacing.

4. **Diary write endpoints other than `diary log` (e.g. `diary food remove`, `diary note add`, `diary copy`, `food create`, `meal create`, `favorite add/remove`, `diary export pdf|xls`) ship as scaffolded commands without full body-shape verification.** Their parent commands and `--help` are correct; their POST bodies are derived from the AngularJS controller bundle and may need form-tuning the first time you use them. The keystone shipping write (`diary log`) was dry-run verified end-to-end; live commit was not exercised in this session.

5. **Generator-level cookie-auth defect (worth a retro).** The Printing Press's emitted `auth login --chrome` flow saves cookies via `cfg.SaveTokens(..., cookies, ..., zero)` which routes them through `AccessToken` and prepends `Bearer ` on every request — wrong for a Cookie header. Worked around in this CLI with a hand-coded `ktSaveConfig` helper that writes to `cfg.AuthHeaderVal` directly. The `--chrome` login path of every cookie-auth CLI may share this defect.

## Final ship recommendation

**`ship-with-gaps`** — the CLI is fully functional for its headline features (login, food search + nutrition lookup, diary read, macro analytics, weight regression, BMR). The 5 known gaps above are documented and don't block usage of the day-to-day commands. The user's stated value ("a working CLI they can use day-to-day") is delivered.

The two output-review warnings (allergens premise, slug resolution) are real bugs but are scoped as feature work. They're recorded in this report and the printed CLI's `## Known Gaps` section so the user is forewarned, but they don't downgrade to `hold`.
