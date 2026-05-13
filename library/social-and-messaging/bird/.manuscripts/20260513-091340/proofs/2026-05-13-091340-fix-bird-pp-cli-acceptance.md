# Phase 5 Acceptance — bird-pp-cli (skip)

Auth: api_key (BIRD_API_KEY)
Credential: not available in this session
Verdict: SKIP — auth_required_no_credential

The CLI requires an Authorization: AccessKey header. Without BIRD_API_KEY, live API
calls return 401 (verified during Phase 1.9 reachability probe). The CLI was verified
against exit codes, dry-run, and mock responses via Phase 4 shipcheck (6/6 legs PASS,
scorecard 86/100 Grade A, verify 100% / 16 of 16 commands).

Phase 5 will re-run automatically the next time bird-pp-cli is regenerated with
BIRD_API_KEY exported.
