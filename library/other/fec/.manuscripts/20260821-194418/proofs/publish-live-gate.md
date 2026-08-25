# Publish Live Gate — fec-pp-cli (run 20260821-194418)

Gate question: was the final package re-validated after the last content change, before publication?

## Timeline
1. Generated from official FEC Swagger (patched locally: deduped securityDefinitions, host/schemes set). 2026-08-21
2. `go mod tidy` + build + vet clean.
3. Shipcheck first run: scorecard FAIL (spec referenced undefined scheme "apiKey" in global security).
4. Fixed `spec.json` global security to `[{"ApiKeyQueryAuth": []}]`. Shipcheck rerun: **PASS 7/7** (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard).
5. Full dogfood matrix executed against live api.open.fec.gov with a registered key: **35 PASS / 0 FAIL** (8 initial harness misfires were flag-name corrections on the test script, re-run clean; no product defects).
6. `cli-printing-press verify --dir .`: PASS 54/54, 0 critical.
7. Secret scan of final tree for the user's API key literal: zero hits.

## Final state at gate close
- Branch content = library/other/fec as committed; nothing regenerated after step 6.
- Verdict: **GO** for publication PR.

Evidence: proofs/dogfood-matrix-output.txt, proofs/reachability.json, proofs/phase5-acceptance.json, dogfood-results.json (CLI root)
