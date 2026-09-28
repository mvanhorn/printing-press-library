# NAVITIME shipcheck

Local ship recommendation: **ship**, with the explicitly documented local-install template exception below. All five approved workflows are implemented. No known functional bug remains in the approved scope.

## Results

- Full Go test suite, vet and build pass (`go-checks.json`). Provider: 47 tests/subtests, 25 content assertions; CLI and harness regressions pass.
- Stock shipcheck loop 1: verify 100% PASS; scorecard 83/A; narrative dry-run projection, static flag association, source-challenged workflow and canonical install paragraph failed.
- Fixed dry-run projection and traceable flag registration. Seven full narrative examples pass; all five applicable SKILL correctness checks pass.
- Full live lookup → dated route → stored-detail workflow recovered after cooldown and passes (`workflow-verify-2.json`).
- Stock shipcheck loop 2: verify, narrative, dogfood, workflow-verify, Apify-not-applicable and scorecard all PASS. Only verify-skill's canonical public-install paragraph fails. The raw umbrella remains FAIL/exit1; it is not relabeled green (`shipcheck-2.json`).
- The custom live matrix passes 26/27 cases with 16 GETs including failures. The repeated route --no-cache case remained HTTP202 source-blocked after bounded retries. Four source rejections have 32 verified fail-safe properties; no fabricated success or cache writes. Initial uncached route, cache hit, catalogue refresh, all time modes, POI and JR Pass flows succeeded.

## Local-only instruction exception

`local-install-gate-exception.md` explains why stock public npm/GitHub install instructions are inapplicable to this unpublished local checkout. The user's explicit local-delivery requirement and factual documentation take precedence over this template assumption. The five command/flag/argument/shell checks passed separately (`verify-skill-applicable.txt`); local build/version verification replaces only the inaccurate public-install paragraph requirement. Shared Printing Press tools/configuration remain unchanged. This exception waives no runtime, source, workflow, code-review, receipt or promotion work.

## Source limitations

The undocumented website can return temporary challenges even after a successful anonymous request. Cache/refresh/no-cache semantics are deterministically tested; source errors remain explicit. Route estimates are labeled separately from schedules; displayed fares and pass-holder cost remain distinct. Only JAPAN RAIL PASS has representative live constraint verification; other catalogue entries are source-advertised.

## Tooling observations

The static workflow mapper compares fixture values/absolute temp paths to help text, producing false unmapped-step warnings; actual workflow execution passes. The generic pipeline warning expects SQL sync despite this focused CLI intentionally having no bulk-sync command. Neither warning implies missing approved functionality. Root performed orchestration and acceptance; concrete implementation/tests used gpt-6-sol/max.
