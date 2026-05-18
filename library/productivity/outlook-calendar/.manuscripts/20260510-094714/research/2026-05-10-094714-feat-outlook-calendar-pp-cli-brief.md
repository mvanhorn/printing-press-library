# Outlook Calendar CLI Brief

## API Identity
- Domain: Microsoft Graph v1.0 — `https://graph.microsoft.com/v1.0`
- Users: anyone with a Microsoft 365 account (personal *or* work). **Hard requirement for this run: personal accounts must work.**
- Data profile: events, calendars, calendar groups, calendar permissions, attachments, categories, schedule (free/busy), meeting-time suggestions

## User Vision
The user runs `/printing-press outlook-calendar` to drive their personal Microsoft 365 calendar from agentic workflows. **"Consider yourself to have failed if it does not [work with personal accounts]."** Every design decision below is filtered through that constraint:
- All endpoints use `/me/*` (personal-account compatible) — never `/users/{id}/*` or `/groups/{id}/*` (work/school only).
- OAuth 2.0 **device code flow** against `/common` (or `/consumers`) tenant — no localhost server, no interactive browser launch from the CLI process. Refresh tokens stored locally for non-interactive subsequent calls.
- App-registration guidance in the README must instruct: "Public client", "Accounts in any organizational directory and personal Microsoft accounts" (`AzureADandPersonalMicrosoftAccount`).
- Smoke testing in Phase 5 must run against an actual personal Microsoft account.

## Reachability Risk
- None. Microsoft Graph has SLA-grade reliability and no anti-bot/CAPTCHA blocks. The only failure mode is auth misconfiguration.

## Top Workflows
1. **Daily/weekly agenda** — "What's on my calendar today?", "Show me next week" → `calendarView` with date range.
2. **Schedule a meeting** — "Schedule X with Y next week, find a free hour" → `findMeetingTimes` (or local conflict check) + `events POST`.
3. **Respond to invites** — "Accept/decline meeting Z" → `events/{id}/accept|decline|tentativelyAccept`.
4. **Reschedule / cancel** — find by subject or time → `events PATCH` / `events/{id}/cancel`.
5. **Free-busy lookup** — "When am I free between 2-5pm Friday?" → `calendar/getSchedule` (works for self on personal accounts; richer when querying multiple emails on work).

## Table Stakes (every competing tool has these)
- list / get / create / update / delete events
- accept / decline / tentatively-accept / forward an event
- list calendars, switch calendar
- list events in a date window (calendarView)
- search events by subject/text
- recurring-event instance expansion
- categories (list, set on event)
- attachments on event (list, download, upload)

## Data Layer
- **Primary entities:** `events`, `calendars`, `calendar_groups`, `categories`
- **Sync cursor:** `/me/events/delta` and `/me/calendarView/delta` — Microsoft's first-class delta-token incremental sync. The delta token is stored per calendar and allows efficient resync.
- **FTS/search:** SQLite FTS5 over `subject`, `body_preview`, `location.displayName`, attendee emails/names, organizer. Microsoft Graph's `$search` is OK for live but capped (default 25 results, ranking opaque); local FTS gives unlimited offline composability (`<cli> search "1:1" --since 7d --calendar work`).
- **Time fields:** `start.dateTime + start.timeZone` and `end.dateTime + end.timeZone` — must preserve TZ for conflict math; SQLite columns `start_utc TEXT`, `end_utc TEXT`, `start_tz TEXT`, `end_tz TEXT`.

## Codebase Intelligence (from MCP source survey)
- **Auth pattern (every wrapper uses this for CLI):** OAuth 2.0 device code flow, public client app, `AzureADandPersonalMicrosoftAccount`, scopes `Calendars.ReadWrite User.Read offline_access`. Token cached on disk (msgcli uses OS keychain via go-keyring; elyxlz/microsoft-mcp uses `~/.microsoft_mcp_token_cache.json`). Refresh token rotation is automatic.
- **Endpoint patterns:** All competitors use `/me/calendar`, `/me/events`, `/me/calendarView`, `/me/events/{id}/accept`, `/me/calendar/getSchedule`. None hit `/users/{id}` for self-calendar — confirms `/me` is the personal-account-friendly surface.
- **Common quirks:** `Prefer: outlook.timezone="UTC"` header preferred for predictable time math; `Prefer: outlook.body-content-type="text"` returns plain-text body instead of HTML.
- **Pagination:** `@odata.nextLink` cursor in response. List endpoints accept `$top` (max 1000), `$skip`, `$filter`, `$orderby`, `$select`, `$expand`.

## Product Thesis
- **Name:** `outlook-calendar-pp-cli`
- **Why it should exist:** Existing Outlook calendar CLIs are either GUI-on-Windows (outlookctl, OutlookCalendarMCP) or work-account-first (cli-microsoft365). msgcli is the closest peer but exposes no local store, no offline search, no conflict-detection, no agent-tunable JSON output, and no `--select`/`--csv` data-shaping. This CLI is **the only Outlook calendar tool built for AI agents on personal Microsoft 365 accounts**, with a SQLite-backed offline store that enables novel features (conflict detection, recurring-event drift, free-time finder with constraints) that no MCP-only tool can do because they don't persist a corpus.

## Build Priorities
1. **Auth** — OAuth device code flow against `/common`. Token stored at `~/.config/outlook-calendar-pp-cli/token.json` (with strict 0600 perms; OS keychain optional v0.2). `auth login --device-code` (default), `auth logout`, `auth status`.
2. **Foundation: events + calendars data layer** — Sync via `events/delta` and `calendarView/delta`; SQLite tables for events, calendars, attendees, attachments-meta.
3. **Absorbed core (every competitor has these):** events list/get/create/update/delete, calendars list, calendarView (date-range query), respond (accept/decline/tentative/forward), cancel (organizer), getSchedule, findMeetingTimes, search events.
4. **Transcendence (only we can do):**
   - `conflicts` — find overlapping events across all your calendars
   - `freetime` — find N-minute gaps in working hours over the next K days
   - `prep` — meeting prep: next N hours' events with attendee context, body excerpt, attachments
   - `review` — weekly review: what changed since last sync (rescheduled, cancelled, added)
   - `stale` — events I haven't responded to (status = none)
   - `recurring-drift` — recurring-series instances that diverge from the master
5. **Polish — `--json`, `--select`, `--csv`, `--dry-run` everywhere; `--prefer-tz` flag; non-interactive friendly.

## Sources
- [microsoftgraph/msgraph-metadata (master OpenAPI)](https://github.com/microsoftgraph/msgraph-metadata)
- [microsoftgraph/msgraph-sdk-powershell Calendar.yml slice](https://github.com/microsoftgraph/msgraph-sdk-powershell/blob/master/openApiDocs/v1.0/Calendar.yml)
- [Microsoft Graph calendar resource type docs](https://learn.microsoft.com/en-us/graph/api/resources/calendar?view=graph-rest-1.0)
- [Microsoft Graph permissions reference (Calendars.ReadWrite)](https://learn.microsoft.com/en-us/graph/permissions-reference)
- [OAuth 2.0 device authorization grant](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-device-code)
- [skylarbpayne/msgcli](https://github.com/skylarbpayne/msgcli)
- [Softeria/ms-365-mcp-server](https://github.com/softeria/ms-365-mcp-server)
- [elyxlz/microsoft-mcp](https://github.com/elyxlz/microsoft-mcp)
- [sajadghawami/outlook-mcp](https://github.com/sajadghawami/outlook-mcp)
- [pnp/cli-microsoft365](https://pnp.github.io/blog/cli-for-microsoft-365/cli-for-microsoft-365-v11-7/)
