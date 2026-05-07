# Phase 5 Acceptance Proof - cloud-run-admin-pp-cli

Run ID: 20260507T171842Z-6844a8b4

## Command

```bash
CLOUD_RUN_ADMIN_OAUTH2C="$(gcloud auth print-access-token 2>/dev/null)" \
./printing-press dogfood \
  --live \
  --level quick \
  --dir /Users/cathrynlavery/printing-press/library/cloud-run-admin \
  --auth-env CLOUD_RUN_ADMIN_OAUTH2C \
  --write-acceptance /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/proofs/phase5-acceptance.json \
  --json
```

JSON proof: `proofs/20260507T172515Z-dogfood-results.json`

## Result

Verdict: PASS

- Level: quick
- Matrix size: 5
- Passed: 5
- Failed: 0
- Skipped: 4
- Auth: bearer token available from `gcloud auth print-access-token`

## Skips

- `analytics` error-path: skipped because the command has no positional argument.
- `cloud-run-admin-jobs create` happy path and JSON fidelity: skipped because the live dogfood runner cannot safely synthesize a real Cloud Run `parent` without an approved disposable project/location fixture.
- `cloud-run-admin-jobs create` real error-path: skipped for the same non-id parent reason.

## Level Selection

Cloud Run credentials were available, so this was not a no-auth skip. Quick live dogfood was used because the full matrix can exercise write-side Cloud Run APIs and no disposable fixture project was approved for destructive or cost-bearing checks. The acceptance marker was written by `printing-press dogfood` and publish validation accepts it.

## Resulting Marker

`proofs/phase5-acceptance.json` records:

- status: pass
- level: quick
- tests_passed: 5
- tests_skipped: 4
- api_key_available: true
