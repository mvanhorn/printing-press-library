# Absorb Manifest — google-calendar (gcal-pp-cli)

Internal-tool print for the assistant (Personal Assistant). Correctness/safety > breadth.
Constraints binding every row: forced `sendUpdates=none` (hard-coded), attendee-bearing
events never mutated, per-profile OAuth scope roles, NO synced event store as source of
truth (live reads + upstream freshness evidence), no Gmail, no TUI/daemons/NL-parsing.

## Absorbed (match or beat)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List calendars | gcalcli / calendarList API | `calendars [--account X]` merged view | role/manifest-aware, JSON |
| 2 | Windowed events list | gcalcli agenda / events.list | `events list --calendar --from --to` (singleEvents=true) | API-side recurrence expansion + freshness fields (updated/etag/fetched_at) |
| 3 | Merged agenda | gcalcli (single-account) | `agenda --from --to` across all manifest calendars | cross-account, per-source freshness |
| 4 | Freebusy | freebusy.query | `freebusy --from --to` across manifest | multi-account single verdict |
| 5 | Create event | gcalcli add | `events create` client-supplied ID, sendUpdates=none | idempotent under replay; structurally quiet |
| 6 | Update/move | gcalcli edit | `events update/move` + attendee-guard + sendUpdates=none | transitive side-effect barrier |
| 7 | Delete | gcalcli delete | `events delete` same guards | " |
| 8 | Auth/doctor | gcalcli auth | `auth --account`, `accounts`, `doctor`; per-profile scope roles | readonly account cannot write BY TOKEN |
| 9 | Search in window | gcalcli search | `events list --q` | API q param |

## Transcendence (novel-features subagent, adversarially cut; 15 candidates → 6 survivors)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Conflict verdict engine (fresh-or-downgrade "checked N of M", busy-only w/ tentative=busy, tz-normalized, mirror-aware, all-day separate, event_type tags, typed exit codes) | `conflicts --from --to` | 10/10 | Implements the reviewed/grilled verdict contract across 3 accounts live; every landscape tool is single-account and evidence-free |
| 2 | Manifest reconciliation (drift findings: new/missing/role-drifted calendars, tz mismatch; `--emit-skeleton`) | `manifest check` | 9/10 | calendars.yaml governance root is this print's invention |
| 3 | Safe-mutation contract (If-Match etag preconditions; `prior` pre-image + `undo` inverse-op in every write response) | `events update\|delete --if-etag` | 9/10 | Writes fail cleanly on mid-flight human edits and carry their own undo evidence |
| 4 | Stateless change sweep (caller-held watermark, deletions included) | `changes --since <ts>` | 8/10 | The no-store rule makes pull-shaped evidence-stamped change detection the only lawful shape; nobody else has it |
| 5 | Open-slot enumeration (contract-inherited busy semantics; downgrades on partial coverage) | `slots --duration 90m --from --to` | 8/10 | Google has no find-a-time API; "open" becomes a citable claim, not LLM arithmetic |
| 6 | Recurring-series deviation report (moved/cancelled instances) | `events exceptions --from --to` | 7/10 | Cross-account sweep of recurrence-exception semantics — the routine-deviation surprise class |

## Kills (recap)
`mirrors` (folded into conflicts verdict) · typed-block standalone (folded to list flag + conflict tags) · `gaps` (cosmetic, no contract) · `manifest init` (demoted to --emit-skeleton) · `events show` (thin wrapper) · `colors` (no persona) · `watch` (needs public endpoint + daemon — forbidden) · `quickAdd` (NL parsing is the assistant's layer) · other-person freebusy (out of NOI domain).

No stubs. All 15 rows are shipping scope.
