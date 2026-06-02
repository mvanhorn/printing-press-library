# Sybill CLI — Acceptance Report

Level: Skipped (live) — Sybill requires a Bearer key; user declined to provide one.
Gate: N/A (auth-required-no-credential skip; marker phase5-skip.json written)

## Coverage achieved without live API
- Structural: shipcheck PASS 6/6 (verify, validate-narrative, dogfood, workflow-verify, verify-skill, scorecard).
- Scorecard: 92/100 Grade A.
- Behavioral: 7 Go acceptance tests (internal/cli/novel_sybill_test.go, webhook_test.go) against a synthetic store, asserting output content for all 6 novel features + Svix verify. All PASS.
- Sample live probe (shipcheck): 5/6 novel features clean against an empty store; crm-autofill --deal correctly returned a 403 auth error without a key (expected).

## Not exercised (needs a key)
- Live list/get/sync against real Sybill data.
- Live crm-autofill detail fetch.
Recommend a Quick Check (doctor, sync, deals dark, digest) once a SYBILL_API_KEY is available.
