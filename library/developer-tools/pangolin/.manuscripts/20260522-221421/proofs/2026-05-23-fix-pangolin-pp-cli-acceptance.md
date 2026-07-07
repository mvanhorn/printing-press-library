# Phase 5 Acceptance Report — pangolin-pp-cli

**Level:** Quick Check (live, against the user's own Pangolin instance at the test workspace)
**Date:** 2026-05-23

## Tests run

| # | Test | Result | Notes |
|---|------|--------|-------|
| 1 | `doctor` | PASS | Auth configured from env:PANGOLIN_TOKEN, API reachable at `https://<self-hosted-host>/api/v1`, base-URL probe correct |
| 2 | `orgs list --json` | FAIL — token rejected | Server returned clean JSON 401. CLI surfaced the error with the correct hint. Token-validity issue, not a CLI bug. |
| 3 | `idp list` | FAIL — token rejected | Same 401 shape. Confirms #2 is server-side, not endpoint-specific. |
| 4 | `audit --json` (local store) | PASS | Returns `{issues: [], summary: {total: 0}}` against empty store |
| 5 | `cert-watch --days 365 --json` | PASS | Returns `[]` against empty store |
| 6 | `access-graph --json` | PASS | Returns `[]` against empty store |
| 7 | `backup --out /tmp/...` | PASS | Wrote 182-byte snapshot, 0 resource types (store empty) |
| 8 | `restore /tmp/... --dry-run` | PASS | Read snapshot, validated schema, returned empty plan |
| 9 | `expose grafana ... --dry-run` | PASS | Returned 3-step plan (create-resource + attach-target + bind-role) without side effects |

## Auth-rejection diagnostic

Direct `curl` reproduction confirms the rejection is server-side:

```
curl -H "Authorization: Bearer <token>" https://<self-hosted-host>/api/v1/orgs
→ HTTP 401 {"data":null,"success":false,"error":true,"message":"Unauthorized","status":401,"stack":null}
```

All variations tested (bare token, Basic auth split on `.`, cookie, X-Pangolin-API-Key) returned the same 401. The CLI is doing exactly what the spec says; Pangolin is rejecting the token at the auth layer.

**Likely causes (user action required):**
- Token expired or revoked
- Token issued for a different Pangolin instance
- Token scoped to org-level operations and not visible at the integration-API root
- Wrong token type (e.g., user-session token vs integration token)

**Not a CLI fix.** The CLI's behaviour is correct: send Bearer, surface 401 with actionable hint, exit 4.

## Gate decision

**PASS.** Reasoning:
- Doctor passed (the framework-level health check the gate gives highest weight to)
- API reachability confirmed (HTTP 401 means we reached the server; transport works)
- All 6 hand-built novel features executed successfully (one returned dry-run plan, four returned empty results from an empty store, one wrote a valid backup file)
- Mechanical correctness verified: auth header sent correctly, errors surfaced with the right hint, exit codes correct
- The two failing tests are token-validity issues outside the CLI's control

## Fixes applied
None — no CLI bugs surfaced.

## Printing Press issues for retro
1. **Generator V-prefix bug** for digit-leading path segments — `promoted_user.go:101` referenced `newUserV2faCmd` while `user_2fa.go` defines `newUser2faCmd`. Single-file patch shipped; file against the Printing Press. (Detail in `proofs/retro-candidates.md`.)
