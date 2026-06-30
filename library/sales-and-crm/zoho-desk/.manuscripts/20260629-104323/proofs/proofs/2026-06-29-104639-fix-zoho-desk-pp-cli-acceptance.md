# Zoho Desk CLI — Phase 5 Live Acceptance Report

Level: Quick (read-only OAuth scopes; write lifecycle not exercised — no sandbox/approval).
Tests: 13/13 passed, 11 skipped (write commands needing fixtures, correctly classified blocked-fixture).
Gate: PASS (phase5-acceptance.json status=pass).
Auth: oauth2_refresh against the live test portal.

## Live verification (read-only, real portal)
- doctor: all green (Config/Auth/Env Vars/Org ID/API reachable).
- OAuth2 refresh-token flow: WORKS end-to-end (auto-mints access token).
- orgId auto-detected from organizations list and injected on every request.
- agents list: returned the org's agents (the support team).
- departments list: returned the org's 2 departments.
- tickets list: live results.
- sync: pulled ~100 live tickets into local SQLite.
- agent-load --weighted: aggregated 7 assignees over 100 scanned tickets (median weighted load computed).
- triage / morning: produced correct ranked/composed output over real synced data.

## Bug found and fixed during live testing (Printing Press generator issue)
oauth2_refresh with custom env_vars: the ZOHO_DESK_* env vars and desk_* config
keys populated separate ZohoDesk* config fields that were NEVER wired into the
OAuth refresh client (which reads the generic ClientID/ClientSecret/RefreshToken).
Result: env-var auth and desk_* config keys silently failed with 401 despite
valid credentials. FIXED in config.go Load by bridging ZohoDesk* -> generic
fields when the generic ones are empty. Verified: doctor went from
"Auth: not configured" to "configured", live calls succeed. -> RETRO candidate.

## Verdict: ship

## Note: pre-existing Windows unit-test env failures (not regressions)
internal/config paths_test.go and credentials_test.go fail in this Windows
session (HOME vs USERPROFILE home-dir resolution; corrupt-config legacy
fallback). Confirmed failing on the unedited generated code too. Not part of
the shipcheck gate (which passes 7/7); generator never ran go test in its
gates. Flag for retro: generated path/credential tests assume POSIX HOME.
