# Phase 5 Acceptance — hubspot-pp-cli

## Status: SKIPPED (no API key in env at Phase 5 gate)

## Reason
User stated intent in Phase 0.5 to `export HUBSPOT_TOKEN` before Phase 5, but the env var was missing when Phase 5 fired. Per skill auto-skip rule (bearer_token auth + no credential = skip), live dogfood was not run.

## What's verified
The CLI passed all non-live verification:
- 6/6 shipcheck legs (dogfood, verify, workflow-verify, verify-skill, validate-narrative, scorecard)
- 100% mock-mode verify (37/37 commands, HELP+DRY-RUN+EXEC all pass)
- Scorecard 83/100 Grade A
- 9/9 transcendence commands present + dry-runnable

## What's not verified
- Real auth handshake with `Authorization: Bearer <token>` against api.hubapi.com
- Real sync of contacts/deals/companies/engagements into local SQLite
- Behavioral correctness of the 9 transcendence commands against synced data
- Rate-limit adaptive backoff under real 429s

## How to run later
```bash
export HUBSPOT_TOKEN=<your private app token>
cd ~/printing-press/library/hubspot
./hubspot-pp-cli doctor
./hubspot-pp-cli sync
printing-press dogfood --live --dir . --level full --json \
  --write-acceptance proofs/phase5-acceptance.json
```

## Gate
Per skill, the JSON gate marker is at `proofs/phase5-skip.json` with `status: skip`, `skip_reason: auth_required_no_credential`. Phase 5.6 allows the promote step to proceed when a valid skip marker exists.
