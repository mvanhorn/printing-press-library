# Acceptance Report: nutrition-pp-cli

Level: Full Dogfood (live, real FDC_API_KEY)
Tests: 83/83 passed (binary-owned live matrix)
Gate: PASS

Auth context: USDA api_key (real key provided for testing; DEMO_KEY fallback for keyless use). NutritionValue.org: no auth.
Failures: none.
Fixes applied during Phase 5: 7 live-testing bugs (see shipcheck report).
Printing Press issues (retro): 2 — (1) dual x-auth-env-vars colliding toml tags; (2) spec-serving meta-endpoints promoted to commands.

PII: no organization/user PII in outputs (public food-database data only).
