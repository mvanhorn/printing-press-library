# Email Bison CLI — Acceptance Report

  Level: Live dogfood SKIPPED (auth required, no credential)
  Reason: Email Bison uses bearer-token auth and the user declined to provide a key/base URL. Per Phase 5 rules, live dogfood auto-skips when the API requires auth and no key is available. Marker: phase5-skip.json (status: skip, skip_reason: auth_required_no_credential).

## What was verified instead
- Shipcheck (6/6 legs PASS): verify, validate-narrative, dogfood (structural), workflow-verify, verify-skill, scorecard 89/100 Grade A. Live sample probe 7/7 (100%).
- Behavioral correctness of all 7 novel features confirmed against a seeded local SQLite fixture (Phase 3 build log): headroom, sender-emails health, replies interested, replies triage, leads stale, campaigns variants, campaigns preflight all computed correct joins (not just empty-safe).
- Empty-store behavior: every read command returns valid `[]` / structured JSON and exit 0.
- Agentic reviews (Phase 4.8 / 4.9 / 4.85): SKILL semantics, README/SKILL correctness, and output plausibility all resolved; residual `senders health` -> `sender-emails health` references fixed in prose/recipes/env-table.

  Fixes applied: 4
    - verify-skill: campaigns resume recipe -> `campaigns resume campaign 6`
    - README/SKILL prose + recipe: `senders health` -> `sender-emails health` (3 spots)
    - README env-var table: added EMAIL_BISON_BASE_URL row

  Gate: SKIP (valid auth-aware skip; not a failure)

## To exercise live later
Set `EMAIL_BISON_BASE_URL` (your dedicated instance) and `EMAIL_BISON_API_KEY` (workspace api-user token), then run `email-bison-pp-cli doctor`, `sync`, and the novel commands against real data.
