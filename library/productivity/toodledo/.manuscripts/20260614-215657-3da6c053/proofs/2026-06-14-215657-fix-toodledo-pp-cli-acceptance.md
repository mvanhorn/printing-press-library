# Toodledo CLI — Phase 5 Acceptance (live, read-only)

**Level:** Full read-only (user constraint: no writes to the live account).
**Auth:** OAuth 2.0 v3 — re-authenticated via `auth login --port 9876` (browser authorize). This **validated the hand-coded Basic-auth token-exchange fix end-to-end** against the real Toodledo token endpoint (the code exchange succeeded; the earlier refresh attempt returned `errorCode 102 invalid_refresh_token`, confirming Toodledo *accepted* the Basic-authenticated client and rejected only the expired refresh token).

## Matrix: 18/18 PASS (after 2 fixes)
- `doctor` — Auth configured, API reachable, OAuth2 source. PASS
- `account` — returns the authenticated viewer's profile + Pro tier + sync cursors. PASS
- `sync` — mirrored the whole account: ~613 tasks + folders/contexts/lists/outlines, 0 errored. The `{num,total}` metadata element was correctly skipped on id-extraction. PASS
- `folders/contexts/locations list` — return the real reference data. PASS
- `search "<term>" --type tasks` — offline FTS returns matches. PASS
- `next-actions` — returns exactly the Next-Action (status=1) count; cross-checked against the raw status distribution. PASS
- `review` — buckets match the underlying status counts (waiting/someday) and surface real stalled projects. PASS
- `dashboard` — correct status/priority/folder breakdown over real data. PASS
- `stalled-projects` — surfaces real reference folders with open tasks but no Next Action. PASS
- `goal-progress` — returns empty (the account has no goals); verified correct, not a failure. PASS
- `sync-cost` — forecasts call cost against the 100-call budget (authed). PASS
- `--agent --select` (incl. the documented `review --agent --select` recipe) — field narrowing works on nested envelopes. PASS
- `tasks list --num N` — returns N real tasks. PASS

## Bugs found live and fixed (CLI fixes)
1. **sync omitted the `fields` selector** (critical): the syncer never applied the endpoint's `fields` default, so Toodledo returned only id/title/modified/completed — every optional GTD field (folder, context, status, priority, duedate, star) was 0/null, breaking next-actions, review, dashboard, and stalled-projects. Fixed by injecting per-resource default params (`syncResourceDefaultParams`) into the sync loop. After the fix, status/folder/priority populate correctly and all GTD commands return correct results.
2. **`tasks list` live response included the `{num,total}` metadata element** as the first "task." Fixed with `stripTaskMetaElement` on the live read path.

## Printing Press retro candidates (machine gaps surfaced)
- The syncer should apply endpoint param `default` values (not only pagination/since), or support a spec-level "always-send" param. Toodledo's `fields` is essential and was silently dropped.
- Live reads of metadata-first array endpoints need a generic strip/`response_path`-style mechanism.
- OAuth2 authorization_code should support `client_secret_basic` natively (RFC 6749 default) — the generator emits form-encoded creds only.

## Gate: PASS
PII handling: no real task titles, folder contents, emails, or account identifiers recorded in this report (described generically). Token values never written to any artifact.
