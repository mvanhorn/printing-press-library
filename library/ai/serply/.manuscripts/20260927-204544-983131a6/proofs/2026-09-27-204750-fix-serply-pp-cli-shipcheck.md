# serply-pp-cli shipcheck

Command: `cli-printing-press shipcheck --dir $CLI_WORK_DIR --spec serply-openapi.yaml --research-dir $API_RUN_DIR` (defaults: verify --fix, validate-narrative --strict --full-examples, scorecard --live-check).

## Legs (final run)

| Leg | Result |
|---|---|
| verify | PASS, 100% (37/37, 0 critical), mock mode |
| validate-narrative | PASS, 9 narrative commands resolved, full examples passed |
| dogfood | PASS (WARN: 4 dead generated helpers) |
| workflow-verify | PASS (workflow-pass, no manifest) |
| apify-audit | PASS (no actor references) |
| verify-skill | PASS |
| scorecard | PASS, 91/100 Grade A; live sample probe 2/2 passed, 1 skipped |

Verdict: PASS (7/7 legs passed), two runs.

## Top blockers found

- None blocked the umbrella. Behavioral sample of every novel command (live, num <= 5) found one bug: `/v1/news` ignores `num` upstream (returned 71 entries for num=2), so `research` over-counted news sources.

## Fixes applied

- `fetchVertical` now caps results at `--num` client-side (internal/cli/serp_common.go). Rechecked live: news count 2 for --num 2.
- Before the umbrella: Example lines for 7 promoted commands (see build log).

## Before/after

- verify pass rate: 100% -> 100%
- scorecard: 91 -> 91 (Grade A). Lowest dimensions: Dead Code 2/5 (generator helpers), Cache Freshness 5/10 (disabled on purpose: paid stateless search), Auth Protocol 8/10.

## Behavioral samples (live)

- `rank github.com --q "open source cli" --num 5`: found, position 3, exit 0.
- `research "retrieval augmented generation evaluation" --num 2`: web/news/scholar sources numbered and deduplicated.
- `serp diff --q "serp api" --num 5`: first run stores a baseline, first_run=true.

Final ship recommendation: ship
