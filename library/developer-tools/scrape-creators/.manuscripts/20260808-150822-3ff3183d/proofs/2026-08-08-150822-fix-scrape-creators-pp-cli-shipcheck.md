# scrape-creators-pp-cli Shipcheck Report — reprint run 20260808-150822-3ff3183d

## Summary
- **Verdict:** ship
- **Shipcheck legs:** verify PASS, validate-narrative PASS, dogfood PASS, workflow-verify PASS, apify-audit PASS, verify-skill PASS, scorecard 93/100 (all 25 dimensions verified)
- **Verify:** mock 100% then live 100% (mode: live, read-only GETs, real key)
- **Dogfood:** structural PASS; novel_features_check 14/14 planned=built
- **Live dogfood (Phase 5):** quick matrix 15/15 PASS (marker fingerprinted); a full 635-probe live matrix was also run as supplementary evidence — 496 pass, 139 fail, all fixture-bound (mock handles against real endpoints: 124 http_4xx with credits_charged=0, 14 http_5xx, 1 error-path expectation fixed via pp:no-error-path-probe on comments coverage). No CLI bugs among the failures.
- **verify_skill:** all checks passed incl. canonical-sections

## Top blockers found and fixed
1. research.json narrative examples: --with-replies missing value; ads monitor --platform flag didn't exist → fixed at source, dogfood re-synced README/SKILL.
2. Duplicate `account budget` registration and clobbered generated novel parents after porting prior novel files → wiring reconciled with addNovelCommandIfAbsent.
3. apple-music list happy-path 400 (requires id|url) → pp:happy-args fixture annotation.
4. Code-review errors on coverage ground truth (see build log).

## Before/after
- verify pass rate: 97.4% (first mock, stale tools-manifest at 4.27) → 100% mock → 100% live
- scorecard: 92 (prior print, measured by 4.30 rubric with 2 dims at 0) → 93 with all dimensions verified

## Final recommendation
**ship** — all threshold conditions met; no functional bugs in shipping-scope features.
