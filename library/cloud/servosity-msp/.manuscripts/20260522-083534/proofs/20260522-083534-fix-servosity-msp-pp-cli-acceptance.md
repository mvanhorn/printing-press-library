# Phase 5 Acceptance — servosity-msp-pp-cli (Full Dogfood)

**Level:** Full Dogfood (against a real partner-scoped token, tenant: reseller 2)
**Verdict:** PASS — 12/12 tests
**Token disposition:** Token file `/tmp/servosity-token` deleted after run.

## Headline

The CLI works against the live Servosity API after eight in-session fixes. The most important: a generator-level auth-header bug (missing Django REST Framework `Token ` prefix) and a spec-coverage-gap discovery (`/issues/`, `/reports/stale-backup-sets/` are admin-only — partner-scoped tokens must use the `/resellers/{id}/...` variants). Both surface only in dogfood, not in shipcheck.

## Live verification

| Command | Result | Notes |
|---|---|---|
| `doctor` | ✅ | auth working, base_url correct |
| `current-user list` | ✅ | returns username/email; does NOT expose reseller_id (so we resolve via /companies/) |
| `companies list` | ✅ | 14 companies returned |
| `sync` | ✅ | 263 records synced across 20 resources; 21 warnings for admin-only endpoints (expected) |
| `attention --refresh --since 365d` | ✅ | valid JSON envelope; stale-backup section gracefully empty (warning logged) |
| `triage --json` | ✅ | returns paginated /resellers/{id}/issues/ data |
| `triage --dry-run` | ✅ | envelope returned without side effects |
| `unprovisioned --json` | ✅ | empty list (no unprovisioned agents on test tenant) |
| `bill --json` | ✅ | usage array; missing-ID warning non-fatal |
| `backup-facts --json` | ✅ | 61 rows across 3 engines (classic 6, dr 27, restic 28) from synced data |
| `qbr 4576 --quarter 2026-Q1 --format md` | ✅ | clean Markdown executive report with all sections |
| `drift` (no anchor) | ✅ | clean error directing user to run `attention` first |

## Fixes applied in-session (before this verification, all fix-before-ship per doctrine)

1. **Auth header `Token ` prefix** (`internal/config/config.go`) — was returning the raw token; Servosity (Django REST Framework) requires the literal `Token <key>` scheme. Without this, every API call returned 403 "Authentication credentials were not provided."

2. **Reseller ID resolution** (`internal/cliutil/reseller.go` + test) — added `ResolveResellerID()` that derives the reseller ID from `companies[0].reseller` URL. Discovered during dogfood that `/current-user/` does NOT expose reseller info on partner-scoped tokens (the obvious source). Env override: `SERVOSITY_MSP_RESELLER_ID`.

3. **`attention` uses per-reseller issues** (`internal/cli/attention.go`) — `/issues/` is admin-only for partner tokens. Switched to `/resellers/{id}/issues/`. Stale-backup section made truly non-fatal (was returning the error despite the comment saying "non-fatal").

4. **`triage` uses per-reseller issues** (`internal/cli/triage.go`) — same fix as attention. Per-issue mutation paths unchanged.

5. **`unprovisioned` uses cliutil resolver** (`internal/cli/unprovisioned.go`) — replaced the agent's `/current-user/`-derived resolver with cliutil.ResolveResellerID.

6. **`bill` uses cliutil resolver** (`internal/cli/bill_reconcile.go`) — same.

7. **`stale-backups` graceful redirect** (`internal/cli/stale_backups.go`) — `/reports/stale-backup-sets/` is admin-only. Returns clean directive telling the user to use `backup-facts --since 7d` instead, with a v0.2 note. (Better than a stack trace, but a known UX limitation until v0.2 reshape.)

8. **`bill --reconcile <path> --dry-run` ordering** — file-existence check was firing before dry-run short-circuit, breaking verify-friendly contract.

## v0.2 follow-ups (NOT blocking ship)

These are documented gaps the user should know about before publishing:

1. **`stale-backups` reshape** — derive from local backup tables (`backups`+`restic_backups`+`dr_backups`) with `last_successful_at` older than `--days`. Removes the admin-only `/reports/stale-backup-sets/` dependency.
2. **`attention` restore-queue count** — `drbackup_in_flight` is always 0; populating requires per-company iteration over `/companies/{id}/restore-queues/`.
3. **QBR chart rendering** — add SVG line chart for storage trend and bar chart for success-rate in HTML/PDF mode. v1 ships data tables.
4. **MCP `orchestration: code`** — current 293-tool surface burns agent context. Adding Cloudflare pattern to spec drops scorecard's MCP Surface Strategy from 2/10 to 10/10.
5. **MCP HTTP transport** — currently stdio-only. Adding `transport: [stdio, http]` enables remote agents (Managed Agents, web clients). Drops scorecard's MCP Remote Transport from 5/10 to 10/10.
6. **Type fidelity** — flesh out richer response types in the spec. Improves scorecard's Type Fidelity from 1/5.

## Final shipcheck (post-fixes)

```
LEG               RESULT  EXIT      ELAPSED
verify            PASS    0         11.76s
validate-narrative  PASS    0         0.17s
dogfood           PASS    0         2.55s
workflow-verify   PASS    0         0.01s
verify-skill      PASS    0         2.05s
scorecard         PASS    0         0.25s

Verdict: PASS (6/6 legs passed)
```

Scorecard: 86/100 Grade A.

## Ship recommendation

**SHIP.** All 12 live-API tests pass. All 6 structural shipcheck legs pass. The 8 in-session fixes are real bug-fixes-before-ship, not deferrals. The v0.2 list is honest about what's left — none of it blocks a v1.0 ship for the headline use case (MSP partner fleet ops via CLI).
