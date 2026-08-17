# nutrition-pp-cli Shipcheck

## Final result (with real FDC_API_KEY)
- shipcheck: **PASS (7/7 legs)** — verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard all exit 0.
- scorecard: **92/100 Grade A**. Sample Output Probe: **7/7 (100%)**.
- Live dogfood (Phase 5) full matrix: **83/83 PASS, status: pass**.

## Live behavioral verification (real USDA key)
All 7 novel commands exercised against the live USDA API + NutritionValue.org:
- enrich 173414 → net_carbs=4.45, omega_6_3_ratio=6.2, nv_matched=true
- rank protein → Soy protein isolate 88.3g top (descending correct)
- compare 2646170 173414 --basis 100kcal → chicken 21.24 g protein/100kcal (highest), calories present
- find --min protein=25 --max-kcal 175 → matches satisfy thresholds, Foundation foods included
- meal 173414:50g 1105314:120g → 2 items, 318 kcal, 0 failures
- cite 173414 → correct APA + MLA with year 2019
- log add 2646170 --grams 200 → 212 kcal / 45g protein; progress 11%/30% vs targets

## Bugs found in live testing and fixed (would have shipped broken without a real key)
1. USDA batch dropped all but first food (comma %2C-encoded) — fixed to repeated fdcIds params. Broke meal AND compare.
2. Foundation foods report energy via Atwater 957/958, not 208 — Calories() now falls back. Broke protein-density, find --max-kcal, meal, log for a huge share of the DB.
3. cite year parse on M/D/YYYY → "4/1/"; fixed to handle both date formats.
4. enrich NV miss on program-note descriptions; fixed by searching NV on core name.
5. compare now reports missing_ids when USDA batch omits an id.
6. log-mutation dry-run emitted plain text under --json; fixed with JSON-aware emitDryRun.
7. USDA spec-serving meta-endpoints (json-spec/yaml-spec) unregistered (returned non-JSON).

## Ship recommendation: ship
No known functional bugs in shipping-scope features. All 22 features (15 absorbed + 7 novel) work.

## Known non-blocking item (retro candidate, generator bug)
Dual x-auth-env-vars emit colliding `toml:"api_key"` tags, failing 5 generated cliutil credential-round-trip tests. Confirmed pre-existing on a pristine generated tree. Env-var auth (documented path) verified working live. Does not gate shipcheck. See phase-4.95-findings.md.
