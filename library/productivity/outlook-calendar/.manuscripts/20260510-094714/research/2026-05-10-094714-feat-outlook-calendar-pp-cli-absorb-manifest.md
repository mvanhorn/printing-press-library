# Outlook Calendar CLI — Absorb Manifest

## Tools surveyed
- skylarbpayne/msgcli (Go) — calendar list/get/create/update/delete/respond/availability
- Softeria/ms-365-mcp-server (TS, 200+ tools, calendar coverage)
- elyxlz/microsoft-mcp (Python) — list_events, get_event, create_event, update_event, delete_event, respond_event, check_availability, search_events
- sajadghawami/outlook-mcp (TS, 23 tools / 9 calendar) — events CRUD, decline-event, cancel-event, list-categories, create-category, delete-category
- ryaker/outlook-mcp, kacase/mcp-outlook, XenoXilus/outlook-mcp — overlapping subsets
- pnp/cli-microsoft365 (work/school heavy; calendar tooling sparse)
- inovex/CalendarSync (sync only; not a competitor on commands)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List events | msgcli, elyxlz/microsoft-mcp, sajadghawami | `events list` → `GET /me/events?$top&$skip&$filter&$orderby&$select` | Local FTS + agent-friendly `--json/--select/--csv` |
| 2 | List events in date range | msgcli, ms-365-mcp | `events range` → `GET /me/calendarView?startDateTime=&endDateTime=` | Human ISO date input (`--from "today" --to "+7d"`); auto UTC convert; `Prefer: outlook.timezone="UTC"` header |
| 3 | Get event by id | every wrapper | `events get <id>` → `GET /me/events/{id}` | `--select` to narrow fields |
| 4 | Create event | every wrapper | `events create` → `POST /me/events` (body: subject/start/end/location/attendees/body/recurrence/isOnlineMeeting) | `--dry-run`, `--stdin-json` for batch, `--from-template` |
| 5 | Update event | every wrapper | `events update <id>` → `PATCH /me/events/{id}` | Idempotent semantic flags (`--add-attendee`, `--remove-attendee`, `--set-subject`) |
| 6 | Delete event | every wrapper | `events delete <id>` → `DELETE /me/events/{id}` | `--dry-run`, returns deleted summary in JSON |
| 7 | Accept invite | msgcli, sajadghawami | `events accept <id>` → `POST /me/events/{id}/accept` | `--comment`, `--send-response` |
| 8 | Decline invite | msgcli, sajadghawami | `events decline <id>` → `POST /me/events/{id}/decline` | `--comment`, `--send-response` |
| 9 | Tentatively accept | msgcli, sajadghawami | `events tentative <id>` → `POST /me/events/{id}/tentativelyAccept` | -- |
| 10 | Forward event | sajadghawami | `events forward <id>` → `POST /me/events/{id}/forward` | `--to alice@x.com,bob@y.com`, `--comment` |
| 11 | Cancel event (organizer) | sajadghawami, ms-365-mcp | `events cancel <id>` → `POST /me/events/{id}/cancel` | `--comment` |
| 12 | Snooze reminder | ms-365-mcp | `events snooze <id> --until <ISO>` → `POST /me/events/{id}/snoozeReminder` | -- |
| 13 | Dismiss reminder | ms-365-mcp | `events dismiss <id>` → `POST /me/events/{id}/dismissReminder` | -- |
| 14 | List calendars | every wrapper | `calendars list` → `GET /me/calendars` | -- |
| 15 | Get default calendar | -- | `calendars default` → `GET /me/calendar` | -- |
| 16 | Create calendar | sajadghawami | `calendars create --name X` → `POST /me/calendars` | -- |
| 17 | Update calendar | -- | `calendars update <id>` → `PATCH /me/calendars/{id}` | `--name`, `--color`, `--hex-color` |
| 18 | Delete calendar | -- | `calendars delete <id>` → `DELETE /me/calendars/{id}` | -- |
| 19 | List instances of recurring event | ms-365-mcp | `events instances <id>` → `GET /me/events/{id}/instances?startDateTime=&endDateTime=` | -- |
| 20 | Get free/busy schedule | msgcli (availability) | `availability schedule --emails ... --start ... --end ...` → `POST /me/calendar/getSchedule` | Self-only on personal MSA; degrade message printed |
| 21 | Find meeting times | -- | `availability find --duration 30m --start ... --end ...` → `POST /me/findMeetingTimes` | `--attendees` (work-account peers) |
| 22 | List attachments | sajadghawami | `attachments list <event-id>` → `GET /me/events/{id}/attachments` | -- |
| 23 | Get attachment | sajadghawami | `attachments get <event-id> <id>` → `GET /me/events/{id}/attachments/{aid}` | `--download <path>` writes file |
| 24 | Add attachment | sajadghawami | `attachments add <event-id> --file <path>` → `POST /me/events/{id}/attachments` | -- |
| 25 | Delete attachment | sajadghawami | `attachments delete <event-id> <id>` → `DELETE /me/events/{id}/attachments/{aid}` | -- |
| 26 | List categories | sajadghawami | `categories list` → `GET /me/outlook/masterCategories` | -- |
| 27 | Create category | sajadghawami | `categories create --name X --color ...` → `POST /me/outlook/masterCategories` | -- |
| 28 | Delete category | sajadghawami | `categories delete <id>` → `DELETE /me/outlook/masterCategories/{id}` | -- |
| 29 | Search events (server) | every wrapper | `events search "<query>"` → `GET /me/events?$search="<q>"` | Local FTS fallback if server $search returns capped results |
| 30 | Delta sync events | -- | `delta events` → `GET /me/events/delta` (deltaToken cached per calendar) | First-class incremental sync; no other CLI does this |
| 31 | Delta sync calendarView | -- | `delta view --start <iso> --end <iso>` → `GET /me/calendarView/delta` | First-class incremental sync |

Stub items: none. All 31 absorbed features ship as full implementations against `/me/*` endpoints.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Cross-calendar conflicts | `conflicts --since today --until +7d --json` | 9/10 | SQL self-join on synced `events` (`start_utc/end_utc`) across all user calendars where intervals overlap and the row pair is not the same id; outputs collision pairs as JSON | Brief Build Priorities #4 (Maya persona); absorb manifest has no overlap detection — Outlook's own UI doesn't either |
| 2 | Free-time finder | `freetime --duration 60m --within "Mon-Fri 9-17" --next 7d --exclude-oof` | 9/10 | Walks merged busy-intervals from local `events` (incl. recurrence instances cached during sync), subtracts from working-hours window, returns gaps ≥ duration; honors `showAs` for tentative/OOF | Brief Build Priorities #4; Tomás + Maya; `getSchedule` is degraded on personal MSA — local-data is the only viable substitute |
| 3 | Weekly review (change set) | `review --since last-sync --json` | 8/10 | Compares pre-sync row snapshots vs post-`events/delta` rows; emits added / rescheduled / cancelled / rsvp-changed buckets | Brief Codebase Intelligence + delta cursor; Priya persona; absorb #30/#31 supplies cursor but no diff UX |
| 4 | Pending invites | `pending --json` | 7/10 | Local SELECT `responseStatus.response = 'none' AND start_utc > now()` ordered by start | Brief Build Priorities #4; Priya persona; no Graph endpoint surfaces this filter |
| 5 | Recurring drift | `recurring-drift --json` | 7/10 | For each series master, fetch cached instances via `/me/events/{id}/instances` and compare each instance's start/end/subject/location to the master pattern projection | Brief Build Priorities #4; Priya persona; absorb #19 lists raw instances but not drift |
| 6 | Meeting prep dossier | `prep --next 4h --json` | 7/10 | Local SELECT for events in `[now, now+N]` joined to attendees + attachments-meta + body_preview; recurrence/online-meeting flags added | Brief Build Priorities #4; Devin persona; agent-shaped output is the brief's stated thesis |
| 7 | Attendee co-occurrence | `with --who alice@example.com --since 90d --json` | 6/10 | Local FTS/attendee-table SELECT where `attendees.email = ?` AND `start_utc >= now()-interval` returning count, last_seen, recent N | Devin + Maya personas; brief Data Layer mentions FTS over attendee emails/names; no Graph aggregation endpoint |
| 8 | Time-zone audit | `tz-audit --json` | 5/10 | Local SELECT where `start_tz != calendar.default_tz` OR `start_tz != end_tz`, surfacing rows likely to render wrong on devices | Brief Common quirks (TZ Prefer header); Maya/Priya frustrations |

8 transcendence features — all scoring ≥ 5/10. None require LLM, external service, or write actions beyond what's in the absorbed table. Each uses local SQLite over data already synced via `sync events` / `sync view`.
