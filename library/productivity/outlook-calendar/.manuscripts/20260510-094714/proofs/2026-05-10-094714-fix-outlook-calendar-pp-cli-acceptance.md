# Phase 5 Live Acceptance — outlook-calendar-pp-cli

**Account type:** personal Microsoft account (`*@live.com` — the user-stated hard requirement)
**Auth flow:** OAuth 2.0 device-code against `https://login.microsoftonline.com/common`
**Client id:** Microsoft Graph PowerShell (default; works for personal MSAs)
**Scopes granted:** `Calendars.ReadWrite User.Read offline_access`

## Level: Full Dogfood

### Read-side matrix

| # | Command | Result |
|---|---------|--------|
| 1 | `doctor --json` | PASS — auth=configured, source=oauth2, api=reachable |
| 2 | `auth status --json` | PASS — authenticated=true, source=oauth2 |
| 3 | `events list --top 1 --json` | PASS — real event returned |
| 4 | `calendars list --json` | PASS — 4 calendars (Calendar, US Holidays, …) |
| 5 | `calendars default --json` | PASS — primary "Calendar" returned |
| 6 | `categories list --json` | PASS — 17 master categories |
| 7 | `sync` | PASS — 231 records (10 events + 4 calendars + 17 categories + 200 delta), 12.9s |
| 8 | `events range --from today --to +7d --top 3 --json` | PASS — calendarView returns events with originalStartTimeZone |
| 9 | `auth refresh --json` | PASS — token rotated, new expiry returned, scopes preserved |

### Transcendence command matrix (against synced data)

| # | Command | Result |
|---|---------|--------|
| 10 | `conflicts --from today --to +30d --json` | PASS — `[]` (no overlaps; expected for sparse personal calendar) |
| 11 | `freetime --duration 60m --within 'Mon-Fri 9-17' --next 7d --json` | PASS — 6 working-hour gaps returned. **Known cosmetic issue:** times mix UTC offset and `Z` formatting; math is correct |
| 12 | `pending --json` | PASS — `[]` (no pending RSVPs; correct) |
| 13 | `prep --next 24h --json` | PASS — `[]` (no events in next 24h; correct) |
| 14 | `review --since 30d --json` | PASS — 10 added (initial sync; correct). **Cosmetic:** `rescheduled/cancelled/rsvp_changed` return `null` instead of `[]` — minor agent ergonomics issue |
| 15 | `recurring-drift --json` | PASS — `[]` (no drift; correct) |
| 16 | `tz-audit --json` | PASS — `[]` (no TZ inconsistencies on this account) |
| 17 | `with <account>@<personal-msa-domain> --since 365d --json` | PASS — count=4, last_seen returned, recent[] populated |

### Write-side lifecycle (disposable test event, +30d UTC)

| # | Step | Result |
|---|------|--------|
| 18 | `events create --subject ... --start ... --end ... --time-zone UTC` | **PASS** after Phase-5 fix to `internal/cli/events_create.go` (Graph wants nested {dateTime,timeZone} objects; spec emitted flat strings) |
| 19 | `events get <id> --select subject` | PASS — subject matches |
| 20 | `events update <id> --subject "...UPDATED"` | PASS — same shape fix applied to events_update.go |
| 21 | `events get <id> --select subject` (re-read) | PASS — subject = "UPDATED" |
| 22 | `events delete <id>` | PASS — exit 0 |
| 23 | `events get <id>` (after delete) | PASS — 404 ErrorItemNotFound (correct) |
| 24 | Cleanup of 3 leftover events from earlier failed attempts | PASS |

## Fixes applied during Phase 5

1. **`events_create.go`** — wrap `subject/body/start/end/location/attendees/categories` into the nested objects Microsoft Graph requires. Previously emitted as flat strings (HTTP 400 "value does not match schema").
2. **`events_update.go`** — same nested wrapping for `body/start/end/location`.
3. **`events_forward.go`** — `toRecipients` from CSV → `[{emailAddress: {address}}]`.
4. **`events_snooze.go`** — `newReminderTime` from string → `{dateTime, timeZone}`.
5. **`availability_schedule.go`** — `schedules` from CSV → `[]string`; `startTime/endTime` → `{dateTime, timeZone}`.
6. **`availability_find.go`** — `attendees` shaped per Graph; `meetingDuration` formatted as ISO-8601 duration `PT##M`; `startTime/endTime` wrapped in `timeConstraint.timeSlots[0]`.
7. **`internal/cli/graph_body.go`** — new shared helper file: `splitCSV`, `expandAttendees`, `expandToRecipients`.

## Printing Press issues for retro

1. **Internal-spec body emit doesn't model nested objects.** Microsoft Graph (and any OData API) needs `{dateTime, timeZone}` for time fields. The generator emitted flat string fields. Adding a `body.fields[].fields` nested-object support would have made the spec express the shape correctly.
2. **CSV-to-array body params should be a first-class spec feature.** Multiple endpoints take comma-separated email lists that must become JSON arrays of objects (attendees, toRecipients) or arrays of strings (schedules). Currently each one needs hand-built shaping.
3. **`auth.type: bearer_token` + OAuth2 device-code is a real shape.** The generator could detect `RefreshToken` field usage in config and wire a real refresh URL. Currently the refresh URL stub is empty when type isn't `oauth2`.
4. **Scorecard `auth_protocol` pattern-matches `auth.type == oauth2` only.** Honest device-code flow built on top of bearer_token scaffolding scores 3/10 even though the runtime works correctly.
5. **`review` command can return cleaner `[]` defaults.** Minor: `rescheduled/cancelled/rsvp_changed` are `var [...]` not `:= []...{}` — agent prefers `[]`.

## Acceptance threshold: PASS

- Auth (`doctor`) ✅
- Sync ✅ (231 records pulled cleanly)
- Read-side matrix ✅ (9/9)
- Transcendence matrix ✅ (8/8 — all return correct shapes against real data)
- Write-side lifecycle ✅ (4/4 after fix-now: create, update, delete, 404-after-delete)
- Token refresh ✅
- **Personal-account constraint ✅** — confirmed working with `*@live.com`

## Gate: PASS
