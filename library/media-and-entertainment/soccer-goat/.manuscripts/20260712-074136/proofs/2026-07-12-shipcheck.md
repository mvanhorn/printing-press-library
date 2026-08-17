# soccer-goat shipcheck

Verdict: PASS (7/7 legs)
- verify PASS, validate-narrative PASS, dogfood PASS, workflow-verify PASS, apify-audit PASS, verify-skill PASS, scorecard PASS
- Scorecard: 91/100 Grade A
- Sample probe: 6/6 (100%)

## Fixes applied during shipcheck
1. Potential client skips network when no clearance cookie set (team commands 19.9s -> 3.7s; honest best-effort).
2. EA limiter 3->8 req/sec (30-player squad enrichment under probe budget).
3. Narrative: removed nonexistent `auth login --chrome` reference; wonderkids example now includes --team; TM base URL env corrected to SOCCER_GOAT_BASE_URL.

## Known soft gap (non-blocking)
- Scorecard insight dimension 4/10 (learning/insight commands) - not a shipping-scope feature.

## Environmental note (not a code defect)
- Local Go toolchain is 1.26.4; govulncheck flags GO-2026-5856 (crypto/tls, fixed in 1.26.5). Affects `generate --validate` only; build/verify/tests all pass. Publish would want Go 1.26.5+.

Verdict: ship
