# Phase 4.95 Local Code Review — findings

## Autofix summary
8 hand-written-code findings from the reviewer; 7 fixed in-place (see below). All in-scope files.

## Fixed in-place (hand-written files)
1. NV omega regex missing (?s) — could fail if HTML spans lines (client.go).
2. Search-row slug↔id zip could misalign — rewrote to bounded per-row forward-search (client.go parseSearchRows).
3. Branded s-prefix ids never match numeric fdcId → added name-overlap guard (>=0.5) before accepting a fallback NV row (client.go FoodByID).
4. Normalize didn't read /foods/search shape (nutrientNumber/nutrientName/value) — added (nutridata.go).
5. Branded labelNutrients with non-gram serving stored per-serving as per-100g — now skipped to avoid mis-scale (nutridata.go).
6. Duplicate firstNonEmpty arg (nutridata.go).
7. round2 rounded negatives toward zero — switched to math.Round (nutrition_shared.go).
8. find --limit 0 misleading note — validate --limit>0 (find.go).

## Live-testing bugs (found in Phase 5 with a real FDC key; fixed)
- **fetchUSDAFoods batch dropped all but the first food**: USDA /v1/foods needs REPEATED fdcIds params; the client comma-joined and URL-encoded the comma to %2C, so the API returned only the first food. Broke `meal` AND `compare`. Fixed by building url.Values with one fdcIds param per id (nutrition_shared.go).
- **cite year parse**: USDA publicationDate is "M/D/YYYY"; took first 4 chars → "4/1/". Fixed citationYear to handle both M/D/YYYY and YYYY-MM-DD (cite.go).
- **enrich NV miss on program-note descriptions**: USDA "Cheese, cheddar (Includes foods for USDA's Food Distribution Program)" found no NV match. Fixed by searching NV on the core name before the first "(" (client.go searchQuery).

## Retro candidate (generator bug — NOT patched; generator-reserved)
- **Dual x-auth-env-vars produce colliding `toml:"api_key"` tags.** A spec with two canonical auth env vars (here FDC_API_KEY + USDA_API_KEY) emits `FdcApiKey` and `UsdaApiKey` both tagged `toml:"api_key"` in internal/config/config.go. This breaks the persisted-credential save/load round-trip (`auth set-token` → reload), failing generated cliutil tests TestAuthWriteMigratesLegacyConfigToCredentialsOnly, TestConcurrentCredentialWritersLeaveParseableCredentials, and 3 related. CONFIRMED on a pristine generated tree (not a hand-edit regression). Env-var auth (the documented path) is unaffected and verified working live. Does not gate shipcheck. File against the generator: multi-env-var apiKey auth should emit distinct toml keys (or a single field with the primary env var canonical).

## Additional live-testing fixes (Phase 5, with real key)
- Atwater energy fallback (957/958) in nutridata.Calories() — critical, affected all Foundation foods.
- find --max-kcal now resolves energy via Calories() (was raw Amount("208"), missed Foundation).
- compare reports missing_ids for USDA batch omissions.
- log-mutation dry-run emits JSON under --json (emitDryRun helper); applied to all novel commands.
- json-spec/yaml-spec meta-endpoints unregistered (retro candidate: generator should not promote spec-serving endpoints).
