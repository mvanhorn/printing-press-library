# Phase 5 Acceptance Proof - cloud-run-admin-pp-cli

Run ID: 20260507T171842Z-6844a8b4

## Command

```bash
CLOUD_RUN_ADMIN_OAUTH2C="$(minted from the local Google Ads OAuth refresh-token config with cloud-platform scope)" \
./printing-press dogfood \
  --live \
  --level full \
  --dir ~/printing-press/library/cloud-run-admin \
  --auth-env CLOUD_RUN_ADMIN_OAUTH2C \
  --write-acceptance ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/proofs/phase5-acceptance.json \
  --json
```

JSON proof: `proofs/20260507T174200Z-dogfood-results-full-google-ads-token.json`

## Result

Verdict: PASS

- Level: full
- Matrix size: 67
- Passed: 67
- Failed: 0
- Skipped: 62
- Auth: bearer token minted from the same local Google Ads OAuth client/refresh-token material, requested with `https://www.googleapis.com/auth/cloud-platform`

## Skips

Skipped cases were validator-approved live dogfood skips, primarily commands whose required Cloud Run parent/name positional arguments cannot be safely synthesized without a disposable fixture resource. Destructive-at-auth checks were not enabled.

## Level Selection

Cloud Run credentials were available, so this was not a no-auth skip. The Google Ads OAuth refresh token was able to mint a Cloud Platform-scoped token. `<cloud-run-disabled-project>` rejected Cloud Run Admin calls because the API is disabled or inaccessible there, but `<cloud-run-enabled-project>` has Cloud Run enabled and accepted a live read probe with the same credential source. Full live dogfood passed without `--allow-destructive`.

## Resulting Marker

`proofs/phase5-acceptance.json` records:

- status: pass
- level: full
- tests_passed: 67
- tests_skipped: 62
- api_key_available: true
