# zimmo-pp-cli shipcheck (2026-09-23)

Run 1: 6/7 legs (validate-narrative FAIL on `watch run` recipe path; verify WARN 98% mock mode; dogfood WARN: 5 generated dead helpers + internal/zimmo/token.go outbound HTTP without limiter/429 typing). Scorecard 85/100 A.
Fixes: token mint now paced by an AdaptiveLimiter and returns *cliutil.RateLimitError on 429; binary rebuilt so the narrative leg saw `watch run`.
Run 2: 7/7 PASS (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard). Scorecard 85/100 Grade A; live sample probe 6/6.

Verify pass rate: 98% → 98% (mock mode; exec failures on live-only commands are expected without network in mock).
Remaining scorecard gaps: dead_code 1/5 (generator-owned helpers handleBinaryResponseDelivery, readSecretFromStdin, retainCLIQueryParams, retainExplicitQueryParams, successfulNoop), cache_freshness 5/10, path_validity 5/10 (doctor probes "/" — OpenAPI has no health_check_path extension).

Behavioral sample of every novel command (live, 2026-09-23, store with 1030/1050/1060):
- peb-trap: G listings first, suspect kWh (27 516) flagged and sorted last.
- underpriced: 530 checked, viager LRPL8 skipped; top rows are large houses to renovate (condition shown).
- motivated: LL2JD 217 days + 24.2% cut + EPC G; building siblings no longer counted as re-listings.
- comps LRWGW: radius rooftop-only, widened to postcode 1050; Zimmo lists few sold comps → explicit note.
- yield --all 1060: similar-surface rental comps (13-22), no extrapolation to 500 m² houses.
- same-as --all: 642 checked, 385 matched against the real immoweb/immovlan stores, 257 Zimmo-only.
- enrich: only rows not refreshed within --stale (2 geocoded).

Verdict: ship
