# Shipcheck — pinterest-pp-cli (run 20261021-092559-b7e82418)

## Final
- shipcheck: PASS 7/7 legs (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard)
- verify: 100% (255/255, 0 critical) — before and after
- scorecard: 97 (run1) → 98/100 Grade A (run4)
- go test -count=1 ./... green

## Runs
1. run1 FAIL (dogfood): dead helper printStampedDryRunOutput; examples 3/10 sampled (generator omits Example when required params lack spec examples — 306/375 command files had none); novel depth mismatch `ads report` vs `ad-accounts ads` (dogfood collector ignores addNovelCommandIfAbsent); example.com host in pins preflight help.
2. run2 FAIL (dogfood): only the novel-host check remained (`.example` hosts still count when not in research artifacts).
3. run3 PASS.
4. run4 PASS after live-verified fixes.

## Fixes applied
- Spec overlay: `example` on 83 required parameters (real ad account <ad-account-id> where applicable) → generated Examples; regen restored Task A templated edits from git.
- root.go/ads.go: direct AddCommand for novel `ads` group (behavior-identical; dogfood visibility).
- helpers.go: removed dead printStampedDryRunOutput.
- pins preflight examples use documented brand destination host (documented-urls.txt).

## Behavioral sampling (live, read-only 24h Production Limited token from developer portal; scratch DB/config)
- sync (bare): FOUND BUG — dependents fanned out to 36 resources/22k records; fixed parentFilter so bare sync = boards, pins, sections, boards_pins.
- sync pagination: FOUND BUG — generator does not recognize Pinterest `bookmark` cursor; every list resource was offset/no-cursor and truncated at one page (boards 250, pins 241). Fixed via determinePaginationDefaults wrapper → 271 boards, 22,694 pins, 24,333 board pins, 0 warnings.
- pins audit: 24,808 pins scanned, 11,647 with issues; per-board rollup correct.
- pins preflight: real board name resolved to id; ok=true.
- perf compare: FOUND BUG — metric_types sent exploded (Pinterest honors only comma-joined) → only IMPRESSION non-zero. Fixed (explode=false, matches spec); now IMPRESSION +0.9%, SAVE +6.4%, OUTBOUND_CLICK −5.3%.
- pins funnel: FOUND BUG — all-zero distribution flagged every pin; fixed strict flag + regression test; live 24 pins, sensible rates.
- trends gaps: US trends returned with local coverage counts.
- ads report: FOUND BUG — default column OUTBOUND_CTR rejected for async reports; replaced with CAMPAIGN_NAME,...; live report for <ad-account-id> returned rows.

## Known gaps
- `--all` on generated list commands still uses the generator's offset paginator (no bookmark); pass `--bookmark` manually for page 2+. Same as prior CLI (not a regression). Printing Press retro item: add `bookmark` to cursor-param detection.

## Verdict: ship
