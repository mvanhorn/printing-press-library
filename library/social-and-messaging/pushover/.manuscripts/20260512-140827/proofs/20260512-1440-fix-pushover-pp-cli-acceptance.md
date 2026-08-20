Acceptance Report: pushover
Level: Full Dogfood
Tests: 112/112 passed, 104 skipped by the runner
Gate: PASS

Live checks performed:
- `doctor --json` validated parameter credentials and API reachability.
- `quota --json`, `users validate --json`, and `sounds --json` returned successful live API responses.
- `notify` sent one low-priority live test notification; the recipient confirmed receipt in the session.
- `history` recorded the live send in a local redacted ledger with no raw token or user key stored.
- The official `printing-press dogfood --live --level full` runner passed and wrote `phase5-acceptance.json`.

Fixes applied during Phase 5:
- Replaced example placeholder credential values with env credentials when `PUSHOVER_APP_TOKEN` / `PUSHOVER_USER_KEY` are present.
- Added missing Cobra examples for hand-built Pushover workflow commands.
- Reordered `emergency watch` examples so the fixture-free dogfood path uses dry-run.
- Rebuilt the working binary before rerunning live dogfood.

Printing Press issues:
- The live dogfood runner uses Cobra examples literally. For parameter-auth APIs, placeholder examples can override valid env credentials unless the CLI compensates.

No raw credentials or recipient keys are included in this report.
