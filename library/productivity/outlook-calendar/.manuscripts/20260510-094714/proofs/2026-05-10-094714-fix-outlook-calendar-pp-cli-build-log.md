# Phase 3 Build Log — outlook-calendar-pp-cli

## What was built

### OAuth device-code flow (the personal-account constraint)
- New package `internal/oauth/` with `device.go` implementing the OAuth 2.0 device authorization grant against `https://login.microsoftonline.com/common`.
- New command `outlook-calendar-pp-cli auth login --device-code` (`internal/cli/auth_login.go`) — issues a device code, polls the token endpoint, persists access + refresh tokens to the existing config.
- New command `outlook-calendar-pp-cli auth refresh` (`internal/cli/auth_refresh.go`) — explicit refresh-token rotation for diagnostics.
- Default client id = Microsoft-published Graph PowerShell client (`14d82eec-204b-4c2f-b7e8-296a70dab67e`), which is configured for `AzureADandPersonalMicrosoftAccount` and works with personal MSAs out of the box. Override via `--client-id` or `OUTLOOK_CALENDAR_CLIENT_ID`.
- Default scopes: `Calendars.ReadWrite User.Read offline_access` — `offline_access` is required for refresh-token issuance.
- Auto-refresh on expiry: edited `internal/client/client.go` `refreshAccessToken` to use Microsoft's `/oauth2/v2.0/token` endpoint instead of the empty token-URL stub the generator emitted. Activated when `cfg.AccessToken` + `cfg.RefreshToken` are present and the access token has expired.

### Eight transcendence commands
All read from the local SQLite store populated by the framework `sync` command. None hand-roll API responses.

| Cmd | File | Power source |
|-----|------|-------------|
| `conflicts` | `internal/cli/conflicts.go` | Self-join over synced events; emits overlap pairs with delta_minutes |
| `freetime` | `internal/cli/freetime.go` | Merge busy intervals + working-hours window subtraction |
| `review` | `internal/cli/review.go` | Diff buckets (added/rescheduled/cancelled/rsvp_changed) keyed off synced_at + last_modified_date_time |
| `pending` | `internal/cli/pending.go` | Local filter where responseStatus.response = none AND start > now() |
| `recurring-drift` | `internal/cli/recurring_drift.go` | Compare each occurrence/exception against its seriesMaster's projected start time |
| `prep` | `internal/cli/prep.go` | Pre-joined dossier (subject + location + attendees + body excerpt + flags) for the next N hours |
| `with` | `internal/cli/with.go` | Per-attendee co-occurrence + recent-N from local store |
| `tz-audit` | `internal/cli/tz_audit.go` | Filter rows where start.timeZone != end.timeZone or != calendar default |

Shared helper: `internal/cli/novel_events.go` parses Microsoft Graph event JSON into a Go-typed `graphEvent` shape, plus `parseGraphTime`, `parseHumanTime`, `parseRelativeDays`, `resolveWindow`, `parseWorkingHours`, and `mergeIntervals`.

### Tests added
- `internal/cli/novel_events_test.go` — table-driven tests for `parseGraphTime`, `parseGraphEvent`, `parseHumanTime`, `parseWorkingHours`, `mergeIntervals`, `resolveWindow`.
- `internal/oauth/device_test.go` — httptest-based tests for `RequestDeviceCode`, `RefreshToken` (including refresh-token preservation when omitted), and error-envelope handling.

### Documentation
- `README.md` and `SKILL.md` value_prop refreshed: stale → pending; auth section now reflects shipped flow (device-code, default client id, config.toml token cache, auto-refresh).
- One README/SKILL recipe updated to use `pending` instead of stale.

## What was intentionally deferred
- **Per-calendar conflicts.** The current `conflicts` self-joins over the entire `events` table. A future improvement would group by `parent_id` (calendar) and report cross-calendar collisions specifically; the underlying schema already supports it via the `parent_id` column.
- **Snapshot table for `review`.** `review` currently approximates the diff via timestamps. A more precise version would write a snapshot row pre-sync and diff against it post-sync.

## Generator limitations encountered
- Internal-spec `auth.type: oauth2` would have wired more OAuth glue, but the device-code flow isn't a generator template; using `bearer_token` and hand-building the device flow was cleaner.
- `sync` and `stale` are reserved framework command names — renamed `sync` resource → `delta` and novel `stale` → `pending`.
- Generator's `refreshAccessToken` template emits an empty `tokenURL` stub when `auth.type` isn't `oauth2`; we filled it in directly with Microsoft's authority. Could be a generator improvement: when `bearer_token` config has `RefreshToken` field present, auto-wire a refresh URL.
