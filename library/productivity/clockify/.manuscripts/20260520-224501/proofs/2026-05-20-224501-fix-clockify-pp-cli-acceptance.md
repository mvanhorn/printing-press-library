# Clockify CLI — Acceptance Report (Phase 5 Live Dogfood)

## Level: Full Dogfood — Gate: PASS

- Live API key supplied; `doctor` confirmed auth valid, API reachable, credentials valid.
- Binary-owned matrix (`dogfood --live --level full`): **281/281 passed, 0 failed**, 563 skipped. Acceptance marker `phase5-acceptance.json` written with `status: pass`.
- All 9 novel features verified against real Clockify data (test fixtures created and removed with the user's approval — see below).

## Behavioral verification of novel features

A real `sync` populated the store (110 records: 1 workspace, 8 users, 18 clients, 50 projects, 30 tags, 3 user-groups). The account had no time entries, so 4 disposable `[pp-test]`-prefixed entries were created (via `backfill --commit`, dogfooding that feature) spanning Mon-Wed of the current week, then deleted afterward.

| Feature | Result |
|---------|--------|
| `timesheet week` | Correct grid: project row, Mon 7.00h / Tue 8.50h / Wed 2.00h, total 17.50h. `--agent --select` dotted-path filtering verified. |
| `timesheet gaps` | Correct: Mon short 1h, Wed short 6h, Thu/Fri 8h each, 23h missing total (Tue 8.5h not flagged). |
| `recap` | Correct: 4 entries, 17.50h, grouped by project, 100% billable split. JSON envelope valid. |
| `audit billable` | Correct: 4 "billable, untagged" findings, 17.50h at risk. |
| `billable pending` | Correct: per-client total 17.50h, 4 entries. |
| `team timesheets` | Correct: full user list, per-user tracked hours; honestly reports submission status as "unknown" when approval data is inaccessible (non-admin key). |
| `projects burn` | Correct: "no projects with a time estimate" — accurate, none of the 50 projects carry a manual estimate. |
| `search` | Correct: full-text search finds the test entries. |
| `backfill` | Correct: CSV (start/end and date/duration), session-log, and shell-history parsers all verified; `--commit` creates entries; `--task` added so force-task workspaces work. |

## Failures found and fixed (all fixed in-session, fix-before-ship)

1. **`search` FTS crash** — a hyphenated query (`pp-test`) raised `SQL logic error: no such column: test`. FTS5 treats `-`/`:`/`[` as operators. Fixed: `ftsSanitizeQuery` quotes each query token as an FTS5 string literal (preserves multi-word AND semantics). Retro candidate: generator's stock `search` should sanitize FTS queries.
2. **`team timesheets` dishonest status** — reported every member "NOT SUBMITTED" when the API key cannot access approval data at all. Fixed: status is "unknown" with an explicit caveat when no approval data is available; tracked-hours caveat added.
3. **`backfill --commit` could not write to a force-task workspace** — the test workspace requires a task on every entry. Added a `--task` flag.
4. **`jobs get/list/prune` missing `Example:`** — generator-emitted subcommands had no help examples. Added. Retro candidate.
5. **`workflow archive --json` invalid JSON** — emitted NDJSON sync events followed by a pretty-printed summary object (two concatenated JSON values). Fixed: the summary is now a single NDJSON line, consistent with `sync`. Retro candidate: `syncResource` hardcodes NDJSON to stdout; `workflow archive --json` should stay NDJSON-consistent.

Fixes applied: 5. Printing Press retro candidates: 3 (search FTS sanitize, jobs Examples, workflow archive JSON consistency) — plus the time-entry sync gap below.

## Printing Press issues for retro
- **Time-entry sync gap:** the bulk `sync` of `/v1/workspaces/{workspaceId}/user/{userId}/time-entries` sent `{userId}` unsubstituted (HTTP 400 "state should be: hexString"). The generator's sync does not resolve the runtime user id for doubly-nested user-scoped resources. Worked around in this CLI: the novel commands hydrate time entries themselves via a live-fetch-and-cache path (`ensureTimeEntries`).
- search FTS query sanitization, jobs subcommand Examples, workflow-archive JSON consistency (above).

## Re-verification
Shipcheck re-run after all fixes: **6/6 legs PASS**, scorecard 92/100 Grade A — no regression.

## Gate: PASS
Full Dogfood matrix 281/281. Every novel feature behaviorally verified against real data. All failures fixed in-session. Test fixtures created and removed; workspace left clean (0 `[pp-test]` entries remain).
