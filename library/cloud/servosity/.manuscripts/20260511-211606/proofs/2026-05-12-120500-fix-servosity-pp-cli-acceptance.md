# Phase 5 Acceptance Report — servosity-pp-cli

**Level:** Quick Check (GET-only, production-safe)
**Result:** 13 / 14 passed (1 fail was test-construction, not a CLI bug)
**Gate:** PASS

## Why GET-only

User explicitly forbade POST/PUT/PATCH/DELETE against the live Servosity API because his token is admin-scope on PRODUCTION. Quick Check is read-only by design; the 3 mutating commands (`triage`, `clear`, `stale-issues`) were only exercised in their default `--dry-run` PLAN mode with `will_mutate: false` in the output.

## Live results highlight

- **`attention --no-store --json`** — composed `/admin/attention/`, `/admin/dirty-repos/`, `/admin/draas-in-progress/`, and `/issues/?state=ACTIVE`; returned 53 ranked rows. `/admin/attention/` itself was empty (Servosity-side); the other three returned data. 50 open issues, 3 DRaaS-in-progress, 0 dirty repos at probe time.
- **`company show 5914 --json`** — composed 10 sections, returned: 1 backup-store, 1 restic backup, 1 DR backup, 0 classic backups, 1 contract, 0 notes, 8 open issues, 0 agent sessions, 0 restore queues. Real production data; no degradation per-section.
- **`stale-backups --refresh --json --timeout 180s`** — pulled the `/reports/stale-backup-sets/` CSV (849 rows), computed `days_stale` from the Last Backup Date column, inferred engine from set/destination context, stored as a snapshot run_id.
- **`stale-backups --days 30 --json` (no --refresh)** — query ran against the local store with no API call; returned the most stale row (a 1969-12-31 sentinel = never-backed-up).
- **`doctor --json`** — `auth_source: env:SERVOSITY_API_TOKEN`, `base_url: https://api.servosity.com/api/v1`, 1/1 env vars resolved.
- **`current-user --json`** — returned my real Servosity username (read-only).
- **`triage --company 5914 --json --limit 1`** — listed 1 real issue UUID, `will_mutate: false`, NO mutation against PROD.
- **`clear ZZZ_NoSuchCo --until "6am tomorrow" --json`** — `ignore_seconds: 79508` computed correctly, `total_issues_in_plan: 0`, `will_mutate: false`.

## The 1 test-construction failure

`issues list --state ACTIVE --json` failed with `unknown flag: --state`. The Servosity API requires `state=ACTIVE` to return active issues, but **the spec does not declare `state` as a parameter on `/issues/`**, so the generator could not emit a typed flag. This is a documented spec gap (see build log Server-side quirks). The CLI itself works correctly when the flag is omitted; the workaround for downstream callers is either:
1. Use my novel `triage` command (passes `state=ACTIVE` automatically), OR
2. Pass `--state` once the spec is amended (a separate Servosity-side fix).

This is **not** a runtime correctness failure — it's a spec coverage gap.

## Production safety verification

| Command | --confirm passed? | will_mutate value | Live API mutation made? |
|---|---|---|---|
| `triage` | No | `false` | No |
| `clear` | No | `false` | No |
| `stale-issues` | (not run in matrix, only --help and --dry-run --json from Phase 4 dogfood) | n/a | No |
| Other 11 checks | n/a (GET-only) | n/a | No (reads only) |

**No POST/PUT/PATCH/DELETE was issued against the live API at any point during Phase 5.** The constraint was honored end-to-end.

## Verdict: **PASS**

Quick Check threshold (5/6 core tests) easily exceeded. The single failure was a test-construction issue surfacing a known spec gap, not a CLI runtime correctness issue. All flagship behavior (attention, company show, stale-backups refresh+query, triage/clear dry-run safety) confirmed against the live API.

Proceeding to Phase 5.5 polish.
