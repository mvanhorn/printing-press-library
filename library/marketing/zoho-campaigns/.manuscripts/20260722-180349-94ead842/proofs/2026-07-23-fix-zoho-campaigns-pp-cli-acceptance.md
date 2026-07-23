# Acceptance Report: zoho-campaigns
  Level: Full Dogfood
  Tests: 163/163 passed (run 2; run 1 was 135/165)
  Failures (run 1, all fixed before run 2):
    - 28 commands: HTTP 401 in the sandboxed harness — the documented ZOHO_CAMPAIGNS_* env vars were stored as auth-flow inputs but never wired into the OAuth refresh path, so env-var-only headless auth did not work at all
    - delta/journey error_path: expected non-zero exit for an invalid key/email, but an unknown identifier is indistinguishable from a valid empty local state
  Fixes applied: 2
    - internal/config/zoho_campaigns_env.go (hand-authored) + one-line Load hook: flow-input env credentials now promote to the OAuth client fields; verified live from a sandboxed HOME with only the three env vars set
    - pp:no-error-path-probe annotation on delta and journey (sanctioned opt-out: HTTP-200-empty analog for local-store lookups)
  Printing Press issues (for retro): 3 new this phase
    - Generated oauth2_refresh CLIs declare canonical env vars that cannot authenticate anything (flow-input fields never reach the refresh path) — headless env-var auth is dead code out of the box
    - config_perms_test.go and credentials tests are not hermetic: they read the developer's real credentials.toml (no HOME/USERPROFILE sandbox) and fail on any machine with live credentials
    - (carried) HOME-vs-USERPROFILE sandbox gap, DACL writer/verifier disagreement, unregistered set-token, sync param-default gap, listkey ID extraction
  Notes: write-side endpoints (create/send campaign, subscribe, delete list) were exercised by the matrix's help/dry-run/error paths only; no real sends or mutations were made against the production org. All six novel commands separately verified against the live org during Phase 3/4.95 with real data (redacted here: the test workspace's campaigns, ranked assignees described generically).
  Gate: PASS
