# Food52 Acceptance Report (Live Dogfood)

**Run:** 20260501-164925 (reprint, v3.2.1)
**Level:** Full Dogfood
**Tests:** 67/67 PASS
**Gate:** PASS

## Test Matrix

The full mechanical test matrix exercised every Food52-specific leaf subcommand against the live food52.com SSR + Typesense backend, plus operational commands (doctor, version, agent-context). For each domain leaf:

- **Help check** — `--help` exits 0 and prints an Examples section
- **Happy path** — one realistic invocation with `--json`, exits 0
- **JSON fidelity** — output parses as valid JSON
- **Error path** — invocation with missing/invalid arg exits non-zero (where applicable)

Pipe-free exit-code checks were used throughout (`> file 2>&1; echo $?`), avoiding the pipe-eats-exit-code bug.

### Coverage

| Surface | Help | Happy | JSON | Error |
|---|---|---|---|---|
| recipes browse/get/search/top | 4/4 | 4/4 | 4/4 | 3/3 |
| articles browse/browse-sub/for-recipe/get | 4/4 | 4/4 | 4/4 | 1/1 |
| pantry add/list/match/remove | 4/4 | 4/4 | 2/2 | 1/1 |
| sync recipes/articles | 2/2 | 2/2 | 2/2 | — |
| tags list | 1/1 | 1/1 | 1/1 | — |
| search (local FTS) | 1/1 | 1/1 | 1/1 | 1/1 |
| scale | 1/1 | 1/1 | 1/1 | 2/2 |
| print | 1/1 | 1/1 | — | — |
| open | 1/1 | 1/1 | — | 1/1 |
| Operational (doctor/version/agent-context) | 3/3 | 3/3 | 1/1 | — |
| **Total** | **22/22** | **22/22** | **17/17** | **8/8** |

### Behavioral correctness samples

- `recipes search "brownies" --limit 3 --json` returned 175 hits, top is "Lunch Lady Brownies" (test_kitchen_approved).
- `recipes browse chicken --json` returned 24 of 760 chicken recipes, structured Sanity JSON.
- `recipes get sarah-fennel-...` returned full recipe (16.5 KB JSON, all fields).
- `recipes top chicken --tk-only --limit 3 --json` returned 3 TK-approved chicken recipes.
- `articles browse food --json` returned 24 articles (52 KB).
- `articles browse-sub food baking --json` returned 14 KB of baking articles.
- `articles get best-mothers-day-gift-ideas --json` returned the article with full body.
- `tags list --json` returned 4 tag families.
- `sync recipes vegetarian --limit 5` pulled 5 summaries + 5 details into the local store.
- `sync articles food --limit 3` pulled 3 article summaries.
- `search vegetarian --json` found vegetarian recipes in the local store via FTS5.
- `pantry add tomato chicken && pantry list && pantry match` round-trip works.
- `scale mom-s-japanese-curry-chicken-with-radish-and-cauliflower --servings 8` parses yield, scales every ingredient by factor 2.
- `print sarah-fennel-...` renders title + numbered ingredients + numbered steps, no nav/ads.
- `open sarah-fennel-...` prints canonical URL (no launch by default — side-effect convention).
- `doctor`: api reachable, auth not required.

## Failures

None. 67/67 tests passed.

## Fix applied during dogfood

**`scale` validation tightened.** Before: `scale <slug>` (without `--servings`) silently returned help text with exit 0. After: returns `Error: scale requires --servings N (positive integer); got 0` with exit 1. The verify-friendly RunE pattern (return help on `len(args) == 0`) was kept for the bare `scale --help` case. Single-file edit to `internal/cli/scale.go` lines 36-41.

This was a real UX bug — a user who provided a slug but forgot `--servings` got no actionable feedback. Caught during error-path testing in this run; not in scope for v2.3.10's prior dogfood.

## Pass-through warnings (Wave B, no shipcheck blocking)

Recorded in `phase-4.85-findings.md`. Both root-caused upstream:

- `scale` ingredient string `"4 teaspoons kosher"` is missing the trailing word "salt" — Food52's Sanity CMS itself ships the data without "salt".
- `scale` ingredient line `"4 small Yukon Gold potatoes (about 10 ounces), cut into 1 by ½-inch pieces 7 ounces cauliflower, cut into bite-size florets"` — Food52 CMS pasted two ingredients into one field without separator. The CLI extracts what's there.

Neither is a CLI bug. The site itself displays the same text. No fix appropriate at the CLI layer.

## Printing Press improvements (for retro)

- **F7 (verify mock-mode false positives) still unfixed.** The `print FAIL`, `which FAIL` entries in shipcheck verify EXEC come from required-positional commands being called with no positional. Live dogfood passes both. The verify mock-mode dispatcher still doesn't consult positional-arg requirements before invoking. This is the same finding as the prior reprint's retro F7.

- **Traffic-analysis schema drift between v2 and v3.** Prior `traffic-analysis.json` required adapting four fields to load under v3.2.1: `protections[].notes` is now `[]string` (was string), `generation_hints` is `[]string` derived from a fixed vocabulary (was a `{key: bool}` map), `warnings` are `AnalysisWarning` objects (were strings), `auth.candidates` replaced `auth.candidate_types`, version is `"1"` (was `"1.0"`). This silent drift means every browser-sniffed CLI in the library that hand-authored a v2 traffic-analysis will fail to load on v3 generation without manual conversion. Generator could detect old shapes and migrate them, or document the migration recipe.

## Gate

**PASS.** All ship-threshold conditions met:
- shipcheck umbrella exits 0 (5/5 legs)
- verify 94% pass rate, 0 critical
- scorecard 84/100 Grade A
- workflow-verify workflow-pass
- verify-skill exits 0
- Live dogfood 67/67
- No known functional bugs in shipping-scope features
- 1 in-session bug fix applied (scale validation), no deferrals
